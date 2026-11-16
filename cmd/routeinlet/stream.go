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

func (s *terminalEvent) observe(chunk []byte) bool {
	for _, b := range chunk {
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
				return true
			}
			s.hasData = false
		} else if bytes.HasPrefix(line, []byte("data:")) || string(line) == "data" {
			s.terminal = !s.hasData && (string(line) == "data: [DONE]" || string(line) == "data:[DONE]")
			s.hasData = true
		}
	}
	return false
}
