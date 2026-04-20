package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"tal_assistant/pkg/adapterutils"
	"tal_assistant/pkg/adkutils"

	"github.com/redis/go-redis/v9"
)

// ── Key builders ───────────────────────────────────────────────────────────────
// All Redis keys for the question bank subsystem are constructed here.
// Centralising key construction means that if the naming scheme ever changes,
// only these functions need updating — no grep-and-replace across callers.

// questionBankKey returns the Redis hash key for a specific version of a
// question bank. Each field in the hash is a questionID; its value is the
// JSON-encoded QuestionBankQuestion.
//
//	Format: qbank:<eventID>:<version>
//	Example: qbank:iv_abc123:0003
func questionBankKey(eventID, version string) string {
	return fmt.Sprintf("%s%s:%s", questionBankKeyPrefix, eventID, version)
}

// questionBankDocKey returns the Redis hash key for a single question document
// within a versioned question bank. RediSearch treats each of these hashes as
// an individual indexable document, enabling per-question vector search.
//
//	Format: qbank:<eventID>:<version>:<questionID>
//	Example: qbank:iv_abc123:0003:TLQ001
func questionBankDocKey(eventID, version, questionID string) string {
	return fmt.Sprintf("%s:%s", questionBankKey(eventID, version), questionID)
}

// questionBankLatestKey returns the key of the plain string counter that tracks
// the most recently created version number for an interview's question bank.
// Redis INCR initialises this to 1 on first use, so no separate setup is needed.
//
//	Format: qbank:<eventID>:latest
//	Example: qbank:iv_abc123:latest
func questionBankLatestKey(eventID string) string {
	return fmt.Sprintf("%s%s:latest", questionBankKeyPrefix, eventID)
}

// questionBankIndexName returns the FT index name scoped to a specific question
// bank version. Scoping the index to a single version means KNN searches never
// bleed across regenerations of the same interview's question bank.
//
//	Format: qbank:<eventID>:<version>:idx
//	Example: qbank:iv_abc123:0003:idx
func questionBankIndexName(eventID, version string) string {
	return fmt.Sprintf("%s:idx", questionBankKey(eventID, version))
}

// ── Question bank ──────────────────────────────────────────────────────────────

// QuestionBankCreate persists a new versioned question bank for the given
// interview and returns the version string (zero-padded, e.g. "0003").
//
// Each call always creates a new version rather than overwriting the previous
// one, so callers can regenerate a question bank without losing history.
// The version counter lives at questionBankLatestKey and is atomically
// incremented via Redis INCR before any questions are written.
//
// Alongside the question JSON, an embedding of the Question field is stored
// so that QuestionBankSearch can perform semantic (KNN) lookups against this
// version's dedicated RediSearch index.
//
// embedFunc is intentionally injected by the caller so that this function
// stays decoupled from any specific embedding provider (OpenAI, Cohere, etc.).
func (c *RedisCacheClient) QuestionBankCreate(
	ctx context.Context,
	eventID string,
	questions []adkutils.QuestionBankQuestion,
	embedFunc func(text string) ([]float32, error),
) (string, error) {
	if len(questions) == 0 {
		return "", nil
	}

	// ── 1. Allocate the next version number ────────────────────────────────────
	// INCR is atomic, so concurrent creates for the same interview will always
	// receive distinct version numbers with no locking required.
	versionInt, err := c.client.Incr(ctx, questionBankLatestKey(eventID)).Result()
	if err != nil {
		return "", fmt.Errorf("allocate version for interview %s: %w", eventID, err)
	}
	version := fmt.Sprintf("%04d", versionInt)

	// ── 2. Create a RediSearch vector index scoped to this version ─────────────
	// The index prefix matches questionBankDocKey so only documents belonging
	// to this exact version are indexed. Older versions remain searchable via
	// their own indexes if needed.
	if err := c.ensureQuestionBankIndex(ctx, eventID, version); err != nil {
		return "", fmt.Errorf("create vector index for bank %s v%s: %w", eventID, version, err)
	}

	// ── 3. Write all questions in a single pipeline round-trip ─────────────────
	pipe := c.client.Pipeline()
	for i, q := range questions {
		// Preserve the generator's explicit ordering when provided; fall back to
		// insertion order so the bank can always be reconstructed in sequence.
		if q.Order == 0 {
			q.Order = i + 1
		}

		data, err := json.Marshal(q)
		if err != nil {
			return "", fmt.Errorf("marshal question %s: %w", q.ID, err)
		}

		// Embed only the Question field — other fields (category, difficulty, etc.)
		// are not semantically searched and do not need vector representation.
		vec, err := embedFunc(q.Question)
		if err != nil {
			return "", fmt.Errorf("embed question %s: %w", q.ID, err)
		}

		// Each question is stored as its own hash so RediSearch can index it as a
		// separate document. The hash carries three fields:
		//   data          – full JSON blob, returned as-is to callers
		//   question_text – plain text copy used for optional TEXT search filters
		//   embedding     – raw float32 bytes consumed by the KNN index
		docKey := questionBankDocKey(eventID, version, q.ID)
		pipe.HSet(ctx, docKey,
			"data", string(data),
			"question_text", q.Question,
			"embedding", adapterutils.Float32ToBytes(vec),
		)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("write question bank %s v%s: %w", eventID, version, err)
	}

	return version, nil
}

