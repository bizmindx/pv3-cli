**PV3.dev CLI — Revised PRD**
**Feature: Local Docker Sandbox Mode (MVP)**
**Version:** 2.0
**Date:** February 28, 2026
**Status:** Ready for implementation

---

### 1. What This Is

`pv3 dev` runs your project's dev server inside a Docker container that feels identical to running it natively. Same terminal output, same hot reload, same localhost URLs. The only difference: your code runs isolated from your host system — no rogue dependency can touch your SSH keys, browser cookies, or crypto wallets.

```bash
pv3 dev                        # reads package.json, runs "dev" script
pv3 dev --port 5173            # publish a specific port
pv3 dev --no-net               # fully offline (no network access)
pv3 dev --image node:20-slim   # override the default image
pv3 dev --verbose              # show the docker run command being executed
```

The existing `pv3 run` command (cloud microVM) stays completely untouched.

---

### 2. Success Criteria

1. Running `pv3 dev` in a Next.js, Vite, or CRA project produces identical terminal output to `npm run dev`.
2. Hot reload works — edit a file, see the change in the browser.
3. `http://localhost:<port>` works in the host browser.
4. Ctrl+C cleanly stops the dev server and removes the container in <2s.
5. No permission errors on mounted files (container runs as host user).
6. `ls ~/.ssh` inside the container fails (host filesystem not accessible).
7. The developer never needs to know Docker is involved.

---

### 3. Target Users (MVP)

Frontend and full-stack JavaScript/TypeScript developers who run `npm run dev` (or yarn/pnpm equivalents) daily. That's the only workflow this MVP targets.

---

### 4. Command Spec

#### `pv3 dev [flags]`

Reads `package.json` in the current directory, finds the dev script, runs it inside a Docker container with live terminal output.

**Script resolution order:**
1. `scripts.dev`
2. `scripts.start`
3. `scripts.serve`
4. If none found, exit with: `No dev script found in package.json`

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port <n>` | `5173` | Host port to publish (maps to same port in container) |
| `--no-net` | `false` | Disable all network access (`--network=none`) |
| `--image <str>` | `node:22-bookworm-slim` | Override container image |
| `--verbose` | `false` | Print the full `docker run` command before executing |

That's it. Four flags.

**Why default to port 5173:** Vite dominates the 2026 frontend ecosystem — create-vite, Nuxt, SvelteKit, Astro all use it. Next.js users on port 3000 pass `--port 3000`. The majority default wins.

#### Enterprise Flags (Post-MVP, Sales-Led)

These flags are planned for enterprise customers (available via sales engagement, not in the open MVP):

| Flag | Default | Description |
|------|---------|-------------|
| `--cpus <n>` | `4` | CPU core limit |
| `--memory <n>g` | `6g` | Memory limit |
| `--ephemeral` | `false` | node_modules lives only inside container, host stays clean |
| `--cloud` | `false` | Force cloud microVM even if Docker is present |
| `--read-only` | `false` | Read-only root filesystem (strict security, requires tmpfs config) |

These will be gated behind license/config and documented separately. The MVP ships without them — resource limits (`--cpus=4`, `--memory=6g`) are hardcoded to sane defaults.

---

### 5. How It Works

#### The Docker Command

```bash
docker run \
  --rm \
  -it \
  --name pv3-dev-<project>-<short-id> \
  --user <host-uid>:<host-gid> \
  -v "<cwd>:/workspace:delegated" \
  -w /workspace \
  -p <port>:<port> \
  --cap-drop=ALL \
  --security-opt no-new-privileges:true \
  --cpus=4 \
  --memory=6g \
  --env-file .env \
  -e NODE_ENV=development \
  -e TERM=$TERM \
  node:22-bookworm-slim \
  sh -c "<pm> run <script-name>"
