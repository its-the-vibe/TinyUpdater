# TinyUpdater

[![CI](https://github.com/its-the-vibe/TinyUpdater/actions/workflows/ci.yaml/badge.svg)](https://github.com/its-the-vibe/TinyUpdater/actions/workflows/ci.yaml)

TinyUpdater is an auto-updater framework for MicroPython programs running on
Raspberry Pi Pico W. This repository provides an HTTP service and a CLI for
generating file manifests.

## Requirements

- Go 1.24 or newer
- Docker with the Compose plugin (optional, for containerized use)
- Make (optional, for the provided build shortcuts)

## Build and test

```sh
go build -o bin/tinyupdater ./cmd/tinyupdater
go test ./...
```

Or use `make build` and `make test`.

## Run the service locally

Copy the example configuration and start the service:

```sh
cp config.example.yaml config.yaml
make run
```

The HTTP service listens on `0.0.0.0:8080` by default. Check
`http://localhost:8080/healthz` for its health response. You can also run
`go run ./cmd/tinyupdater server --config config.yaml`.
Repositories listed in `allowedRepositories` are served from
`repositoryPath` through `GET /files?repo=<owner/repo>&file=<filename>`.
Requests can retrieve `manifest.json` and files listed by that manifest.

To run it in Docker, use `docker compose up --build`. The container uses a
distroless runtime and a read-only filesystem.

## Run the CLI

```sh
go run ./cmd/tinyupdater manifest --repo its-the-vibe/PicoPaper
```

The `manifest` command clones the repository to
`repositories/<owner>/<repo>` (or updates its existing `main` branch), reads
`tinyupdater.json`, and writes a `manifest.json` containing the repository in
`owner/repo` format, current short Git SHA, and CRC32 checksums for the
configured files. Build the standalone CLI with `make build`. Then use either
`./bin/tinyupdater server --config config.yaml` or
`./bin/tinyupdater manifest --repo owner/repo`.

## Configuration

The service reads `server.address` from a YAML config file. Start from
`config.example.yaml`; keep your local `config.yaml` and `.env` files out of
version control. `.env.example` documents a placeholder for secrets that may
be used by future integrations; the skeleton service does not currently read
an API key.

## Development guidelines

- Keep changes focused and add tests for new behavior.
- Run `make test` before submitting changes.
- Do not commit generated binaries, local configuration, or secrets.
