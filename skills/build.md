# Build

Build the ClassGo/TutorOS binary locally.

## Full Build (frontend + Go)

Run `make build` which:
- Formats and vets Go code (`go fmt`, `go vet`, `go mod tidy`)
- Builds Tailwind CSS (`./tailwindcss -i static/css/input.css -o static/css/tailwind.css`)
- Builds the Memos React frontend (`pnpm install && pnpm run release` in `memos/web/`)
- Compiles the Go binary to `bin/classgo`

## Go-Only Build (skip frontend)

If frontend assets are already built or the user says "quick" or "go only":

```bash
go fmt ./... && go vet ./... && go mod tidy && go build -o bin/classgo .
```

This works if `memos/server/router/frontend/dist/` already has built assets.

## Cross-Compile

If all platforms are requested, run `make build-all` to produce binaries for darwin/linux (amd64+arm64) and windows (amd64).

## Verify

After building, report the binary path and size (`ls -lh bin/classgo`).
