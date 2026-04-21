import { stopServer } from './helpers/server.js';

async function globalTeardown() {
  console.log('Stopping test server...');
  stopServer();
  console.log('Server stopped.');
}

export default globalTeardown;
