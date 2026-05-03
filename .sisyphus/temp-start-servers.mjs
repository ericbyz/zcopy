
import { spawn } from 'child_process';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const projectRoot = join(__dirname, '..', '..');

const servers = [];

function startServer(name, cwd, args) {
  console.log(`Starting ${name}...`);
  const server = spawn('npm', args, {
    cwd: join(projectRoot, cwd),
    detached: true,
    stdio: 'inherit'
  });
  
  server.on('error', (err) => {
    console.error(`Failed to start ${name}:`, err);
  });
  
  server.on('close', (code) => {
    console.log(`${name} exited with code ${code}`);
  });
  
  servers.push(server);
  return server;
}

// Start server frontend on port 5174
const serverFront = spawn('npx', ['vite', '--port', '5174'], {
  cwd: join(projectRoot, 'server/front'),
  detached: false,
  stdio: 'pipe'
});

// Start client frontend on port 5173
const clientFront = spawn('npx', ['vite', '--port', '5173'], {
  cwd: join(projectRoot, 'client/front'),
  detached: false,
  stdio: 'pipe'
});

let serverReady = false;
let clientReady = false;

serverFront.stdout.on('data', (data) => {
  const output = data.toString();
  console.log('[Server Front]', output);
  if (output.includes('ready in')) {
    serverReady = true;
    checkAllReady();
  }
});

clientFront.stdout.on('data', (data) => {
  const output = data.toString();
  console.log('[Client Front]', output);
  if (output.includes('ready in')) {
    clientReady = true;
    checkAllReady();
  }
});

function checkAllReady() {
  if (serverReady && clientReady) {
    console.log('\n✅ Both servers are ready!');
    console.log('   - Server Front: http://localhost:5174');
    console.log('   - Client Front: http://localhost:5173');
  }
}

// Handle exit
process.on('SIGINT', () => {
  console.log('\nShutting down servers...');
  serverFront.kill();
  clientFront.kill();
  process.exit();
});
