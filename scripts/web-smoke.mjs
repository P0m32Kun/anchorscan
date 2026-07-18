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
let workDir;

const importFixture = `<nmaprun>${Array.from({ length: 51 }, (_, index) => `<host><address addr="192.0.2.${index + 1}"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx" version="1.24"/></port></ports></host>`).join('')}</nmaprun>`;

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
    if (server?.exitCode !== null || server?.signalCode !== null) {
      throw new Error(`Web server exited before becoming ready.\n${serverOutput}`);
    }
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

async function startServer(configPath) {
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const port = await freePort();
    const baseURL = `http://127.0.0.1:${port}`;
    serverOutput = '';
    server = spawn(binary, ['web', '--config', configPath, '--db', path.join(workDir, 'scans.sqlite'), '--listen', `127.0.0.1:${port}`], {
      cwd: repoRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    server.stdout.on('data', appendOutput);
    server.stderr.on('data', appendOutput);
    try {
      await waitForServer(baseURL);
      return baseURL;
    } catch (error) {
      await stopServer();
      if (attempt === 3) throw error;
    }
  }
  throw new Error('Web server could not acquire a test port.');
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
  await Promise.all(['nse.yaml', 'service-tags.yaml'].map(async (name) => {
    await fs.copyFile(path.join(repoRoot, 'config', name), path.join(workDir, name));
  }));
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

async function seedRun(sql) {
  const child = spawn('sqlite3', [path.join(workDir, 'scans.sqlite'), sql]);
  const [code] = await once(child, 'close');
  assert.equal(code, 0, 'sqlite fixture setup failed');
}

try {
  await fs.access(binary);
  await fs.access(fixture);
  await fs.rm(artifactDir, { recursive: true, force: true });

  workDir = await fs.mkdtemp(path.join(os.tmpdir(), 'anchorscan-web-smoke-'));
  const configPath = await writeTestConfig(workDir);
  const xmlPath = path.join(workDir, 'import.xml');
  await fs.writeFile(xmlPath, importFixture);
  const baseURL = await startServer(configPath);

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
  const projectLink = page.getByRole('link', { name: 'Browser gate project' });
  await assert.doesNotReject(() => projectLink.waitFor());
  const projectURL = await projectLink.getAttribute('href');
  await projectLink.click();
  await page.getByRole('link', { name: /发起扫描/ }).click();
  await page.locator('textarea[name="ports"]').fill('invalid');
  await page.getByRole('button', { name: '立即启动引擎扫描' }).click();
  await assert.doesNotReject(() => page.getByText('预检失败').waitFor());
  await page.setViewportSize({ width: 1280, height: 960 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false);
  await page.setViewportSize({ width: 1440, height: 960 });
  await page.locator('textarea[name="target"]').fill('198.51.100.99');
  await page.locator('textarea[name="ports"]').fill('80');
  await page.getByRole('button', { name: '立即启动引擎扫描' }).click();
  await page.waitForURL(/\/runs\/run-/);
  const cancelButton = page.getByRole('button', { name: '中止扫描' });
  const readyDeadline = Date.now() + 5_000;
  while (Date.now() < readyDeadline && await cancelButton.count() === 0) {
    await page.waitForTimeout(100);
    await page.reload({ waitUntil: 'networkidle' });
  }
  assert.equal(await cancelButton.count(), 1, 'running scan detail did not become ready');
  const cancelURL = await cancelButton.evaluate((button) => button.form.action);
  await page.evaluate(async (url) => {
    const response = await fetch(url, { method: 'POST', redirect: 'manual' });
    if (!response.ok && response.type !== 'opaqueredirect') throw new Error(`cancel failed: ${response.status}`);
  }, cancelURL);
  await assert.doesNotReject(() => page.getByText('canceled').waitFor({ timeout: 5_000 }));

  await page.goto(`${baseURL}${projectURL}/scans/new`, { waitUntil: 'networkidle' });
  await page.locator('textarea[name="target"]').fill('192.0.2.20');
  await page.locator('textarea[name="ports"]').fill('80');
  await page.getByRole('button', { name: '立即启动引擎扫描' }).click();
  await page.waitForURL(/\/runs\/run-/);
  await assert.doesNotReject(() => page.getByText('completed').waitFor({ timeout: 10_000 }));
  await page.getByRole('link', { name: '查看扫描报告' }).click();
  await assert.doesNotReject(() => page.getByText('检测执行覆盖').waitFor());
  await assert.doesNotReject(() => page.getByText('anchorscan-test').waitFor());

  const projectID = projectURL.split('/').pop();
  await seedRun(`INSERT INTO scan_runs (run_id, project_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir) VALUES
    ('browser-errors', '${projectID}', '192.0.2.30', '443', 'normal', 'completed_with_errors', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{"target":"192.0.2.30","ports":"443","profile":"normal"}', ''),
    ('browser-interrupted', '${projectID}', '192.0.2.31', '80,443', 'normal', 'interrupted', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{"target":"192.0.2.31","ports":"80,443","profile":"fast"}', '');
    INSERT INTO detection_checks (run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at) VALUES
    ('browser-errors', '192.0.2.30', 443, 'tcp', 'nuclei', 'failed', 'command_failed', 'fixture failure', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z');`);
  await page.goto(`${baseURL}/runs/browser-errors`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('completed_with_errors').waitFor());
  await assert.doesNotReject(() => page.getByText(/检测检查：.*失败 1/).waitFor({ timeout: 5_000 }));
  await page.goto(`${baseURL}/runs/browser-interrupted`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('interrupted').waitFor());
  await page.getByRole('link', { name: '确认并重新运行' }).click();
  await page.waitForURL(new RegExp(`/projects/${projectID}/scans/new\\?rerun=browser-interrupted`));
  assert.equal(await page.locator('textarea[name="target"]').inputValue(), '192.0.2.31');
  assert.equal(await page.locator('textarea[name="ports"]').inputValue(), '80,443');
  assert.equal(await page.locator('select[name="profile"]').inputValue(), 'fast');
  await page.locator('textarea[name="target"]').focus();
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'target');
  await page.locator('textarea[name="target"]').press('Tab');
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'ports');

  await page.getByRole('link', { name: '扫描历史' }).click();
  await page.waitForURL(`${baseURL}/runs`);

  await page.getByRole('link', { name: '导入 Nmap XML' }).click();
  await page.locator('input[name="xml_file"]').setInputFiles(xmlPath);
  await page.getByRole('button', { name: /导入/ }).click();
  await page.waitForURL(/\/runs\/run-/);
  const runID = page.url().split('/').pop();
  await page.goto(`${baseURL}/reports/${runID}`);
  await page.getByLabel(/端口/).fill('80');
  await page.getByRole('button', { name: /筛选|应用/ }).click();
  await assert.doesNotReject(() => page.getByText('192.0.2.53').first().waitFor());
  await page.getByRole('link', { name: '下一页' }).first().click();
  await page.getByRole('button', { name: '复制字段' }).first().click();

  await page.setViewportSize({ width: 1280, height: 960 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false);
  await page.setViewportSize({ width: 1440, height: 960 });
  await page.goto(`${baseURL}/config`);
  assert.equal(await page.locator('input[name="timeout_rustscan"]').inputValue(), '0');
  await page.locator('input[name="timeout_rustscan"]').fill('30s');
  await page.locator('input[name="timeout_rustscan"]').focus();
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'timeout_rustscan');
  await page.locator('input[name="timeout_nmap"]').fill('0');
  await page.locator('textarea[name="raw_config"]').fill(': invalid');
  await page.getByRole('button', { name: '应用高级 YAML 配置' }).click();
  await assert.doesNotReject(() => page.getByText(/配置应用失败/).waitFor());
  await page.goto(`${baseURL}/config`);
  assert.equal(await page.locator('input[name="timeout_rustscan"]').inputValue(), '0');
  await page.locator('input[name="timeout_rustscan"]').fill('30s');
  await page.getByLabel('全局默认端口').fill('80,443');
  await page.getByRole('button', { name: /保存/ }).first().click();
  await page.waitForURL(/\/config\?saved=1/);
  await page.goto(`${baseURL}/config`);
  assert.equal(await page.locator('input[name="timeout_rustscan"]').inputValue(), '30s');
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
  if (workDir) await fs.rm(workDir, { recursive: true, force: true }).catch(() => {});
}
