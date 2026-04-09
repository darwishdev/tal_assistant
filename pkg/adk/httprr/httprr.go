// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Copied from google.golang.org/adk/internal/httprr into this package so that
// code outside the ADK module can use it.  No logic changes — only the package
// declaration is different.

// Package httprr implements HTTP record and replay, mainly for use in tests.
//
// [Open] creates a new [RecordReplay]. Whether it is recording or replaying
// is controlled by the -httprecord flag, which is defined by this package
// only in test programs (built by "go test").
// See the [Open] documentation for more details.
package httprr

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var record = new(string)

func init() {
	if testing.Testing() {
		record = flag.String("httprecord", "", "re-record traces for files matching `regexp`")
	}
}

// A RecordReplay is an [http.RoundTripper] that can operate in two modes: record and replay.
//
// In record mode, the RecordReplay invokes another RoundTripper
// and logs the (request, response) pairs to a file.
//
// In replay mode, the RecordReplay responds to requests by finding
// an identical request in the log and sending the logged response.
type RecordReplay struct {
	file string            // file being read or written
	real http.RoundTripper // real HTTP connection

	mu        sync.Mutex
	reqScrub  []func(*http.Request) error // scrubbers for logging requests
	respScrub []func(*bytes.Buffer) error // scrubbers for logging responses
	replay    map[string]string           // if replaying, the log
	record    *os.File                    // if recording, the file being written
	writeErr  error                       // if recording, any write error encountered
}

// ScrubReq adds new request scrubbing functions to rr.
//
// Before using a request as a lookup key or saving it in the record/replay log,
// the RecordReplay calls each scrub function, in the order they were registered,
// to canonicalize non-deterministic parts of the request and remove secrets.
// Scrubbing only applies to a copy of the request used in the record/replay log;
// the unmodified original request is sent to the actual server in recording mode.
// A scrub function can assume that if req.Body is not nil, then it has type [*Body].
//
// Calling ScrubReq adds to the list of registered request scrubbing functions;
// it does not replace those registered by earlier calls.
func (rr *RecordReplay) ScrubReq(scrubs ...func(req *http.Request) error) {
	rr.reqScrub = append(rr.reqScrub, scrubs...)
}

// ScrubResp adds new response scrubbing functions to rr.
//
// Before using a response as a lookup key or saving it in the record/replay log,
// the RecordReplay calls each scrub function on a byte representation of the
// response, in the order they were registered, to canonicalize non-deterministic
// parts of the response and remove secrets.
//
// Calling ScrubResp adds to the list of registered response scrubbing functions;
// it does not replace those registered by earlier calls.
func (rr *RecordReplay) ScrubResp(scrubs ...func(*bytes.Buffer) error) {
	rr.respScrub = append(rr.respScrub, scrubs...)
}

// Recording reports whether rr is in recording mode.
func (rr *RecordReplay) Recording() bool {
	return rr.record != nil
}

// Open opens a new record/replay log in the named file and
// returns a [RecordReplay] backed by that file.
//
// By default Open expects the file to exist and contain a
// previously-recorded log of (request, response) pairs,
// which [RecordReplay.RoundTrip] consults to prepare its responses.
//
// If the command-line flag -httprecord is set to a non-empty
// regular expression that matches file, then Open creates
// the file as a new log. In that mode, [RecordReplay.RoundTrip]
// makes actual HTTP requests using rt but then logs the requests and
// responses to the file for replaying in a future run.
func Open(file string, rt http.RoundTripper) (*RecordReplay, error) {
	rec, err := Recording(file)
	if err != nil {
		return nil, err
	}
	if rec {
		return create(file, rt)
	}
	return open(file, rt)
}

// Recording reports whether the "-httprecord" flag is set for the given file.
// It returns an error if the flag is set to an invalid value.
func Recording(file string) (bool, error) {
	if *record != "" {
		re, err := regexp.Compile(*record)
		if err != nil {
			return false, fmt.Errorf("invalid -httprecord flag: %v", err)
		}
		if re.MatchString(file) {
			return true, nil
		}
	}
	return false, nil
}

func create(file string, rt http.RoundTripper) (*RecordReplay, error) {
	f, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(f, "httprr trace v1\n"); err != nil {
		f.Close()
		return nil, err
	}
	return &RecordReplay{file: file, real: rt, record: f}, nil
}