// QuestionBankGetByQuestionID retrieves a single question from a known question
// bank version without any index lookups. The caller must supply all three
// identifiers; there is no fallback resolution.
func (c *RedisCacheClient) QuestionBankQuestionFind(
	ctx context.Context,
	eventID string,
	qbankID string,
	questionID string,
) (*adkutils.QuestionBankQuestion, error) {
	docKey := questionBankDocKey(eventID, qbankID, questionID)

	raw, err := c.client.HGet(ctx, docKey, "data").Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("question %s not found in bank %s v%s", questionID, eventID, qbankID)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch question %s from bank %s v%s: %w", questionID, eventID, qbankID, err)
	}

	var q adkutils.QuestionBankQuestion
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		return nil, fmt.Errorf("unmarshal question %s: %w", questionID, err)
	}

	return &q, nil
}

// QuestionBankSearch performs semantic similarity search against a specific
// version of a question bank. Only the Question field is indexed, so results
// reflect conceptual similarity to the search query — not keyword matching
// against categories, difficulty levels, or other metadata.
//
// threshold is a cosine similarity value in [0, 1]; only questions that meet
// or exceed this value are returned. A threshold of 0.85 is a reasonable
// starting point for "meaningfully related" questions.
func (c *RedisCacheClient) QuestionBankSearch(
	ctx context.Context,
	eventID, qbankID, query string,
	threshold float64,
	embedFunc func(text string) ([]float32, error),
) ([]adkutils.QuestionBankQuestion, error) {
	// ── 1. Embed the search query using the same model used at write time ───────
	// Using a different model than the one used in QuestionBankCreate will
	// produce vectors in a different space, causing incorrect similarity scores.
	vec, err := embedFunc(query)
	if err != nil {
		return nil, fmt.Errorf("embed search query: %w", err)
	}

	// ── 2. Run KNN against the version-scoped index ────────────────────────────
	// Fetching 10 candidates before threshold filtering gives enough headroom
	// to return multiple results without over-fetching from Redis.
	res, err := c.client.Do(ctx, "FT.SEARCH", questionBankIndexName(eventID, qbankID),
		"*=>[KNN 10 @embedding $vec AS score]",
		"PARAMS", "2", "vec", adapterutils.Float32ToBytes(vec),
		"RETURN", "2", "data", "score",
		"SORTBY", "score",
		"DIALECT", "2",
	).Result()
	if err != nil {
		return nil, fmt.Errorf("KNN search in bank %s v%s: %w", eventID, qbankID, err)
	}

	// ── 3. Parse FT.SEARCH response ────────────────────────────────────────────
	// FT.SEARCH returns: [totalCount, key1, [field, val, …], key2, [field, val, …], …]
	items, ok := res.([]interface{})
	if !ok || len(items) < 1 {
		return nil, nil
	}

	var matched []adkutils.QuestionBankQuestion
	for i := 1; i < len(items); i += 2 {
		fields, ok := items[i+1].([]interface{})
		if !ok {
			continue
		}

		fieldMap := make(map[string]string)
		for j := 0; j+1 < len(fields); j += 2 {
			k, _ := fields[j].(string)
			v, _ := fields[j+1].(string)
			fieldMap[k] = v
		}

		// RediSearch returns cosine distance in [0, 2] (0 = identical, 2 = opposite).
		// Convert to similarity in [0, 1] so callers can reason in familiar terms.
		distance, err := strconv.ParseFloat(fieldMap["score"], 64)
		if err != nil {
			continue
		}
		if similarity := 1 - (distance / 2); similarity < threshold {
			continue
		}

		var q adkutils.QuestionBankQuestion
		if err := json.Unmarshal([]byte(fieldMap["data"]), &q); err != nil {
			return nil, fmt.Errorf("unmarshal search result: %w", err)
		}
		matched = append(matched, q)
	}

	return matched, nil
}

