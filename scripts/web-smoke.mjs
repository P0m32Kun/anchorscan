import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { promises as fs } from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const artifactDir = path.resolve(process.env.E2E_ARTIFACTS_DIR || path.join(repoRoot, 'test-artifacts', 'web-smoke'));
const binary = path.resolve(process.env.ANCHORSCAN_BINARY || path.join(repoRoot, 'dist', 'anchorscan'));
const fixture = path.join(repoRoot, 'scripts', 'test-fixtures', 'tool-fixture.sh');
const consoleLogs = [];
let serverOutput = '';
let browser;
let context;
let page;
let server;

function appendOutput(chunk) {
  serverOutput += chunk.toString();
}

async function freePort() {
  const probe = net.createServer();
  probe.listen(0, '127.0.0.1');
  await once(probe, 'listening');
  const { port } = probe.address();
  probe.close();
  await once(probe, 'close');
  return port;
}

async function waitForServer(baseURL) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(baseURL);
      if (response.ok) return;
    } catch {
      // The process needs a short moment to bind its socket.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`Web server did not become ready.\n${serverOutput}`);
}

async function writeTestConfig(workDir) {
  const source = await fs.readFile(path.join(repoRoot, 'config', 'default.yaml.example'), 'utf8');
  const quotedFixture = JSON.stringify(fixture);
  const config = ['rustscan', 'nmap', 'httpx', 'nuclei'].reduce(
    (text, name) => text.replace(new RegExp(`^(\\s*${name}:).*$`, 'm'), `$1 ${quotedFixture}`),
    source,
  );
  const configPath = path.join(workDir, 'config.yaml');
  await fs.writeFile(configPath, config);
  return configPath;
}

async function saveFailureArtifacts(error) {
  await fs.mkdir(artifactDir, { recursive: true });
  await fs.writeFile(path.join(artifactDir, 'console.log'), `${consoleLogs.join('\n')}\n${serverOutput}`);
  if (page) await page.screenshot({ path: path.join(artifactDir, 'failure.png'), fullPage: true }).catch(() => {});
  if (context) await context.tracing.stop({ path: path.join(artifactDir, 'trace.zip') }).catch(() => {});
  await fs.writeFile(path.join(artifactDir, 'failure.txt'), `${error.stack || error}\n`);
}

async function stopServer() {
  if (!server || server.exitCode !== null || server.signalCode !== null) return;
  server.kill('SIGTERM');
  await once(server, 'exit');
}

try {
  await fs.access(binary);
  await fs.access(fixture);
  await fs.rm(artifactDir, { recursive: true, force: true });

  const workDir = await fs.mkdtemp(path.join(os.tmpdir(), 'anchorscan-web-smoke-'));
  const configPath = await writeTestConfig(workDir);
  const port = await freePort();
  const baseURL = `http://127.0.0.1:${port}`;

  server = spawn(binary, ['web', '--config', configPath, '--db', path.join(workDir, 'scans.sqlite'), '--listen', `127.0.0.1:${port}`], {
    cwd: repoRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  server.stdout.on('data', appendOutput);
  server.stderr.on('data', appendOutput);
  await waitForServer(baseURL);

  browser = await chromium.launch();
  context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  await context.tracing.start({ screenshots: true, snapshots: true, sources: true });
  page = await context.newPage();
  page.on('console', (message) => {
    if (message.type() === 'error') consoleLogs.push(`console.error: ${message.text()}`);
  });
  page.on('pageerror', (error) => consoleLogs.push(`pageerror: ${error.message}`));

  await page.goto(baseURL, { waitUntil: 'networkidle' });
  await page.getByRole('link', { name: '项目管理' }).click();
  await page.getByRole('link', { name: '新建项目' }).click();
  await page.getByLabel(/项目名称/).fill('Browser gate project');
  await page.locator('textarea[name="default_targets"]').fill('192.0.2.10');
  await page.locator('textarea[name="default_ports"]').fill('80,443');
  await page.getByRole('button', { name: '保存项目设置' }).click();
  await page.waitForURL(`${baseURL}/projects`);
  await assert.doesNotReject(() => page.getByRole('link', { name: 'Browser gate project' }).waitFor());

  await page.getByRole('link', { name: '扫描历史' }).click();
  await page.waitForURL(`${baseURL}/runs`);
  assert.equal(consoleLogs.length, 0, consoleLogs.join('\n'));

  await context.tracing.stop();
  await browser.close();
  browser = undefined;
  context = undefined;
  page = undefined;
  await stopServer();
  console.log('Web browser smoke test passed.');
} catch (error) {
  await saveFailureArtifacts(error);
  throw error;
} finally {
  if (browser) await browser.close().catch(() => {});
  await stopServer().catch(() => {});
}
