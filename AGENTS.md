# Repository Guidelines

## Project Structure & Module Organization
- `cmd/server/main.go` is the application entrypoint (Echo HTTP server).
- `internal/` holds core logic: `handlers/`, `services/`, `adapters/`, `converters/`, `middleware/`, `models/`, `config/`, and `database/`.
- `templates/` contains HTML pages for auth and dashboard; `static/` holds CSS/JS assets.
- `migrations/` stores SQL schema changes; `data/` contains the SQLite database file.
- `docs/` includes architecture notes, API references, and design decisions.

## Build, Test, and Development Commands
- `go run ./cmd/server` runs the gateway locally (loads `.env` if present).
- `go build ./cmd/server` builds a server binary in the current directory.
- `go test ./...` runs all unit tests.
- `gofmt -w cmd internal` formats Go source files.
- `go vet ./...` runs static analysis for common issues.
- Legacy Python/uvicorn steps in `docs/getting-started.md` are outdated; use the Go commands above.

## Coding Style & Naming Conventions
- Go 1.21, gofmt formatting (tabs for indentation).
- Exported identifiers use `CamelCase`; unexported identifiers use `lowerCamelCase`.
- Package names are short, lowercase, and match their folder name.
- Keep JSON tags and request/response field names consistent with existing models.

## Testing Guidelines
- Tests use the standard Go `testing` package and live next to code as `*_test.go`.
- Name tests `TestXxx` and focus on converter behavior and handler request/response mapping.
- Run `go test ./...` before opening a PR.

## Commit & Pull Request Guidelines
- Follow Conventional Commits observed in this repo (e.g., `feat:`, `chore:`); keep messages short and present tense.
- PRs should include: a clear summary, testing steps, linked issues, and screenshots for UI changes under `templates/` or `static/`.
- Call out configuration or migration changes explicitly in the PR description.

## Security & Configuration Notes
- Use `.env.example` as the template; never commit real API keys or secrets.
- Provider base URLs and secrets are configured via environment variables; defaults are in `internal/config`.
- If you add a migration, verify it against a fresh `data/ai_gateway.db` and document the impact.