func open(file string, rt http.RoundTripper) (*RecordReplay, error) {
	bdata, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	data := string(bdata)
	line, data, ok := strings.Cut(data, "\n")
	if !ok || line != "httprr trace v1" {
		return nil, fmt.Errorf("read %s: not an httprr trace", file)
	}
	replay := make(map[string]string)
	for data != "" {
		line, data, ok = strings.Cut(data, "\n")
		f1, f2, _ := strings.Cut(line, " ")
		n1, err1 := strconv.Atoi(f1)
		n2, err2 := strconv.Atoi(f2)
		if !ok || err1 != nil || err2 != nil || n1 > len(data) || n2 > len(data[n1:]) {
			return nil, fmt.Errorf("read %s: corrupt httprr trace", file)
		}
		var req, resp string
		req, resp, data = data[:n1], data[n1:n1+n2], data[n1+n2:]
		replay[req] = resp
	}
	return &RecordReplay{file: file, real: rt, replay: replay}, nil
}

// Client returns an http.Client using rr as its transport.
func (rr *RecordReplay) Client() *http.Client {
	return &http.Client{Transport: rr}
}

// A Body is an io.ReadCloser used as an HTTP request body.
// In a Scrubber, if req.Body != nil, then req.Body is guaranteed
// to have type *Body, making it easy to access the body to change it.
type Body struct {
	Data       []byte
	ReadOffset int
}

// Read implements io.Reader.
func (b *Body) Read(p []byte) (int, error) {
	n := copy(p, b.Data[b.ReadOffset:])
	if n == 0 {
		return 0, io.EOF
	}
	b.ReadOffset += n
	return n, nil
}

// Close is a no-op, implementing io.Closer.
func (b *Body) Close() error { return nil }

// RoundTrip implements [http.RoundTripper].
func (rr *RecordReplay) RoundTrip(req *http.Request) (*http.Response, error) {
	reqWire, err := rr.reqWire(req)
	if err != nil {
		return nil, err
	}
	if rr.replay != nil {
		return rr.replayRoundTrip(req, reqWire)
	}
	if err := rr.writeError(); err != nil {
		return nil, err
	}
	resp, err := rr.real.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	respWire, err := rr.respWire(resp)
	if err != nil {
		return nil, err
	}
	if err := rr.writeLog(reqWire, respWire); err != nil {
		return nil, err
	}
	return resp, nil
}

func (rr *RecordReplay) reqWire(req *http.Request) (string, error) {
	rkey := req.Clone(context.Background())
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return "", err
		}
		req.Body = &Body{Data: body}
		rkey.Body = &Body{Data: bytes.Clone(body)}
	}
	for _, scrub := range rr.reqScrub {
		if err := scrub(rkey); err != nil {
			return "", err
		}
	}
	if rkey.Body != nil {
		rkey.ContentLength = int64(len(rkey.Body.(*Body).Data))
	}
	var key strings.Builder
	if err := rkey.WriteProxy(&key); err != nil {
		return "", err
	}
	return key.String(), nil
}

func (rr *RecordReplay) respWire(resp *http.Response) (string, error) {
	var key bytes.Buffer
	if err := resp.Write(&key); err != nil {
		return "", err
	}
	resp2, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(key.Bytes())), resp.Request)
	if err != nil {
		return "", err
	}
	*resp = *resp2
	for _, scrub := range rr.respScrub {
		if err := scrub(&key); err != nil {
			return "", err
		}
	}
	return key.String(), nil
}

func (rr *RecordReplay) replayRoundTrip(req *http.Request, reqLog string) (*http.Response, error) {
	respLog, ok := rr.replay[reqLog]
	if !ok {
		return nil, fmt.Errorf("cached HTTP response not found for:\n%s", reqLog)
	}
	resp, err := http.ReadResponse(bufio.NewReader(strings.NewReader(respLog)), req)
	if err != nil {
		return nil, fmt.Errorf("read %s: corrupt httprr trace: %v", rr.file, err)
	}
	return resp, nil
}

func (rr *RecordReplay) writeError() error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.writeErr
}

func (rr *RecordReplay) writeLog(reqWire, respWire string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.writeErr != nil {
		return rr.writeErr
	}
	_, err1 := fmt.Fprintf(rr.record, "%d %d\n", len(reqWire), len(respWire))
	_, err2 := rr.record.WriteString(reqWire)
	_, err3 := rr.record.WriteString(respWire)
	if err := cmp.Or(err1, err2, err3); err != nil {
		rr.writeErr = err
		rr.record.Close()
		os.Remove(rr.file)
		return err
	}
	return nil
}

// Close closes the RecordReplay. It is a no-op in replay mode.
func (rr *RecordReplay) Close() error {
	if rr.writeErr != nil {
		return rr.writeErr
	}
	if rr.record != nil {
		return rr.record.Close()
	}
	return nil
}
