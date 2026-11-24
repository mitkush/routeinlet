package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDuplicateKeysNeverReachConnector(t *testing.T) {
	for _, tc := range []struct{ name, body, field string }{
		{"large model number", strings.Replace(history, `"model"`, `"model":1e1000,"model"`, 1), "model"},
		{"negative large model number", strings.Replace(history, `"model"`, `"model":-1e1000,"model"`, 1), "model"},
		{"escaped model key", strings.Replace(history, `"model"`, `"mo\u0064el":1e1000,"model"`, 1), "model"},
		{"nested role", strings.Replace(history, `"role":"user"`, `"role":1e1000,"role":"user"`, 1), "messages[1].role"},
		{"nested content array", strings.Replace(history, `"content":"A17"`, `"content":[1e1000],"content":"A17"`, 1), "messages[1].content"},
		{"escaped content key", strings.Replace(history, `"content":"A17"`, `"con\u0074ent":1e1000,"content":"A17"`, 1), "messages[1].content"},
		{"overwritten history", strings.Replace(history, `"messages"`, `"messages":[{"content":1e1000}],"messages"`, 1), "messages"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				attempts++
				return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})}
			g := gateway{"http://connector", "pilot", "local", "transport", client}
			w := httptest.NewRecorder()
			g.ServeHTTP(w, makeRequest(context.Background(), "http://test", tc.body))
			var response struct {
				Error struct{ Code, Param string }
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if attempts != 0 || w.Code != 400 || response.Error.Code != "duplicate_field" || response.Error.Param != tc.field {
				t.Fatalf("attempts=%d, status=%d, body=%s", attempts, w.Code, w.Body.String())
			}
		})
	}
}

// Deliver exact read boundaries, including a final read returning data and EOF.
type chunkBody struct {
	chunks []string
	reads  int
	closed bool
}

func (b *chunkBody) Read(p []byte) (int, error) {
	b.reads++
	if len(b.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.chunks[0])
	b.chunks[0] = b.chunks[0][n:]
	if b.chunks[0] == "" {
		b.chunks = b.chunks[1:]
	}
	if len(b.chunks) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (b *chunkBody) Close() error { b.closed = true; return nil }

func TestStreamStopsAtTerminalEvent(t *testing.T) {
	for _, newline := range []string{"\n", "\r", "\r\n"} {
		for _, marker := range []string{"data: [DONE]", "data:[DONE]"} {
			// Lookalikes and multiline data must not end the stream.
			lines := []string{": data: [DONE]", "", "data: [DONE]more", "", "data: text", "data: [DONE]", "", "data: [DONE]", "data:", "", marker, "", ""}
			wire := strings.Join(lines, newline)
			const suffix = "data: MUST NOT BE FORWARDED\n\n"
			for split := 0; split <= len(wire); split++ {
				t.Run(fmt.Sprintf("%q/%s/split=%d", newline, marker, split), func(t *testing.T) {
					chunks := []string{wire[split:] + suffix}
					if split > 0 {
						chunks = append([]string{wire[:split]}, chunks...)
					}
					body := &chunkBody{chunks: chunks}
					attempts := 0
					var upstreamContext context.Context
					client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
						attempts++
						upstreamContext = r.Context()
						return &http.Response{StatusCode: 200, Body: body, Header: http.Header{"Content-Type": {"text/event-stream"}}}, nil
					})}
					w := httptest.NewRecorder()
					gateway{"http://connector", "pilot", "local", "transport", client}.ServeHTTP(w, makeRequest(context.Background(), "http://test", history))
					want := wire
					// A terminal CR dispatches the event immediately. If its optional
					// LF is in the next read, closing here still leaves valid SSE.
					if newline == "\r\n" && split == len(wire)-1 {
						want = wire[:len(wire)-1]
					}
					if w.Code != 200 || w.Body.String() != want {
						t.Fatalf("status=%d, got %q, want %q", w.Code, w.Body.String(), want)
					}
					expectedReads := 1
					if split > 0 && split < len(want) {
						expectedReads = 2
					}
					if attempts != 1 || body.reads != expectedReads || !body.closed || upstreamContext.Err() != context.Canceled {
						t.Fatalf("attempts=%d reads=%d want=%d closed=%t context=%v", attempts, body.reads, expectedReads, body.closed, upstreamContext.Err())
					}
				})
			}
		}
	}
}
