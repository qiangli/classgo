import { execSync, spawn, ChildProcess } from 'child_process';
import { writeFileSync, readFileSync, unlinkSync, rmSync, existsSync } from 'fs';
import path from 'path';

const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const STATE_FILE = path.join(__dirname, '..', '.server-state.json');

interface ServerState {
  pid: number;
  dbPath: string;
  baseURL: string;
}

export function buildServer(): void {
  execSync('go build -o bin/classgo .', { cwd: PROJECT_ROOT, stdio: 'inherit' });
}

export function startServer(port: number, dataDir: string): ServerState {
  const tmpDir = execSync('mktemp -d /tmp/classgo-e2e-XXXXXX').toString().trim();
  const dbPath = path.join(tmpDir, 'classgo.db');

  const proc: ChildProcess = spawn(
    path.join(PROJECT_ROOT, 'bin', 'classgo'),
    ['-port', String(port), '-data-dir', dataDir, '-db', dbPath],
    { cwd: PROJECT_ROOT, detached: true, stdio: 'ignore' }
  );
  proc.unref();

  const state: ServerState = {
    pid: proc.pid!,
    dbPath,
    baseURL: `http://localhost:${port}`,
  };
  writeFileSync(STATE_FILE, JSON.stringify(state));
  return state;
}

export async function waitForReady(baseURL: string, timeoutMs = 10_000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${baseURL}/api/settings`);
      if (res.ok) return;
    } catch {}
    await new Promise(r => setTimeout(r, 500));
  }
  throw new Error(`Server not ready after ${timeoutMs}ms`);
}

export function stopServer(): void {
  if (!existsSync(STATE_FILE)) return;
  const state: ServerState = JSON.parse(readFileSync(STATE_FILE, 'utf-8'));
  try {
    process.kill(state.pid, 'SIGTERM');
  } catch {}
  try {
    rmSync(path.dirname(state.dbPath), { recursive: true, force: true });
  } catch {}
  try {
    unlinkSync(STATE_FILE);
  } catch {}
}

export function getState(): ServerState {
  return JSON.parse(readFileSync(STATE_FILE, 'utf-8'));
}
