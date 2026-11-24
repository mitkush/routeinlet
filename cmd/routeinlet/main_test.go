package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const history = `{"model":"pilot","stream":true,"messages":[{"role":"system","content":"remember"},{"role":"user","content":"A17"},{"role":"assistant","content":"OK"},{"role":"user","content":"repeat"}]}`

func makeRequest(ctx context.Context, url, body string) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, "POST", url+chatPath, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer local")
	req.Header.Set("Content-Type", "application/json")
	return req
}
func TestForwardAndCancel(t *testing.T) {
	stopped := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) != history || r.Header.Get("Authorization") != "Bearer transport" || r.URL.Path != chatPath {
			t.Error("request or credentials changed")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(stopped)
	}))
	defer up.Close()
	app := httptest.NewServer(gateway{up.URL, "pilot", "local", "transport", up.Client()})
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	res, err := app.Client().Do(makeRequest(ctx, app.URL, history))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(res.Body, buf)
	if err != nil || string(buf) != "data: first\n\n" || res.Header.Get("Gateway-Billing-Source") != "unknown" {
		t.Error("incremental stream or funding metadata missing")
	}
	cancel()
	res.Body.Close()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not observe cancellation")
	}
}
func TestPreflight(t *testing.T) {
	for _, tc := range []struct{ body, field string }{
		{strings.Replace(history, `"stream":true`, `"stream":false`, 1), "stream"},
		{strings.Replace(history, `"remember"`, `[{"type":"image"}]`, 1), "messages[0].content"},
		{strings.Replace(history, `"model"`, `"tools":[],"model"`, 1), "tools"},
		{strings.Replace(history, `"pilot"`, `"other"`, 1), "model"},
		// Duplicates must not pass whichever value the connector's reader keeps.
		{strings.Replace(history, `"model":"pilot"`, `"model":"pilot","model":"other"`, 1), "model"},
		{strings.Replace(history, `"model"`, `"model":"other","model"`, 1), "model"},
		{strings.Replace(history, `{"role":"user","content":"A17"}`, `{"role":"tool","role":"user","content":"A17"}`, 1), "messages[1].role"},
		{strings.Replace(history, `"content":"A17"`, `"content":[{"type":"image_url"}],"content":"A17"`, 1), "messages[1].content"},
	} {
		w := httptest.NewRecorder()
		g := gateway{model: "pilot", key: "local"}
		g.ServeHTTP(w, makeRequest(context.Background(), "http://test", tc.body))
		if w.Code != 400 || !strings.Contains(w.Body.String(), tc.field) {
			t.Errorf("expected %s: %d %s", tc.field, w.Code, w.Body.String())
		}
	}
}
func TestRateLimitSingleAttempt(t *testing.T) {
	attempts := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { attempts++; w.WriteHeader(429) }))
	defer up.Close()
	g := gateway{up.URL, "pilot", "local", "transport", up.Client()}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, makeRequest(context.Background(), "http://test", history))
	if attempts != 1 || w.Code != 429 || !strings.Contains(w.Body.String(), "rate_limit_cause_unknown") {
		t.Fatal(w.Body.String())
	}
}

func TestConnectorStreamCompletion(t *testing.T) {
	const partial = "data: partial text\n\n"
	for _, tc := range []struct {
		name, suffix string
		complete     bool
	}{
		{"early EOF", "", false},
		{"unframed marker", "data: [DONE]\n", false},
		{"normal", "data: [DONE]\n\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, partial+tc.suffix)
			}))
			defer up.Close()
			app := httptest.NewServer(gateway{up.URL, "pilot", "local", "transport", up.Client()})
			defer app.Close()
			res, err := app.Client().Do(makeRequest(context.Background(), app.URL, history))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if res.StatusCode != 200 || string(body) != partial+tc.suffix {
				t.Fatalf("unexpected committed response: %d %q", res.StatusCode, body)
			}
			if tc.complete && err != nil || !tc.complete && err != io.ErrUnexpectedEOF {
				t.Fatalf("complete=%t, body read error=%v", tc.complete, err)
			}
		})
	}
}

func TestCompletionWithoutUpstreamClose(t *testing.T) {
	const wire = "data: text\n\ndata: [DONE]\n\n"
	released := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, wire)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			close(released)
		case <-time.After(5 * time.Second):
		}
	}))
	defer up.Close()
	app := httptest.NewServer(gateway{up.URL, "pilot", "local", "transport", up.Client()})
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := app.Client().Do(makeRequest(ctx, app.URL, history))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || string(body) != wire {
		t.Fatalf("completed stream not delivered cleanly: %q, %v", body, err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("gateway kept the connector request open after [DONE]")
	}
}

func TestTerminalEventAcrossReads(t *testing.T) {
	for _, tc := range []struct {
		wire string
		done bool
	}{
		{": keepalive\r\ndata:[DONE]\r\n\r\n", true},
		{"data: [DONE]\r\r", true},
		{"data: [DONE]\r\n", false},
		{"data: [DONE]more\n\n", false},
		{"data: text\ndata: [DONE]\n\n", false},
		{"data: [DONE]\ndata:\n\n", false},
		{": data: [DONE]\n\n", false},
	} {
		var event terminalEvent
		done := false
		for i := range tc.wire {
			if _, complete := event.observe([]byte{tc.wire[i]}); complete {
				done = true
				break
			}
		}
		if done != tc.done {
			t.Errorf("%q: got %t", tc.wire, done)
		}
	}
}
