# pv3

Run your dev server in an isolated Docker container. Same logs, same localhost URLs — rogue dependencies can't touch your host system.

## Quick start

```sh
curl -fsSL https://pv3.dev | sh
```

Or build from source:

```sh
git clone https://github.com/pv3dev/pv3.git
cd pv3
make install
```

## Usage

```sh
cd your-project
pv3 dev
```

pv3 reads your project files, detects the runtime and package manager, and runs your dev script inside a sandboxed Docker container.

```
pv3 dev                        # auto-detect and run
pv3 dev --port 3000            # use a different port
pv3 dev --no-net               # fully offline, no network access
pv3 dev --image node:20-slim   # override the container image
pv3 dev --verbose              # print the full docker run command
```

## Supported projects

**Node.js** — detects `package.json`, resolves `dev`/`start`/`serve` scripts, picks npm/yarn/pnpm from lockfiles.

**Python** — detects `pyproject.toml`, `requirements.txt`, `setup.py`, or `Pipfile`. Supports Django, Flask, and FastAPI with auto-configured dev server commands. Picks pip/uv/poetry/pipenv from lockfiles.

**Rust** — detects `Cargo.toml`. Recognizes Solana Anchor projects (`Anchor.toml`, `anchor-lang`) and native Solana programs (`solana-sdk`, `solana-program`). Isolates `cargo build` from your host — build scripts in untrusted crates can't escape the container.

## How it works

1. Detects your project type and dev command from manifest files
2. Checks that Docker (or Podman) is running and the port is free
3. Launches a container with your project mounted at `/workspace`
4. Forwards signals so Ctrl+C stops cleanly

The container runs with `--cap-drop=ALL`, `--security-opt no-new-privileges`, and resource limits (4 CPUs, 6 GB RAM). Your `.env` file is passed through automatically if present.

## Requirements

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/)

## Development

```sh
make build          # build for current platform
go test ./... -v    # run all tests
go vet ./...        # static analysis
make release        # cross-compile for all platforms
```

## Project structure

```
main.go                     # entry point
internal/
  dev/                      # CLI commands (cobra)
  docker/                   # container orchestration
  project/                  # project detection (node, python, rust)
testdata/                   # test fixtures by language
```

## License

MIT
