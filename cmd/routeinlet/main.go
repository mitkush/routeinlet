package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const chatPath = "/v1/chat/completions"

type gateway struct {
	endpoint    string
	model       string
	key         string
	upstreamKey string
	client      *http.Client
}

func problem(w http.ResponseWriter, status int, code, param string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "param": param}})
}

// duplicateKey returns the path of the first repeated object key. Go keeps the
// last duplicate while other JSON readers may keep the first, so a duplicate
// could let the connector act on a field preflight never checked.
func duplicateKey(body []byte) string {
	dec := json.NewDecoder(bytes.NewReader(body))
	var walk func(path string) (string, error)
	walk = func(path string) (string, error) {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch tok {
		case json.Delim('{'):
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return "", err
				}
				name := key.(string)
				field := name
				if path != "" {
					field = path + "." + name
				}
				if seen[name] {
					return field, nil
				}
				seen[name] = true
				if dup, err := walk(field); dup != "" || err != nil {
					return dup, err
				}
			}
			_, err = dec.Token()
		case json.Delim('['):
			for i := 0; dec.More(); i++ {
				if dup, err := walk(fmt.Sprintf("%s[%d]", path, i)); dup != "" || err != nil {
					return dup, err
				}
			}
			_, err = dec.Token()
		}
		return "", err
	}
	dup, _ := walk("")
	return dup
}

// Keep the original request bytes for dispatch. Decoding is solely for preflight.
func validate(body []byte, model string) (string, string) {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil || req == nil {
		return "invalid_json", ""
	}
	if field := duplicateKey(body); field != "" {
		return "duplicate_field", field
	}
	for field := range req {
		switch field {
		case "stream", "model", "messages":
		default:
			return "unsupported_field", field
		}
	}
	var chosen string
	if json.Unmarshal(req["model"], &chosen) != nil || chosen != model {
		return "model_unavailable", "model"
	}
	var stream bool
	if json.Unmarshal(req["stream"], &stream) != nil || !stream {
		return "stream_required", "stream"
	}
	var messages []map[string]json.RawMessage
	if json.Unmarshal(req["messages"], &messages) != nil || len(messages) == 0 {
		return "invalid_history", "messages"
	}
	for i, m := range messages {
		path := fmt.Sprintf("messages[%d]", i)
		for k := range m {
			if k != "role" && k != "content" {
				return "unsupported_field", path + "." + k
			}
		}
		var role string
		if json.Unmarshal(m["role"], &role) != nil || (role != "system" && role != "user" && role != "assistant") {
			return "unsupported_role", path + ".role"
		}
		var content *string
		if json.Unmarshal(m["content"], &content) != nil || content == nil {
			return "text_required", path + ".content"
		}
	}
	return "", ""
}
func (g gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != chatPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		problem(w, 405, "method_not_allowed", "")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+g.key)) != 1 {
		problem(w, 401, "invalid_gateway_key", "")
		return
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		problem(w, 415, "json_required", "")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			problem(w, 413, "body_too_large", "")
		} else {
			problem(w, 400, "invalid_body", "")
		}
		return
	}
	if code, field := validate(body, g.model); code != "" {
		problem(w, 400, code, field)
		return
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		problem(w, 500, "request_id_unavailable", "")
		return
	}
	w.Header().Set("Gateway-Request-Id", hex.EncodeToString(id[:]))
	w.Header().Set("Gateway-Billing-Source", "unknown")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	up, err := http.NewRequestWithContext(ctx, "POST", g.endpoint+chatPath, bytes.NewReader(body))
	if err != nil {
		problem(w, 500, "invalid_connector", "")
		return
	}
	up.Header.Set("Content-Type", "application/json")
	up.Header.Set("Accept", "text/event-stream")
	up.Header.Set("Authorization", "Bearer "+g.upstreamKey)
	result, err := g.client.Do(up)
	if err != nil {
		problem(w, 502, "connector_unreachable_or_interrupted", "")
		return
	}
	defer result.Body.Close()
	if result.StatusCode != 200 {
		status := result.StatusCode
		if status < 400 || status > 599 {
			status = 502
		}
		code := "connector_rejected"
		switch status {
		case 401, 403:
			code = "check_connector_login"
		case 429:
			code = "rate_limit_cause_unknown"
		}
		problem(w, status, code, "")
		return
	}
	kind, _, _ := mime.ParseMediaType(result.Header.Get("Content-Type"))
	if kind != "text/event-stream" {
		problem(w, 502, "expected_event_stream", "")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	control := http.NewResponseController(w)
	if control.Flush() != nil {
		return
	}
	var stream terminalEvent
	buf := make([]byte, 16*1024)
	for {
		n, err := result.Body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			complete := stream.observe(buf[:n])
			if control.Flush() != nil {
				return
			}
			// End the response at [DONE]. A connector that holds the connection
			// open afterwards must not delay the client or turn into an abort.
			if complete {
				return
			}
		}
		if err != nil {
			if r.Context().Err() == nil {
				log.Printf("request %x: connector stream ended before [DONE]", id)
			}
			// Headers are committed. Abort the HTTP stream so generic clients
			// also see an interrupted body, even after receiving partial text.
			panic(http.ErrAbortHandler)
		}
	}
}
func localEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" || !net.ParseIP(u.Hostname()).IsLoopback() {
		return "", errors.New("connector must be a loopback HTTP origin, e.g. http://127.0.0.1:8317")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}
func main() {
	if len(os.Args) != 2 || os.Args[1] != "start" {
		fmt.Fprintln(os.Stderr, "usage: routeinlet start")
		os.Exit(2)
	}
	origin := os.Getenv("ROUTEINLET_CONNECTOR")
	if origin == "" {
		origin = "http://127.0.0.1:8317"
	}
	endpoint, err := localEndpoint(origin)
	if err != nil {
		log.Fatal(err)
	}
	g := gateway{endpoint: endpoint, model: os.Getenv("ROUTEINLET_MODEL"), key: os.Getenv("ROUTEINLET_KEY"), upstreamKey: os.Getenv("ROUTEINLET_CONNECTOR_KEY"),
		client: &http.Client{Transport: &http.Transport{Proxy: nil, DisableCompression: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if g.model == "" || g.key == "" || g.upstreamKey == "" {
		log.Fatal("set ROUTEINLET_MODEL, ROUTEINLET_KEY and ROUTEINLET_CONNECTOR_KEY")
	}
	srv := &http.Server{Addr: "127.0.0.1:8787", Handler: g, ReadTimeout: 15 * time.Second, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 135 * time.Second}
	log.Printf("Owner-run connectivity pilot on %s; funding unknown; no gateway fallback", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
