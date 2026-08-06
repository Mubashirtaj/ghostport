# 👻 GhostPort

**Stop guessing what's hogging your port.** GhostPort tells you — in plain English — exactly which process (or Docker container) is bound to a local port, how long it's been running, and lets you kill it on the spot.

```
👻 GhostPort v0.1.0
Port 8080 is BUSY
→ Process: node (PID: 8421)
→ Project: ~/code/my-app
→ Running: npm run dev
→ Docker: postgres-db container
Prompt: [k] Kill, [o] Open folder, [q] Quit
```

<!-- ![ghostport demo](docs/demo.gif) -->
> 📽️ _Demo GIF coming soon — replace this placeholder with `docs/demo.gif`._

## Features

- **Human-readable port ownership** — process name, PID, working directory, full command line, and uptime.
- **Docker awareness** — detects when a port is published by a running container and shows its name.
- **One-key actions** — kill the process, open its project folder, or quit, right from the prompt.
- **`ghostport list`** — a bird's-eye view of common dev ports (3000, 3001, 5173, 8000, 8080, 5432).
- **Cross-platform** — Linux, macOS, and Windows.

## Installation

### Quick install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/mubashirtaj/ghostport/main/install.sh | bash
```

This detects your OS/architecture, downloads the latest release, and installs it to `/usr/local/bin/ghostport`.

### Homebrew

```bash
brew install mubashirtaj/tap/ghostport
```

### Go install

```bash
go install github.com/mubashirtaj/ghostport@latest
```

### Manual download

Grab a prebuilt binary for your platform from the [releases page](https://github.com/mubashirtaj/ghostport/releases).

## Usage

```bash
# Inspect a port
ghostport 8080

# See the status of common dev ports at a glance
ghostport list

# Show version
ghostport --version
```

## Building from source

Requires Go 1.22+.

```bash
git clone https://github.com/mubashirtaj/ghostport.git
cd ghostport
go build -o ghostport .
```

## How it works

GhostPort uses [gopsutil](https://github.com/shirou/gopsutil) to enumerate TCP `LISTEN` sockets and resolve the owning process's name, working directory, command line, and start time — no `lsof`/`netstat` shelling out required. If `docker` is available on your `PATH`, GhostPort also cross-references `docker ps` to tell you when a port belongs to a container rather than a bare process.

## License

MIT
