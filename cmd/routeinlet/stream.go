package main

import "bytes"

// terminalEvent recognizes a dispatched SSE event whose sole data field is
// [DONE]. Keep only the prefix needed for comparison, even for large deltas.
type terminalEvent struct {
	line     [len("data: [DONE]") + 1]byte
	size     int
	afterCR  bool
	hasData  bool
	terminal bool
}

// observe returns the prefix safe to forward and whether it ends at [DONE].
// Bytes after the terminating blank line must never reach the client, even
// when the connector supplies them in the same read.
func (s *terminalEvent) observe(chunk []byte) (int, bool) {
	for i, b := range chunk {
		if b == '\n' && s.afterCR {
			s.afterCR = false
			continue
		}
		s.afterCR = b == '\r'
		if b != '\r' && b != '\n' {
			if s.size < len(s.line) {
				s.line[s.size] = b
				s.size++
			}
			continue
		}
		line := s.line[:s.size]
		s.size = 0
		if len(line) == 0 {
			if s.terminal {
				end := i + 1
				// Preserve a complete CRLF already in this chunk. A lone CR
				// also dispatches the event, so never wait for another read.
				if b == '\r' && end < len(chunk) && chunk[end] == '\n' {
					end++
				}
				return end, true
			}
			s.hasData = false
		} else if bytes.HasPrefix(line, []byte("data:")) || string(line) == "data" {
			s.terminal = !s.hasData && (string(line) == "data: [DONE]" || string(line) == "data:[DONE]")
			s.hasData = true
		}
	}
	return len(chunk), false
}