```

Note: the command is just `npm run dev` (or yarn/pnpm equivalent). No `npm ci` or install step. If dependencies aren't installed, the dev script fails exactly like it would natively — that's the correct behavior.

Key design choices:

- **No `--read-only`**. Package managers write to `~/.npm`, `~/.cache`, `~/.yarn` and other locations outside `/workspace`. A read-only root filesystem breaks real-world npm/yarn/pnpm workflows. Security is enforced through capability dropping and privilege restrictions instead.

- **`node:22-bookworm-slim` not alpine**. Alpine uses musl libc. Native npm packages (sharp, esbuild, bcrypt, canvas, node-gyp builds) frequently fail or require extra tooling on Alpine. Bookworm-slim uses glibc and just works. The image is ~60MB larger but eliminates an entire class of "works on my machine but not in pv3" issues.

- **`delegated` mount consistency**. On macOS (Docker Desktop), `delegated` gives the best filesystem performance for dev servers watching files. The container may briefly lag behind host writes, which is fine for hot reload (frameworks poll or use inotify with tolerance). On Linux, Docker ignores this flag and uses native bind mounts. Note: Docker Desktop on macOS with VirtioFS (default since ~2023) has significantly reduced bind mount overhead (~3x vs native, down from 5-6x). `:delegated` remains the right default — it tells Docker we tolerate eventual consistency, which pairs well with how file watchers already work.

- **File watcher compatibility**. If users report hot reload lag on macOS, the fix is adding `CHOKIDAR_USEPOLLING=true` (webpack/CRA) or `WATCHPACK_POLLING=true` (Next.js) as container env vars. Not needed for Vite (uses its own optimized watcher). This is a post-MVP enhancement — don't block on it, but it's a one-line addition when needed.

- **Single port, not a range**. Publishing 1000 ports (`3000-3999`) is slow, wasteful, and conflicts with anything already bound on the host. One port is predictable. Default is 5173 (Vite ecosystem standard). If the dev server uses a different port, the user passes `--port`.

- **`--env-file .env`**. Real projects use `.env` files for API keys, database URLs, feature flags. Without this, the dev server experience breaks immediately. Only loaded if `.env` exists in the project root — no error if missing.

- **`-e TERM=$TERM`**. Ensures colored output, progress bars, and interactive prompts render correctly inside the container. This is what makes Vite's colorful output, Next.js's compilation status, and webpack progress bars look native.

#### Container Naming

`pv3-dev-<project>-<8-char-random>` where `<project>` is the basename of the current directory, sanitized to `[a-z0-9-]+` (lowercase, strip special chars, replace spaces/underscores with hyphens, truncate to 30 chars). Docker container names must match `[a-zA-Z0-9][a-zA-Z0-9_.-]+` — sanitizing avoids cryptic Docker errors.

Example: directory `My Cool App!` becomes container `pv3-dev-my-cool-app-a1b2c3d4`. Uses Go's `math/rand` — no UUID dependency needed.

#### Signal Handling

1. Trap `SIGINT` (Ctrl+C) and `SIGTERM` in the Go process.
2. Forward to the container via `docker stop <name>` with a 5s timeout.
3. The `--rm` flag handles cleanup automatically.
4. If the container doesn't stop within 5s, `docker kill` it.
5. Print: `Container stopped. (took <N>ms)`

#### Error Cases

| Situation | Behavior |
|-----------|----------|
| Docker not installed | Print: `Docker is required but not found. Install it: https://docs.docker.com/get-docker/` then exit 1 |
| Docker daemon not running | Print: `Docker is installed but not running. Start Docker Desktop and try again.` then exit 1 |
| No package.json | Print: `No package.json found in current directory.` then exit 1 |
| No dev/start/serve script | Print: `No dev, start, or serve script found in package.json.` then exit 1 |
| Port already in use | Pre-flight check via `net.Listen("tcp", "127.0.0.1:<port>")` in Go before launching Docker. Print: `Port <N> is already in use. Try: pv3 dev --port <N+1>` then exit 1. Catching this early gives a clean error instead of a cryptic Docker bind failure. |
| .env file missing | Silently skip `--env-file` flag. Not an error. |
| Container crashes | Stream whatever output the container produced, show exit code, exit with same code |

No cloud fallback. No magic. Clear errors, obvious fixes.

---

### 6. Security Model

