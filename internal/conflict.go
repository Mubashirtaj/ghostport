package internal

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// maxLineBuffer caps how much of a newline-less stream we hold before scanning
// it anyway, so a server that streams without line breaks can't grow the buffer
// without bound.
const maxLineBuffer = 64 * 1024

// conflictMarkers are lowercased phrases meaning "something already holds this
// address". Every ecosystem words it differently, so match on the phrase rather
// than on any single tool's exact format.
var conflictMarkers = []string{
	"already in use",                        // node, spring, rails, django, vite
	"eaddrinuse",                            // node, raw errno name
	"address in use",                        // python asyncio, uvicorn
	"port is already allocated",             // docker
	"failed to bind",                        // .net kestrel
	"only one usage of each socket address", // windows winsock
}

// conflictPattern catches the wordings that don't say "already", such as
// Flask's "Port 5000 is in use by another program." Requiring port/address/
// socket near "in use" keeps it from firing on unrelated prose.
var conflictPattern = regexp.MustCompile(`(?i)\b(?:port|address|socket)\b[^.\n]{0,40}\bin use\b`)

// portPattern extracts a port from a line already known to report a conflict.
type portPattern struct {
	re *regexp.Regexp
	// last takes the final match instead of the first. Host:port shaped text is
	// matched last-first because log lines are often timestamp-prefixed, and
	// "10:15:30" would otherwise win over the real port later in the line.
	last bool
}

var portPatterns = []portPattern{
	// uvicorn / asyncio: bind on address ('0.0.0.0', 8000)
	{re: regexp.MustCompile(`\(\s*['"][^'"]*['"]\s*,\s*(\d{1,5})\s*\)`)},
	// spring, vite, rails, webpack: "Port 8080 was already in use", `port 3000`
	{re: regexp.MustCompile(`(?i)\bport[:= ]\s*(\d{1,5})\b`)},
	// anything host:port shaped: :::5173, 0.0.0.0:8080, http://127.0.0.1:5000
	{re: regexp.MustCompile(`:(\d{1,5})\b`), last: true},
}

// urlPortPattern recovers a port from a startup banner. Some servers announce
// their address before failing to bind it and then report the failure without
// repeating the number — Django prints "Starting development server at
// http://127.0.0.1:8000/" and only then "Error: That port is already in use."
var urlPortPattern = regexp.MustCompile(`(?i)https?://[^\s/]*:(\d{1,5})`)

// ansiPattern strips terminal styling so escape sequences can't split the
// phrases and numbers we match on.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// ConflictSniffer is an io.Writer that watches a child process's output for a
// port collision while the bytes pass through to the terminal unchanged.
//
// It is safe for concurrent use: a command's stdout and stderr are written from
// separate goroutines.
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

// Flush scans whatever is left in the buffer. Servers commonly die without a
// trailing newline on their final error line, so this must run before the
// result is read.
func (s *ConflictSniffer) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buf) > 0 {
		s.scan(string(s.buf))
		s.buf = s.buf[:0]
	}
}

// Conflict reports whether the output announced a port collision, and the port
// it named. A conflict can be detected without a port when the message omits
// the number, in which case the caller falls back to PortFromArgs.
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

// attachedPortFlag matches the joined flag form: --port=8000, -p:3000.
var attachedPortFlag = regexp.MustCompile(`(?i)^--?(?:port|listen|publish|p)[:=](\d{1,5})$`)

// PortFromArgs guesses the port a command was going to bind by reading its own
// arguments. This covers the servers whose collision message names no number at
// all — Django's "Error: That port is already in use." being the common one.
//
// Flag and address forms win over bare numbers, which are a last resort since
// plenty of arguments are numeric without being ports.
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

		// Skip index 0: that's the executable, never a port.
		if bare == 0 && i > 0 {
			if port, ok := parsePort(arg); ok && port > 1023 {
				bare = port
			}
		}
	}

	return bare
}

// portFromAddr reads a port out of an address-shaped argument. Two shapes
// collide here: in "0.0.0.0:8000" the port follows the colon, while in docker's
// "-p 8080:80" the locally bound port comes first. A numeric left-hand side
// means the docker form.
func portFromAddr(arg string) (int, bool) {
	arg = strings.TrimSuffix(strings.TrimSuffix(arg, "/tcp"), "/udp")

	idx := strings.LastIndex(arg, ":")
	if idx < 0 {
		return 0, false
	}
	left, right := arg[:idx], arg[idx+1:]

	// Handles both "8080:80" and "127.0.0.1:8080:80".
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
