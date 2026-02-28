**PV3.dev CLI Product Requirements Document (PRD)**  
**Feature: Local Docker Sandbox Mode (v1.0)**  
**Version:** 1.0 (MVP)  
**Date:** February 23, 2026  
**Author:** Grok (for Claude one-shot implementation)  
**Status:** Ready for implementation  

---

### 1. Executive Summary
pv3.dev currently lets users run **any** command safely with `pv3 run <command>` inside a remote microVM (full isolation: no host FS access, no secrets, no network egress, <100ms boot, auto-destroy).

**New capability:** Add a **zero-latency local Docker mode** so developers can use `pv3` as their **daily driver** for trusted dev workflows (especially long-running dev servers with hot reload).  

Command examples that must work after this PR:
```bash
pv3 dev                     # auto-runs "dev" script from package.json in local Docker
pv3 --local run npm run dev
pv3 --local run yarn dev
pv3 dev --port 4200
pv3 dev --no-net            # paranoid mode
```

The original `pv3 run` stays **unchanged** (still uses microVM).  
This turns pv3 into **the secure default way every engineer runs code** — local Docker for speed + hot reload, cloud microVM for maximum paranoia.

**Goal:** Ship in one Claude session → full working, production-ready feature.

---

### 2. Goals & Objectives
**Business Goals**
- Become the default CLI for safe dev execution (local + cloud).
- Protect every engineering team from supply-chain attacks while keeping native dev UX.
- Massive adoption boost: devs will alias `npm` → `pv3` or use `pv3 dev` daily.
- High-stakes teams (fintech, crypto, defense) adopt as company standard.

**Product Goals**
- Seamless UX: feels exactly like running on host (hot reload, localhost ports, live logs).
- Bulletproof security: stops crypto drainers, keyloggers, secret exfil, persistent malware.
- Zero friction: auto-fallback to cloud if Docker missing.
- Single static binary (no new deps).

**Success Metrics (MVP)**
- 100% of `npm run dev` / `yarn dev` / `pnpm dev` workflows work with hot reload.
- Container teardown on Ctrl+C in <2s.
- No host pollution (node_modules stays inside container unless user wants).
- <3s cold start on macOS/Linux.

---

### 3. Target Users & Personas
- **Daily Devs** — Run `pv3 dev` instead of `npm run dev`.
- **Security Teams** — Enforce `pv3` via shell alias or pre-commit hook.
- **Contractors / Open-Source Contributors** — `pv3 --local run ...` on any repo.
- **Fintech/Crypto Engineers** — Use `--no-net` for audited code.

---

### 4. User Flows & Commands (Exact Spec)

#### New top-level commands
```bash
pv3 dev [flags]                  # Sugar for package.json "dev" script
pv3 --local run <command> [args] # Explicit local Docker mode
pv3 run <command>                # UNCHANGED — still microVM
```

#### Flags (all apply to `dev` and `--local run`)
| Flag              | Default          | Description |
|-------------------|------------------|-----------|
| `--port <n>`      | Auto (3000-3999) | Publish specific port or range |
| `--no-net`        | false            | `--network=none` (no internet at all) |
| `--ephemeral`     | false            | `node_modules` lives only in container (host stays clean) |
| `--cpus <n>`      | 4                | CPU limit |
| `--memory <n>g`   | 6g               | Memory limit |
| `--image <str>`   | Auto-detect      | Override image (e.g. `node:22-bookworm`) |
| `--cloud`         | false            | Force cloud microVM even if Docker present |
| `--verbose`       | false            | Show docker run command |

#### Auto-detection logic (for `pv3 dev`)
1. Read `./package.json`
2. If `"scripts"."dev"` or `"scripts"."start"` or `"scripts"."serve"` exists → run it.
3. Auto-pick image:
   - `package.json` engines.node → `node:<version>-alpine`
   - Python (requirements.txt or pyproject.toml) → `python:3.12-slim`
   - Go (go.mod) → `golang:1.23`
   - Fallback → your existing `pv3/sandbox:latest` (or `node:22-alpine`)

#### Runtime Behavior (MUST)
- Volume mount: `$(pwd):/workspace:delegated` (macOS) or `:cached` (Linux)
- Working dir: `/workspace`
- User: exact host UID:GID → no permission issues
- `--rm` + auto-kill on SIGINT/SIGTERM
- Interactive TTY (`-it`)
- Live logs streamed to terminal
- On exit: print "Environment vaporized ✅ (took Xms)"
- Port detection: after start, scan container ports and print `→ App ready at http://localhost:XXXX`
- Signal forwarding: Ctrl+C sends SIGINT to app (Vite/Next.js shut down cleanly)

