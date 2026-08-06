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
const artifactDir = path.resolve(process.env.E2E_ARTIFACTS_DIR || path.join(repoRoot, 'docs', 'reports', 'ticket-04-playwright'));
const binary = path.resolve(process.env.ANCHORSCAN_BINARY || path.join(repoRoot, 'dist', 'anchorscan'));
const consoleLogs = [];
const unexpectedConsole = [];
let serverOutput = '';
let browser;
let context;
let page;
let server;
let workDir;

function entry(id, status, mode, effects, cleanup, template) {
  const safety = { mode, effects };
  if (cleanup) safety.cleanup = cleanup;
  return {
    id,
    title: `${id} title`,
    severity: '高危',
    status,
    safety,
    match: { nuclei: [id], nse: [], 'manual-review': [], cve: [] },
    sections: { 漏洞描述: `${id} description`, 修复建议: `${id} remediation` },
    verify: { tool: 'nuclei', template, target: 'host:port' },
    command: `nuclei -t ${template} -u {{host}}:{{port}}`,
    sources: [`https://example.test/${id}`],
    generated: { by: 'ticket-04-playwright', at: '2026-01-01T00:00:00Z' },
  };
}

const entries = [
  entry('safe-entry', 'stable', 'safe', [], '', 'network/safe.yaml'),
  entry('optional-entry', 'stable', 'optional', ['authentication-attempt'], '停止认证尝试', 'network/optional.yaml'),
  entry('manual-entry', 'stable', 'manual-gated', ['file-read', 'test-file-create'], '删除测试文件', 'network/manual.yaml'),
  entry('review-entry', 'needs-review', 'safe', [], '', 'network/review.yaml'),
];

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
    if (server?.exitCode !== null || server?.signalCode !== null) throw new Error(`server exited\n${serverOutput}`);
    try {
      const response = await fetch(baseURL);
      if (response.ok) return;
    } catch {
      // The process needs a moment to bind.
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`server did not become ready\n${serverOutput}`);
}

async function stopServer() {
  if (!server || server.exitCode !== null || server.signalCode !== null) return;
  server.kill('SIGTERM');
  await once(server, 'exit');
}

async function runSQLite(dbPath, sql) {
  const child = spawn('sqlite3', [dbPath, sql], { stdio: ['ignore', 'pipe', 'pipe'] });
  let stderr = '';
  child.stderr.on('data', (chunk) => { stderr += chunk.toString(); });
  const [code] = await once(child, 'close');
  assert.equal(code, 0, `sqlite fixture setup failed: ${stderr}`);
}

async function capture(name) {
  await page.screenshot({ path: path.join(artifactDir, `${name}.png`), fullPage: true });
}

function rowFor(id) {
  return page.getByRole('row').filter({ has: page.getByRole('cell', { name: id, exact: true }) });
}

async function openCommand(id) {
  await rowFor(id).getByRole('button', { name: '生成 Nuclei 命令' }).click();
  const dialog = page.getByRole('dialog', { name: '生成 Nuclei 命令' });
  await dialog.waitFor();
  return dialog;
}

