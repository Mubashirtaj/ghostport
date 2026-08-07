package internal

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const maxLineBuffer = 64 * 1024

var conflictMarkers = []string{
	"already in use",                        // node, spring, rails, django, vite
	"eaddrinuse",                            // node, raw errno name
	"address in use",                        // python asyncio, uvicorn
	"port is already allocated",             // docker
	"failed to bind",                        // .net kestrel
	"only one usage of each socket address", // windows winsock
}

var conflictPattern = regexp.MustCompile(`(?i)\b(?:port|address|socket)\b[^.\n]{0,40}\bin use\b`)

type portPattern struct {
	re *regexp.Regexp

	last bool
}

var portPatterns = []portPattern{
	{re: regexp.MustCompile(`\(\s*['"][^'"]*['"]\s*,\s*(\d{1,5})\s*\)`)},
	{re: regexp.MustCompile(`(?i)\bport[:= ]\s*(\d{1,5})\b`)},
	{re: regexp.MustCompile(`:(\d{1,5})\b`), last: true},
}

var urlPortPattern = regexp.MustCompile(`(?i)https?://[^\s/]*:(\d{1,5})`)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type ConflictSniffer struct {
	mu sync.Mutex

	buf []byte

	sawConflict bool
	port        int
	bannerPort  int
}

func NewConflictSniffer() *ConflictSniffer {
	return &ConflictSniffer{}
}

func (s *ConflictSniffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buf = append(s.buf, p...)

	for {
		i := bytes.IndexByte(s.buf, '\n')
		if i < 0 {
			break
		}
		s.scan(string(s.buf[:i]))
		s.buf = s.buf[i+1:]
	}

	if len(s.buf) > maxLineBuffer {
		s.scan(string(s.buf))
		s.buf = s.buf[:0]
	}

	return len(p), nil
}

func (s *ConflictSniffer) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buf) > 0 {
		s.scan(string(s.buf))
		s.buf = s.buf[:0]
	}
}

func (s *ConflictSniffer) Conflict() (port int, sawConflict bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.port != 0 {
		return s.port, s.sawConflict
	}
	return s.bannerPort, s.sawConflict
}

func (s *ConflictSniffer) scan(line string) {
	line = ansiPattern.ReplaceAllString(line, "")

	if port, ok := extractPort(urlPortPattern, line, false); ok && s.bannerPort == 0 {
		s.bannerPort = port
	}

	if !hasConflictMarker(line) {
		return
	}
	s.sawConflict = true

	if s.port != 0 {
		return
	}
	for _, p := range portPatterns {
		if port, ok := extractPort(p.re, line, p.last); ok {
			s.port = port
			return
		}
	}
}

func hasConflictMarker(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range conflictMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return conflictPattern.MatchString(lower)
}

func extractPort(re *regexp.Regexp, line string, last bool) (int, bool) {
	matches := re.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return 0, false
	}

	if last {
		for i := len(matches) - 1; i >= 0; i-- {
			if port, ok := parsePort(matches[i][1]); ok {
				return port, true
			}
		}
		return 0, false
	}

	for _, m := range matches {
		if port, ok := parsePort(m[1]); ok {
			return port, true
		}
	}
	return 0, false
}

func parsePort(s string) (int, bool) {
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

var attachedPortFlag = regexp.MustCompile(`(?i)^--?(?:port|listen|publish|p)[:=](\d{1,5})$`)

func PortFromArgs(args []string) int {
	var bare int

	for i, arg := range args {
		if m := attachedPortFlag.FindStringSubmatch(arg); m != nil {
			if port, ok := parsePort(m[1]); ok {
				return port
			}
		}

		if port, ok := portFromAddr(arg); ok {
			return port
		}

		// Separated form: --port 8000, -p 3000
		if isPortFlag(arg) && i+1 < len(args) {
			if port, ok := parsePort(args[i+1]); ok {
				return port
			}
			if port, ok := portFromAddr(args[i+1]); ok {
				return port
			}
		}

		if bare == 0 && i > 0 {
			if port, ok := parsePort(arg); ok && port > 1023 {
				bare = port
			}
		}
	}

	return bare
}

func portFromAddr(arg string) (int, bool) {
	arg = strings.TrimSuffix(strings.TrimSuffix(arg, "/tcp"), "/udp")

	idx := strings.LastIndex(arg, ":")
	if idx < 0 {
		return 0, false
	}
	left, right := arg[:idx], arg[idx+1:]

	if port, ok := parsePort(left[strings.LastIndex(left, ":")+1:]); ok {
		return port, true
	}

	return parsePort(right)
}

func isPortFlag(arg string) bool {
	switch strings.ToLower(arg) {
	case "--port", "-port", "-p", "--listen", "--publish":
		return true
	}
	return false
}