---

### 5. Technical Requirements & Architecture

**Language:** **Go** (required for one-shot).  
If current CLI is not Go, the entire new code path + wrapper must be written in Go and compiled into the existing binary (or full port — preferred).

**Dependencies (only these)**
- `github.com/docker/docker/client` (or just `os/exec` for ultra-MVP simplicity — **prefer exec for first pass**)
- `github.com/google/uuid`
- Standard library only otherwise

**Core Package Structure (add these files)**
```
cmd/pv3/
  local/
    runner.go          # main Docker logic
    detector.go        # package.json + runtime detection
    ports.go           # free port scanner
  cli/
    dev.go             # cobra command for "dev"
    local.go           # --local flag handler
```

**Docker Run Template (exact flags)**
```bash
docker run --rm -it \
  --name pv3-$(whoami)-$(basename $(pwd))-$(uuid) \
  --user $(id -u):$(id -g) \
  -v "$(pwd):/workspace:delegated" \
  -w /workspace \
  -p 3000-3999:3000-3999 \   # or specific
  --read-only \
  --tmpfs /tmp:exec \
  --cap-drop=ALL \
  --security-opt no-new-privileges:true \
  --cpus=4 --memory=6g \
  --network=none \           # if --no-net
  -e NODE_ENV=development \
  node:22-alpine \
  sh -c "npm ci --prefer-offline && npm run dev"
```

**Error Handling**
- Docker not installed/running → nice message + "Falling back to cloud microVM..." + link to Docker install
- Permission issues → clear message
- Port in use → auto-pick next free port
- Container dies unexpectedly → show exit code + logs

---

### 6. Non-Functional Requirements
- **Security (hard requirements)**
  - Never mounts anything outside current directory
  - Runs as non-root (host UID)
  - `--read-only` + tmpfs everywhere else
  - No capability to escalate privileges
- **Performance**
  - Cold start <5s on M1/M2 Mac
  - Hot reload latency identical to native
- **Platform Support**
  - macOS (Docker Desktop)
  - Linux (native Docker)
  - Windows (Docker Desktop + WSL2) — paths handled automatically
- **Binary**
  - Single static binary (same as today)
  - `install.sh` unchanged

---

### 7. Out of Scope (MVP)
- Building custom images (user can `--image` but no Dockerfile auto-build)
- Persistent volumes
- Multi-container (docker-compose)
- GUI / VS Code extension
- Automatic aliasing (`npm` → `pv3`)
- Windows native (non-WSL) full support

---

### 8. Assumptions & Dependencies
- Docker Desktop / Docker Engine is the only new runtime dependency.
- Existing `pv3 run` cloud backend stays untouched.
- You have access to the current CLI source (Go recommended).

---

### 9. Implementation Notes for Claude (One-Shot)
- Use Cobra for CLI if already present, else add minimal flag parsing.
- Start with `os/exec.Command("docker", "run", ...)` — easiest and most reliable for MVP.
- Make it modular so later we can swap to Docker SDK.
- Include `pv3 doctor` command that checks Docker + permissions.
- Add colorful output (use `github.com/fatih/color` if you want, or just fmt with ANSI).
- Full help text + examples in `--help`.

---

### 10. Acceptance Criteria
1. `pv3 dev` on a Next.js/Vite/Vue/React repo works with hot reload and browser access.
2. Ctrl+C cleanly stops container and prints vaporized message.
3. `ls ~/.ssh` inside container fails (no host access).
4. `--no-net` prevents `curl google.com`.
5. `--ephemeral` leaves host `node_modules` untouched.
6. No Docker → seamless cloud fallback.
7. All new code is in `local/` package, zero breakage to existing `pv3 run`.


High-level Go code structure suggestion (add under existing project)
pv3/
├── cmd/
│   └── pv3/
│       ├── root.go               # existing
│       ├── run.go                # existing cloud path
│       └── local/
│           ├── dev_cmd.go        # cobra command "pv3 dev"
│           ├── local_run_cmd.go  # handler for --local run ...
│           ├── runner.go         # builds & runs docker cmd, watches signals
│           ├── detector.go       # reads package.json, picks image/script
│           ├── ports.go          # finds free port in 3000-3999 or custom
│           └── doctor.go         # "pv3 doctor" checks docker + perms
├── go.mod
└── ...