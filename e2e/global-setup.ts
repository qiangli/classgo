import path from 'path';
import { existsSync, readFileSync, writeFileSync } from 'fs';
import { buildServer, startServer, waitForReady } from './helpers/server.js';
import { adminLogin } from './helpers/api.js';

function loadEnvFile(filePath: string) {
  if (!existsSync(filePath)) return;
  const lines = readFileSync(filePath, 'utf-8').split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eqIdx = trimmed.indexOf('=');
    if (eqIdx === -1) continue;
    const key = trimmed.slice(0, eqIdx).trim();
    const value = trimmed.slice(eqIdx + 1).trim();
    if (!process.env[key]) {
      process.env[key] = value;
    }
  }
}

async function globalSetup() {
  // Load .env from project root, then ~/.env as fallback
  loadEnvFile(path.resolve(__dirname, '..', '.env'));
  loadEnvFile(path.join(process.env.HOME || '', '.env'));
  console.log('Building server...');
  buildServer();

  console.log('Starting test server on port 9090...');
  const state = startServer(9090, 'data/csv.example');

  console.log('Waiting for server to be ready...');
  await waitForReady(state.baseURL);
  console.log('Server ready.');

  // Attempt admin auth if credentials provided
  const user = process.env.CLASSGO_TEST_ADMIN_USER;
  const pass = process.env.CLASSGO_TEST_ADMIN_PASS;
  if (user && pass) {
    console.log('Authenticating as admin...');
    const cookie = await adminLogin(user, pass);
    if (cookie) {
      const storageState = {
        cookies: [{
          name: 'classgo_session',
          value: cookie.split('=')[1],
          domain: 'localhost',
          path: '/',
          httpOnly: true,
          secure: false,
          sameSite: 'Lax' as const,
          expires: -1,
        }],
        origins: [],
      };
      writeFileSync(
        path.join(__dirname, '.auth', 'admin-state.json'),
        JSON.stringify(storageState, null, 2)
      );
      console.log('Admin auth saved.');
    } else {
      console.warn('Admin login failed — admin tests will be skipped.');
    }
  } else {
    console.log('No CLASSGO_TEST_ADMIN_USER/PASS set — admin tests will be skipped.');
  }
}

export default globalSetup;
