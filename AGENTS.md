# Repository Guidelines

## Project Structure & Module Organization
- `cmd/server`: HTTP entrypoint wiring config, middleware, handlers, and processors.
- `cmd/cli`: Cobra utility (version/server/config/test verbs).
- `internal/api`: Gin handlers, middleware, and route registration.
- `internal/core`: Domain logic for audio, text, prompt, LLM backends, and processing services.
- `pkg`: Shared utilities such as auth strategies and metrics collectors.
- `config`: Runtime configuration (`config.yaml`, `api_keys.json`) plus templates; prefer config over code changes.
- `docs` and `README.md`: Reference docs; `test/` holds sample audio fixtures.

## Build, Test, and Development Commands
- `./start_local.sh`: Local dev (Go 1.25.4) reading `config/config.yaml` and `api_keys.json`.
- `go build ./cmd/server ./cmd/cli`: Build the HTTP server and CLI.
- `go test ./...`: Run unit tests (add `_test.go` coverage as you contribute).
- `./test_api.sh`: Smoke-test endpoints against a running instance using sample OPUS audio.
- Production: `docker-compose up -d lingualink-core` (reads `docker-compose.yml` and mounts `./config`/`./logs`).

## Coding Style & Naming Conventions
- Go 1.25.4+. Format all changes with `gofmt` before committing; keep imports goimports-friendly ordering.
- Package, file, and directory names stay lowercase with underscores only when needed (Go style).
- Keep handlers thin; place business logic in `internal/core/*` and reusable helpers in `pkg/*`.
- Log with `logrus` and prefer structured fields over string concatenation.
- Keep config-driven behaviors in `config` structs instead of hard-coded constants.

## Testing Guidelines
- Prefer table-driven tests in `_test.go` files; co-locate with the package under test.
- Stub external calls (LLM/backend) and exercise processors and handlers with representative payloads.
- Seed sample audio from `test/` when validating audio flows; avoid embedding new large fixtures in git.
- Aim for coverage on new logic paths; ensure `go test ./...` passes before opening PRs.

## Auth & Access
- Default API key is `lingualink-demo-key` in `config/api_keys.json`; it is unlimited and should be sent via `X-API-Key`.
- For production, swap in your own key file and keep the anonymous strategy disabled unless intentionally exposing open endpoints.

## Commit & Pull Request Guidelines
- Follow concise, action-led commit subjects (e.g., `Add prompt engine config validation`); recent history favors clear verbs over ticket prefixes.
- Each PR should include: purpose summary, key changes, and any config/env toggles. Link related issues where applicable.
- Include screenshots or example responses for API-affecting changes. Note breaking changes and migration steps explicitly.
- Before requesting review, rerun `gofmt`, `go test ./...`, and, if applicable, `./test_api.sh`; ensure configs do not leak real keys or secrets.
