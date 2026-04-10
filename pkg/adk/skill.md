# Skill: Add a New ADK Agent

Follow these steps exactly each time a new agent is needed.

---

## 1. Create the agent package

Create a new directory under `pkg/adk/<agentname>/` (use lowercase, no underscores in the dir name — e.g. `signalingagentmapper`).

### `<agentname>_definition.go`

Holds only the agent's constants — name, description, and instruction template.
State variables available to the instruction template are injected from the session
state by key name, referenced as `{key_name}` inside the instruction string.

```go
package <agentname>

const (
    agentName        = "<agent_name>"           // snake_case, unique across the app
    agentDescription = "<one-line description>"
    agentInstructions = `<system prompt>

Available state variables are injected via {key_name} placeholders.`
)
```

### `<agentname>.go`

Contains:
- **State struct** — fields that go into the session state.
- **Agent struct** — holds `agentName string` and `llm *model.LLM`.
- **`New<AgentName>(llm *model.LLM) *<AgentName>`** — constructor.
- **`NewAgentConfig(model model.LLM) *llmagent.Config`** — returns the config wired to the constants.
- **`NewAgentState(state <State>) map[string]any`** — converts the typed state to the `map[string]any` expected by `session.CreateRequest.State`. Keys must match the `{placeholders}` in the instruction template.
- **`Run(r *runner.Runner, req adkutils.AgentRunRequest) <return>`** — executes the agent.

**Streaming vs non-streaming:**
- Use `iter.Seq2[string, error]` + `agent.StreamingModeSSE` when the caller needs to stream tokens (e.g. displaying output live).
- Use `(string, error)` + `agent.StreamingModeNone` when the caller needs a single complete value (e.g. returning an ID, a classification, a score).

```go
// Streaming example
func (a *MyAgent) Run(r *runner.Runner, req adkutils.AgentRunRequest) iter.Seq2[string, error] {
    return func(yield func(string, error) bool) {
        events := r.Run(req.Ctx, req.UserID, req.SessionID,
            &genai.Content{Role: "user", Parts: []*genai.Part{{Text: req.Prompt.(string)}}},
            agent.RunConfig{StreamingMode: agent.StreamingModeSSE},
        )
        for event, err := range events {
            if err != nil { yield("", err); return }
            if text := adkutils.SessionEventToString(event); text != "" {
                if !yield(text, nil) { return }
            }
        }
    }
}

// Non-streaming example
func (a *MyAgent) Run(r *runner.Runner, req adkutils.AgentRunRequest) (string, error) {
    events := r.Run(req.Ctx, req.UserID, req.SessionID,
        &genai.Content{Role: "user", Parts: []*genai.Part{{Text: req.Prompt.(string)}}},
        agent.RunConfig{StreamingMode: agent.StreamingModeNone},
    )
    var sb strings.Builder
    for event, err := range events {
        if err != nil { return "", err }
        if text := adkutils.SessionEventToString(event); text != "" { sb.WriteString(text) }
    }
    return strings.TrimSpace(sb.String()), nil
}
```

---

## 2. Wire the agent into `ADKService`

### `adkservice.go`

1. Import the new package.
2. Add two fields to `ADKService`:
   ```go
   <agentName>        *<agentpkg>.<AgentType>
   <agentName>Runner  *runner.Runner
   ```
3. Add two methods to `ADKServiceInterface`:
   ```go
   New<AgentName>State(req <agentpkg>.<StateType>) map[string]any
   <AgentName>Run(req adkutils.AgentRunRequest) <return type>
   ```
4. Inside `NewADKService`, after existing agent setup:
   ```go
   myAgent := <agentpkg>.New<AgentName>(&<chosenModel>)
   myAgentConfig := myAgent.NewAgentConfig(<chosenModel>)
   myAgentRunner, err := NewAgentRunner(ctx, appName, sessionService, *myAgentConfig)
   if err != nil {
       return nil, fmt.Errorf("error creating runner for <agent_name>: %w", err)
   }
   ```
5. Add both to the returned `&ADKService{...}` struct literal.

### `<agentname>.go` (service method file)

Create `pkg/adk/<agentname>.go` (in the `adk` package, not the agent sub-package).
It wires the service methods to the agent:

```go
package adk

import (
    "<module>/pkg/adk/<agentpkg>"
    "<module>/pkg/adkutils"
)

func (s *ADKService) <AgentName>Run(req adkutils.AgentRunRequest) <return> {
    return s.<agentName>.Run(s.<agentName>Runner, req)
}

func (s *ADKService) New<AgentName>State(req <agentpkg>.<StateType>) map[string]any {
    return s.<agentName>.NewAgentState(req)
}
```

---

## 3. Add tests

### `main_test.go`

Shared helpers live here (`newService`, `writeRecord`, `uniqueID`, `BuildSignalString`).
Do not add agent-specific variables here.

### `<agentname>_test.go`

Create `pkg/adk/<agentname>_test.go` in the `adk` package.

```go
package adk

import (
    "context"
    "<module>/pkg/adk/<agentpkg>"
    "<module>/pkg/adkutils"
    "testing"
)

var test<AgentName>State = <agentpkg>.<StateType>{ /* test data */ }

func Test<AgentName>Run(t *testing.T) {
    svc := newService(t)   // skips automatically when GOOGLE_API_KEY is unset
    ctx := context.Background()
    id := uniqueID(t)

    state := svc.New<AgentName>State(test<AgentName>State)
    if err := svc.SessionUpsert(ctx, id, testUserName, state); err != nil {
        t.Fatalf("SessionUpsert: %v", err)
    }

    req := adkutils.AgentRunRequest{Ctx: ctx, SessionID: id, UserID: testUserName, Prompt: "<input>"}
    output, err := svc.<AgentName>Run(req)
    if err != nil {
        t.Fatalf("<AgentName>Run: %v", err)
    }
    writeRecord(t, id, req.Prompt.(string), output)
    // assert on output …
}
```

---

## Model selection guidelines

| Model constant          | Use when                                              |
|-------------------------|-------------------------------------------------------|
| `geminiLiteModel`       | Classification, ID extraction, short non-streaming tasks |
| `geminiModel`           | General reasoning, moderate-length responses          |
| `geminiProModel`        | Complex reasoning, long-form generation               |
