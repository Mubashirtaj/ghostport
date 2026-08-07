# 👻 GhostPort

**Stop guessing what's hogging your port.** GhostPort tells you — in plain English — exactly which process (or Docker container) is bound to a local port, how long it's been running, and lets you kill it on the spot.

```
👻 GhostPort v0.1.5
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
- **`ghostport run`** — wrap any dev command; when it dies on a port conflict, GhostPort opens the prompt for that port and restarts your command once it's free.
- **`ghostport list`** — a bird's-eye view of common dev ports (3000, 3001, 5173, 8000, 8080, 5432).
- **Cross-platform** — Linux, macOS, and Windows.

## Automatic mode: `ghostport run`

Prefix whatever you already type. GhostPort passes the output straight through, and only steps in if the command exits because its port was taken.

```bash
ghostport run npm run dev
```

```
> vite
Error: listen EADDRINUSE: address already in use :::5173

👻 GhostPort v0.1.5
  detected port conflict on 5173
╭─────────────────────────────────────╮
│  Port 5173 is BUSY                  │
│                                     │
│  → Process: node (PID: 8421)        │
│  → Project: ~/code/old-app          │
│  → Running: npm run dev             │
│  → Uptime: 2h 14m ago               │
│                                     │
│  Prompt: [k] Kill, [o] Open, [q] Quit │
╰─────────────────────────────────────╯
> k
✓ Killed node (PID: 8421)
↻ Retrying: npm run dev

  VITE ready in 340 ms
  ➜ Local: http://localhost:5173/
```

It reads the port out of the error message, so it works with whatever you run:

```bash
ghostport run docker compose up
ghostport run python manage.py runserver
ghostport run go run ./cmd/server
ghostport run ./mvnw spring-boot:run
ghostport run rails server
ghostport run dotnet run
```

Recognised wordings include Node's `EADDRINUSE`, Go's `bind: address already in use`, Docker's `port is already allocated`, Kestrel's `Failed to bind to address`, Spring's `Port 8080 was already in use`, Windows' `Only one usage of each socket address…`, and Django's `That port is already in use.` — the last of which names no port, so GhostPort falls back to the port in your command's arguments or startup banner.

Notes:

- Anything that isn't a port conflict is left alone: output passes through and GhostPort exits with the same code your command did.
- Pass `--no-retry` to inspect the port without restarting the command afterwards.
- Put GhostPort's own flags before the command; use `--` if your command's flags would otherwise be ambiguous:

  ```bash
  ghostport run --no-retry npm run dev
  ghostport run -- go run . --port 8080
  ```

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

# Run a command and auto-inspect the port if it conflicts
ghostport run npm run dev

# See the status of common dev ports at a glance
ghostport list

# Show version
ghostport --version
```

## Building from source

Requires Go 1.26+.

```bash
git clone https://github.com/mubashirtaj/ghostport.git
cd ghostport
go build -o ghostport .
```

## How it works

GhostPort uses [gopsutil](https://github.com/shirou/gopsutil) to enumerate TCP `LISTEN` sockets and resolve the owning process's name, working directory, command line, and start time — no `lsof`/`netstat` shelling out required. If `docker` is available on your `PATH`, GhostPort also cross-references `docker ps` to tell you when a port belongs to a container rather than a bare process.

`ghostport run` starts your command with its stdin, stdout, and stderr attached to your terminal as usual, while tee-ing the output through a scanner that watches for the phrases servers use to report a bind collision. Nothing is buffered or rewritten — you see your command's output exactly as you would without the wrapper. Detection only matters once the command has exited non-zero, so a server that recovers on its own is never interrupted.

## License

MIT
