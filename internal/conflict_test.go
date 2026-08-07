package internal

import "testing"

func TestSnifferRealWorldErrors(t *testing.T) {
	tests := []struct {
		name string
		out  string
		port int
	}{
		{
			name: "node listen EADDRINUSE",
			out:  "Error: listen EADDRINUSE: address already in use :::5173\n",
			port: 5173,
		},
		{
			name: "node bound to explicit host",
			out:  "Error: listen EADDRINUSE: address already in use 127.0.0.1:3000\n",
			port: 3000,
		},
		{
			name: "go net listen",
			out:  "listen tcp :8080: bind: address already in use\n",
			port: 8080,
		},
		{
			name: "docker port already allocated",
			out:  "Bind for 0.0.0.0:8080 failed: port is already allocated\n",
			port: 8080,
		},
		{
			name: "docker userland proxy",
			out:  "driver failed programming external connectivity on endpoint web: Error starting userland proxy: listen tcp4 0.0.0.0:3000: bind: address already in use\n",
			port: 3000,
		},
		{
			name: "uvicorn address tuple",
			out:  "[Errno 98] error while attempting to bind on address ('0.0.0.0', 8000): address already in use\n",
			port: 8000,
		},
		{
			name: "spring boot",
			out:  "Web server failed to start. Port 8080 was already in use.\n",
			port: 8080,
		},
		{
			name: "rails puma",
			out:  `Address already in use - bind(2) for "127.0.0.1" port 3000` + "\n",
			port: 3000,
		},
		{
			name: "dotnet kestrel",
			out:  "Failed to bind to address http://127.0.0.1:5000: address already in use.\n",
			port: 5000,
		},
		{
			name: "nginx",
			out:  "nginx: [emerg] bind() to 0.0.0.0:80 failed (98: Address already in use)\n",
			port: 80,
		},
		{
			name: "flask",
			out:  "OSError: [Errno 98] Address already in use\nPort 5000 is in use by another program.\n",
			port: 5000,
		},
		{
			name: "windows winsock",
			out:  "listen tcp 0.0.0.0:4200: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.\n",
			port: 4200,
		},
		{
			name: "timestamp prefix does not win over the real port",
			out:  "[10:15:30] Error: listen EADDRINUSE: address already in use :::5173\n",
			port: 5173,
		},
		{
			name: "ansi coloured output",
			out:  "\x1b[31mError: listen EADDRINUSE: address already in use :::4000\x1b[0m\n",
			port: 4000,
		},
		{
			name: "django names no port, banner supplies it",
			out: "Watching for file changes with StatReloader\n" +
				"Starting development server at http://127.0.0.1:8000/\n" +
				"Error: That port is already in use.\n",
			port: 8000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewConflictSniffer()
			if _, err := s.Write([]byte(tc.out)); err != nil {
				t.Fatalf("Write: %v", err)
			}
			s.Flush()

			port, sawConflict := s.Conflict()
			if !sawConflict {
				t.Fatalf("no conflict detected in %q", tc.out)
			}
			if port != tc.port {
				t.Errorf("port = %d, want %d", port, tc.port)
			}
		})
	}
}

func TestSnifferIgnoresUnrelatedOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{"compile error", "SyntaxError: Unexpected token '}' at line 42\n"},
		{"missing module", "Error: Cannot find module 'express'\n"},
		{"normal startup", "VITE v5.0.0 ready in 340 ms\n  ➜ Local: http://localhost:5173/\n"},
		{"test failure", "FAIL src/app.test.js (3 failed, 8 passed)\n"},
		{"permission denied", "listen tcp :80: bind: permission denied\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewConflictSniffer()
			if _, err := s.Write([]byte(tc.out)); err != nil {
				t.Fatalf("Write: %v", err)
			}
			s.Flush()

			if _, sawConflict := s.Conflict(); sawConflict {
				t.Errorf("falsely detected a port conflict in %q", tc.out)
			}
		})
	}
}

func TestSnifferSplitWrites(t *testing.T) {
	chunks := []string{
		"Error: listen EADD",
		"RINUSE: address alr",
		"eady in use :::5173",
	}

	s := NewConflictSniffer()
	for _, c := range chunks {
		if _, err := s.Write([]byte(c)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if _, sawConflict := s.Conflict(); sawConflict {
		t.Error("detected conflict before Flush on an unterminated line")
	}

	s.Flush()

	port, sawConflict := s.Conflict()
	if !sawConflict || port != 5173 {
		t.Errorf("after Flush: port = %d, sawConflict = %v; want 5173, true", port, sawConflict)
	}
}

func TestSnifferKeepsFirstConflict(t *testing.T) {
	s := NewConflictSniffer()
	out := "Error: listen EADDRINUSE: address already in use :::3000\n" +
		"Error: That port is already in use.\n"

	if _, err := s.Write([]byte(out)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	s.Flush()

	if port, _ := s.Conflict(); port != 3000 {
		t.Errorf("port = %d, want 3000", port)
	}
}

func TestPortFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		port int
	}{
		{"no port anywhere", []string{"npm", "run", "dev"}, 0},
		{"separated long flag", []string{"go", "run", ".", "--port", "8080"}, 8080},
		{"attached long flag", []string{"vite", "--port=5173"}, 5173},
		{"short flag", []string{"serve", "-p", "4000"}, 4000},
		{"bind address", []string{"python", "manage.py", "runserver", "0.0.0.0:8000"}, 8000},
		{"hostname address", []string{"uvicorn", "app:app", "--host", "localhost:9000"}, 9000},
		{"docker host mapping keeps host port", []string{"docker", "run", "-p", "8080:80", "nginx"}, 8080},
		{"docker mapping with bind ip", []string{"docker", "run", "-p", "127.0.0.1:8080:80", "nginx"}, 8080},
		{"docker mapping with protocol", []string{"docker", "run", "-p", "5432:5432/tcp", "postgres"}, 5432},
		{"bare port argument", []string{"python", "manage.py", "runserver", "8000"}, 8000},
		{"low bare numbers are not ports", []string{"node", "cluster.js", "4"}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PortFromArgs(tc.args); got != tc.port {
				t.Errorf("PortFromArgs(%q) = %d, want %d", tc.args, got, tc.port)
			}
		})
	}
}
