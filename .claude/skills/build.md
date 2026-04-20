---
name: build
description: Build the ClassGo/TutorOS binary locally
user_invocable: true
---

# Build ClassGo

Build the project locally. This builds the Memos frontend (if needed) and compiles the Go binary.

## Steps

1. Run `make build` which:
   - Formats and vets Go code (`go fmt`, `go vet`, `go mod tidy`)
   - Builds the Memos React frontend (`pnpm install && pnpm run release` in `memos/web/`)
   - Compiles the Go binary to `bin/classgo`

2. If the frontend build fails (missing pnpm/node), try building Go only:
   ```
   go build -o bin/classgo .
   ```
   This works if `memos/server/router/frontend/dist/` already has built assets.

3. Report the binary size and path on success.

4. If `--all` or cross-compile is requested, run `make build-all` instead to produce binaries for all platforms (darwin/linux amd64+arm64, windows amd64).

## Quick Build (Go only, skip frontend)

If the user says "quick" or "go only", skip the frontend build:
```bash
go fmt ./... && go vet ./... && go mod tidy && go build -o bin/classgo .
```