**What's enforced:**
- `--cap-drop=ALL` — container has zero Linux capabilities
- `--security-opt no-new-privileges:true` — cannot escalate privileges via setuid/setgid
- `--user <uid>:<gid>` — runs as the host user, not root
- Only `$(pwd)` is mounted — no access to home directory, SSH keys, or anything outside the project
- `--cpus=4 --memory=6g` — resource limits prevent runaway processes

**What's NOT enforced (intentionally):**
- Network access is allowed by default (dev servers need it: npm fetches, API calls, WebSocket connections)
- Filesystem is not read-only (package managers need to write caches)
- The project directory is writable (that's the point — hot reload requires it)

**`--no-net` mode:** Adds `--network=none`. Completely air-gapped. Use for audited codebases where you trust the code but want zero exfiltration risk. Note: `npm install` will not work in this mode — dependencies must already be installed.

---

### 7. Technical Implementation

**Language:** Go
**CLI Framework:** Cobra (if already in use) or minimal `flag` package
**Docker interaction:** `os/exec` calling the `docker` binary directly. No Docker SDK dependency.
**External dependencies:** None beyond Go stdlib and Cobra.

#### File Structure

```
cmd/pv3/
  local/
    dev.go          # Cobra command definition for "pv3 dev"
    runner.go       # Builds docker args, executes, handles signals
    project.go      # Reads package.json, resolves dev script
```

Three files. That's the entire feature.

#### project.go

```go
// ReadDevScript reads package.json and returns the dev command to run.
// Resolution order: scripts.dev > scripts.start > scripts.serve
// Returns the script value (e.g., "next dev" or "vite") and the
// package manager detected from lockfiles (npm/yarn/pnpm).
```

Package manager detection:
- `pnpm-lock.yaml` exists -> use `pnpm`
- `yarn.lock` exists -> use `yarn`
- Otherwise -> use `npm`

The resolved command becomes: `<pm> run <script-name>`

**No forced install.** The command is just `<pm> run dev` — not `npm ci && npm run dev`. If `node_modules` is missing, the dev script itself will fail with a clear error. That's the expected behavior — it matches what happens when you run `npm run dev` natively without installing first. Don't papier-mache over it.

#### runner.go

Builds the `[]string` of docker arguments, creates `exec.Cmd`, connects stdin/stdout/stderr directly to the parent process (no piping, no buffering — raw terminal passthrough), sets up signal forwarding, and runs.

#### dev.go

Cobra command wiring. Parses flags, calls project.go for script resolution, calls runner.go to execute.

---

### 8. What's Explicitly Out of Scope

- Python, Go, Ruby, or any non-Node.js runtime detection
- `pv3 doctor` diagnostic command
- Cloud microVM fallback
- `--ephemeral` flag (node_modules management)
- `--read-only` filesystem
- Windows support
- Multi-container / docker-compose
- Custom Dockerfile building
- Persistent named volumes
- GUI or VS Code extension
- Port auto-detection / port range publishing
- Automatic shell aliasing

These are all valid future features. None of them are needed to ship a tool that makes `pv3 dev` feel native for JS/TS developers.

---

### 9. Future Considerations (Post-MVP)

Once the MVP is validated with real users:

1. **Port auto-detection** — watch container stdout via `docker logs -f` for common patterns (`Local: http://localhost:XXXX`, `ready on`, `listening on`) and print it prominently. Implementable with simple regex on log stream. Alternatively, `docker port <name>` after startup.
2. **Smart port default** — read `vite.config.js` / `next.config.js` for `server.port` config and use that as the default instead of 5173.
3. **File watcher polling env vars** — auto-inject `CHOKIDAR_USEPOLLING=true` / `WATCHPACK_POLLING=true` when running on macOS to eliminate hot reload lag edge cases for webpack/Next.js projects.
4. **Multi-port** — `--port 3000,3001` for apps that use multiple ports (e.g., dev server + API).
5. **Python/Go support** — runtime detection + appropriate base images.
6. **`pv3 doctor`** — check Docker version, disk space, permissions, VirtioFS status.
7. **Cloud fallback** — when the cloud backend exists, offer it as an option when Docker is missing.
8. **`--ephemeral`** — run with node_modules inside the container only (volume-based).
9. **Config file** — `.pv3.yaml` in project root for default flags so teams can standardize.