try {
  await fs.access(binary);
  await fs.rm(artifactDir, { recursive: true, force: true });
  await fs.mkdir(artifactDir, { recursive: true });
  workDir = await fs.mkdtemp(path.join(os.tmpdir(), 'anchorscan-ticket-04-'));
  const configPath = path.join(workDir, 'config.yaml');
  const catalogPath = path.join(workDir, 'catalog.json');
  const dbPath = path.join(workDir, 'scan.db');
  await fs.writeFile(configPath, 'knowledge_base:\n  path: catalog.json\n');
  await fs.writeFile(catalogPath, JSON.stringify({ version: 2, source: 'handbook-v3', entry_count: entries.length, entries }, null, 2));

  const port = await freePort();
  const baseURL = `http://127.0.0.1:${port}`;
  server = spawn(binary, ['web', '--config', configPath, '--db', dbPath, '--listen', `127.0.0.1:${port}`], { cwd: repoRoot, stdio: ['ignore', 'pipe', 'pipe'] });
  server.stdout.on('data', (chunk) => { serverOutput += chunk.toString(); });
  server.stderr.on('data', (chunk) => { serverOutput += chunk.toString(); });
  await waitForServer(baseURL);

  await runSQLite(dbPath, `
    INSERT INTO scan_runs (run_id, project_id, zone_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir)
    VALUES ('ticket-04-run', '', '', '192.0.2.0/28', '443', 'normal', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{}', '');
    INSERT INTO findings (run_id, ip, port, protocol, scope, source, finding_id, severity, summary, target, output) VALUES
    ('ticket-04-run', '192.0.2.1', 443, 'tcp', '', 'nuclei', 'safe-entry', 'high', 'safe-entry title', '', ''),
    ('ticket-04-run', '192.0.2.2', 443, 'tcp', '', 'nuclei', 'optional-entry', 'high', 'optional-entry title', '', ''),
    ('ticket-04-run', '192.0.2.3', 443, 'tcp', '', 'nuclei', 'manual-entry', 'high', 'manual-entry title', '', ''),
    ('ticket-04-run', '192.0.2.4', 443, 'tcp', '', 'nuclei', 'review-entry', 'high', 'review-entry title', '', '');
  `);

  browser = await chromium.launch();
  context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  await context.tracing.start({ screenshots: true, snapshots: true, sources: true });
  page = await context.newPage();
  page.on('console', (message) => {
    if (message.type() !== 'error') return;
    const text = message.text();
    if (text.includes('status of 428')) {
      consoleLogs.push(`expected gate response: ${text}`);
      return;
    }
    consoleLogs.push(`console.error: ${text}`);
    unexpectedConsole.push(text);
  });
  page.on('pageerror', (error) => {
    consoleLogs.push(`pageerror: ${error.message}`);
    unexpectedConsole.push(error.message);
  });
  await page.goto(`${baseURL}/reports/ticket-04-run`, { waitUntil: 'networkidle' });

  let dialog = await openCommand('safe-entry');
  await dialog.getByText('nuclei -t network/safe.yaml -u 192.0.2.1:443', { exact: false }).waitFor();
  await capture('safe-command');
  await dialog.getByRole('button', { name: '关闭' }).click();

  dialog = await openCommand('optional-entry');
  await dialog.getByText('authentication-attempt', { exact: true }).waitFor();
  await dialog.getByText('停止认证尝试', { exact: false }).waitFor();
  assert.equal(await dialog.getByText('nuclei -t network/optional.yaml', { exact: false }).count(), 0, 'optional command leaked before confirmation');
  await capture('optional-before-confirm');
  await dialog.getByRole('button', { name: /确认授权范围/ }).click();
  await dialog.getByText('nuclei -t network/optional.yaml -u 192.0.2.2:443', { exact: false }).waitFor();
  await capture('optional-after-confirm');
  await dialog.getByRole('button', { name: '关闭' }).click();

  dialog = await openCommand('manual-entry');
  await dialog.getByText('file-read', { exact: true }).waitFor();
  await dialog.getByText('test-file-create', { exact: true }).waitFor();
  await dialog.getByText('删除测试文件', { exact: false }).waitFor();
  assert.equal(await dialog.getByText('nuclei -t network/manual.yaml', { exact: false }).count(), 0, 'manual command leaked before confirmation');
  await capture('manual-before-confirm');
  await dialog.getByRole('button', { name: /查看 effects 与 cleanup/ }).click();
  await dialog.getByText('nuclei -t network/manual.yaml -u 192.0.2.3:443', { exact: false }).waitFor();
  await capture('manual-after-confirm');
  await dialog.getByRole('button', { name: '关闭' }).click();

  dialog = await openCommand('review-entry');
  await dialog.getByText('needs-review', { exact: false }).first().waitFor();
  await dialog.getByText('待复核', { exact: false }).first().waitFor();
  assert.equal(await dialog.getByText('nuclei -t network/review.yaml', { exact: false }).count(), 0, 'needs-review command leaked before acknowledgement');
  await capture('needs-review-before-acknowledgement');
  await dialog.getByRole('button', { name: /复核来源/ }).click();
  await dialog.getByText('nuclei -t network/review.yaml -u 192.0.2.4:443', { exact: false }).waitFor();
  await dialog.getByText('needs-review', { exact: false }).first().waitFor();
  await capture('needs-review-after-acknowledgement');

  assert.deepEqual(unexpectedConsole, [], 'unexpected browser console errors');
  await context.tracing.stop({ path: path.join(artifactDir, 'trace.zip') });
  await fs.writeFile(path.join(artifactDir, 'console.log'), consoleLogs.join('\n') + '\n');
  await fs.writeFile(path.join(artifactDir, 'server.log'), serverOutput);
  await fs.writeFile(path.join(artifactDir, 'result.txt'), 'PASS: safe, optional, manual-gated, and needs-review command flows\n');
} catch (error) {
  await fs.mkdir(artifactDir, { recursive: true });
  if (page) await page.screenshot({ path: path.join(artifactDir, 'failure.png'), fullPage: true }).catch(() => {});
  if (context) await context.tracing.stop({ path: path.join(artifactDir, 'trace.zip') }).catch(() => {});
  await fs.writeFile(path.join(artifactDir, 'console.log'), consoleLogs.join('\n') + '\n' + serverOutput).catch(() => {});
  await fs.writeFile(path.join(artifactDir, 'failure.txt'), `${error.stack || error}\n`).catch(() => {});
  throw error;
} finally {
  await browser?.close().catch(() => {});
  await stopServer();
  if (workDir) await fs.rm(workDir, { recursive: true, force: true });
}