// ensureQuestionBankIndex creates a fresh RediSearch HNSW vector index for the
// given question bank version. Any pre-existing index with the same name is
// dropped first to handle retried creates cleanly.
//
// The PREFIX is set to the questionBankDocKey prefix for this version so the
// index only covers documents that belong to this bank — never adjacent versions
// or other question banks.
func (c *RedisCacheClient) ensureQuestionBankIndex(ctx context.Context, eventID, version string) error {
	indexName := questionBankIndexName(eventID, version)
	docPrefix := questionBankKey(eventID, version) + ":"

	// Drop silently — the index may not exist yet on first create.
	c.client.Do(ctx, "FT.DROPINDEX", indexName)

	_, err := c.client.Do(ctx, "FT.CREATE", indexName,
		"ON", "HASH",
		"PREFIX", "1", docPrefix,
		"SCHEMA",
		"question_text", "TEXT",
		"embedding", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", "1536", // must match the output dimension of the embedding model
		"DISTANCE_METRIC", "COSINE",
	).Result()
	if err != nil {
		return fmt.Errorf("FT.CREATE %s: %w", indexName, err)
	}
	return nil
}

// QuestionBankFind retrieves all questions belonging to a specific version of
// an event's question bank. Results are sorted by Order so the caller always
// receives questions in their intended sequence regardless of how Redis
// returned the hash fields.
//
// Returns an empty slice when the question bank exists but contains no
// questions, and nil with no error when the key does not exist.
func (c *RedisCacheClient) QuestionBankFind(
	ctx context.Context,
	eventID string,
	qbankID string,
) ([]adkutils.QuestionBankQuestion, error) {
	// KEYS returns all doc keys under this version prefix so we can fetch each
	// question's hash in a single pipeline pass without knowing question IDs upfront.
	prefix := questionBankKey(eventID, qbankID) + ":"
	keys, err := c.client.Keys(ctx, prefix+"*").Result()
	if err != nil {
		return nil, fmt.Errorf("list question keys for bank %s v%s: %w", eventID, qbankID, err)
	}
	if len(keys) == 0 {
		return nil, nil
	}

	// Fetch the data field from every question doc in one round-trip.
	pipe := c.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGet(ctx, key, "data")
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("fetch questions for bank %s v%s: %w", eventID, qbankID, err)
	}

	questions := make([]adkutils.QuestionBankQuestion, 0, len(cmds))
	for _, cmd := range cmds {
		raw, err := cmd.Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read question data for bank %s v%s: %w", eventID, qbankID, err)
		}

		var q adkutils.QuestionBankQuestion
		if err := json.Unmarshal([]byte(raw), &q); err != nil {
			return nil, fmt.Errorf("unmarshal question in bank %s v%s: %w", eventID, qbankID, err)
		}
		questions = append(questions, q)
	}

	// Restore intended question order — Redis hash field iteration order is
	// not guaranteed to match insertion order.
	sort.Slice(questions, func(i int, j int) bool {
		return questions[i].Order < questions[j].Order
	})

	return questions, nil
}
