# Deploy

Deploy, start, stop, or check the status of the ClassGo server.

## Arguments

- First arg (optional): action — `start` (default), `stop`, `restart`, `status`
- Second arg (optional): remote host — `user@hostname` or `user@hostname:port` (SSH)
- If no args, default to `start` locally

## Local Deployment

### Start
1. Build first: `make build` (or `make start` which builds and starts)
2. Check if already running: `if [ -f bin/.pid ] && kill -0 $(cat bin/.pid) 2>/dev/null`
3. If running, report PID and skip
4. Start: `bin/classgo & echo $! > bin/.pid`
5. Wait 2 seconds, verify it's running
6. Report the URLs:
   - Mobile: `http://localhost:8080/`
   - Admin: `http://localhost:8080/admin`
   - Kiosk: `http://localhost:8080/kiosk`
   - Memos: `http://localhost:8080/memos/`

### Stop
1. Run `make stop`, or manually: check PID file `bin/.pid`
2. If running: `kill $(cat bin/.pid) && rm -f bin/.pid`
3. If not running, report that

### Restart
1. Stop (if running)
2. Build
3. Start

### Status
1. Check if PID file exists and process is alive
2. Check if port 8080 is responding: `curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/settings`
3. Report running/stopped status with PID

## Remote Deployment

When a remote host is provided (e.g., `deploy start user@myserver`):

1. Build locally first for the target platform
2. Determine target OS/arch — ask user if not obvious, or default to `linux/amd64`
3. Build for target: `GOOS=linux GOARCH=amd64 go build -o bin/classgo-linux-amd64 .`
4. Copy binary and required files via SCP:
   ```bash
   scp bin/classgo-linux-amd64 user@host:~/classgo/classgo
   scp -r templates/ user@host:~/classgo/templates/
   scp -r static/ user@host:~/classgo/static/
   scp config.json user@host:~/classgo/config.json  # if exists
   ```
5. For start: `ssh user@host 'cd ~/classgo && ./classgo &'`
6. For stop: `ssh user@host 'pkill -f classgo || true'`
7. For status: `ssh user@host 'pgrep -f classgo && curl -s localhost:8080/api/settings || echo "not running"'`

Note: The binary is self-contained (includes Memos). Templates and static files are loaded from disk and must be copied alongside the binary.
