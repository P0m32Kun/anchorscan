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

async function captureThemeScreenshot(suffix) {
  await fs.mkdir(artifactDir, { recursive: true });
  await page.screenshot({ path: path.join(artifactDir, `theme-${suffix}.png`), fullPage: true });
}

async function waitForRunStatus(page, status, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  const statusText = page.getByText(status, { exact: true });
  while (Date.now() < deadline) {
    if (await statusText.count() > 0) return;
    await page.waitForTimeout(250);
    await page.reload({ waitUntil: 'networkidle' });
  }
  throw new Error(`run did not reach ${status} within ${timeout}ms`);
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
    if (message.type() === 'error') {
      const text = message.text();
      if (text.includes('status of 400') || text.includes('Failed to load resource')) return;
      consoleLogs.push(`console.error: ${text}`);
    }
  });
  page.on('pageerror', (error) => consoleLogs.push(`pageerror: ${error.message}`));

  await page.setViewportSize({ width: 1440, height: 960 });
  await page.goto(baseURL, { waitUntil: 'networkidle' });

  // Theme smoke: toggle should apply data-theme immediately and persist across pages.
  assert.ok(['light', 'dark'].includes(await page.evaluate(() => document.documentElement.getAttribute('data-theme'))), 'initial theme not set');
  assert.equal(await page.evaluate(() => document.documentElement.style.colorScheme), await page.evaluate(() => document.documentElement.getAttribute('data-theme')));

  await page.getByRole('button', { name: '深色' }).click();
  await page.waitForTimeout(150);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');
  await captureThemeScreenshot('dark');

  // Keyboard path: Space/Enter on a theme button updates the theme.
  const lightButton = page.getByRole('button', { name: '浅色' });
  await lightButton.focus();
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('aria-pressed')), 'false');
  await lightButton.press('Space');
  await page.waitForTimeout(150);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'light');
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('aria-pressed')), 'true');
  await captureThemeScreenshot('light');

  const darkButton = page.getByRole('button', { name: '深色' });
  await darkButton.focus();
  await darkButton.press('Enter');
  await page.waitForTimeout(150);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');

  // Explicit preference survives a page refresh.
  await page.reload({ waitUntil: 'networkidle' });
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');

  // System preference follows OS color-scheme changes at runtime.
  await page.getByRole('button', { name: '跟随系统' }).first().click();
  await page.waitForTimeout(150);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.waitForTimeout(150);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark', 'system should follow dark OS preference');
  await page.emulateMedia({ colorScheme: 'light' });
  await page.waitForTimeout(150);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'light', 'system should follow light OS preference');

  await page.getByRole('button', { name: '深色' }).click();
  await page.waitForTimeout(150);

  await page.getByRole('link', { name: '项目管理' }).click();
  await page.getByRole('link', { name: '新建项目' }).click();
  await page.getByLabel(/任务名称/).fill('Browser gate project');
  await page.getByLabel(/被测单位/).fill('Browser gate client');
  await page.getByRole('button', { name: '保存项目' }).click();
  await page.waitForURL(/\/projects\/project-/);
  const projectURL = new URL(page.url()).pathname;
  await page.getByRole('link', { name: /发起扫描|新建扫描/ }).click();
  await page.locator('[data-scan-create][data-mounted="true"]').waitFor();
  assert.equal(await page.locator('select[name="zone_id"]').inputValue(), '', 'multiple Zones must require an explicit choice');
  const options = page.locator('[data-scan-create-options]');
  assert.equal(await options.evaluate((element) => element.open), false, 'optional settings should start collapsed');
  await options.locator('summary').click();
  await page.locator('input[name="label"]').focus();
  await page.locator('input[name="label"]').press('Tab');
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'notes', 'expanded optional settings should stay keyboard reachable');
  await page.locator('input[name="label"]').fill('Browser smoke label');
  await assert.doesNotReject(() => options.getByText('已修改 1 项').waitFor());
  await options.locator('summary').click();
  await page.locator('select[name="zone_id"]').selectOption({ index: 1 });
  await page.locator('select[name="profile"]').selectOption('normal');
  await page.locator('textarea[name="target"]').fill('192.0.2.10');
  await page.locator('textarea[name="ports"]').fill('invalid');
  await page.locator('input[name="access_point"]').fill('Browser lab switch');
  await page.locator('input[name="tester_ip"]').fill('192.0.2.250');
  await page.getByRole('button', { name: '立即启动引擎扫描' }).click();
  await assert.doesNotReject(() => page.getByText('预检失败').waitFor());
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'ports', 'server validation should return focus to the invalid field');
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
  await page.locator('select[name="zone_id"]').selectOption({ index: 1 });
  await page.locator('select[name="profile"]').selectOption('normal');
  await page.locator('textarea[name="target"]').fill('192.0.2.20');
  await page.locator('textarea[name="ports"]').fill('80');
  await page.locator('input[name="access_point"]').fill('Browser lab switch');
  await page.locator('input[name="tester_ip"]').fill('192.0.2.250');
  await page.getByRole('button', { name: '立即启动引擎扫描' }).click();
  await page.waitForURL(/\/runs\/run-/);
  await waitForRunStatus(page, 'completed');
  await page.getByRole('link', { name: '查看扫描报告' }).click();
  await assert.doesNotReject(() => page.getByText('检测执行覆盖').waitFor());
  await assert.doesNotReject(() => page.getByRole('cell', { name: 'anchorscan-test' }).waitFor());

  const projectID = projectURL.split('/').pop();
  await seedRun(`INSERT INTO scan_runs (run_id, project_id, zone_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir) VALUES
    ('browser-errors', '${projectID}', 'I', '192.0.2.30', '443', 'normal', 'completed_with_errors', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{"zone_id":"I","target":"192.0.2.30","ports":"443","profile":"normal"}', ''),
    ('browser-interrupted', '${projectID}', 'I', '192.0.2.31', '80,443', 'normal', 'interrupted', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{"zone_id":"I","target":"192.0.2.31","ports":"80,443","profile":"fast"}', '');
    INSERT INTO detection_checks (run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at) VALUES
    ('browser-errors', '192.0.2.30', 443, 'tcp', 'nuclei', 'failed', 'command_failed', 'fixture failure', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z');`);
  await page.goto(`${baseURL}/runs/browser-errors`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('completed_with_errors').waitFor());
  await assert.doesNotReject(() => page.getByText(/检测检查：.*失败 1/).waitFor({ timeout: 5_000 }));
  await page.goto(`${baseURL}/runs/browser-interrupted`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('interrupted', { exact: true }).waitFor());
  await page.getByRole('link', { name: '确认并重新运行' }).click();
  await page.waitForURL(new RegExp(`/projects/${projectID}/scans/new\\?rerun=browser-interrupted`));
  assert.equal(await page.locator('textarea[name="target"]').inputValue(), '192.0.2.31');
  assert.equal(await page.locator('textarea[name="ports"]').inputValue(), '80,443');
  assert.equal(await page.locator('select[name="profile"]').inputValue(), 'fast');
  await page.locator('textarea[name="target"]').focus();
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'target');
  await page.keyboard.press('Tab');
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('name')), 'ports');
  await page.keyboard.press('Tab');
  // The "insert high-risk ports" shortcut is a focusable button between ports and access_point.
  let activeName = await page.evaluate(() => document.activeElement?.getAttribute('name'));
  if (activeName !== 'access_point') {
    await page.keyboard.press('Tab');
    activeName = await page.evaluate(() => document.activeElement?.getAttribute('name'));
  }
  assert.equal(activeName, 'access_point');

  await page.getByRole('link', { name: '扫描历史' }).click();
  await page.waitForURL(`${baseURL}/runs`);

  await seedRun(`DELETE FROM project_zones WHERE project_id = '${projectID}';
    INSERT INTO project_zones (project_id, zone_id, name, sort_order) VALUES ('${projectID}', 'dmz', 'DMZ', 1);`);
  await page.goto(`${baseURL}${projectURL}/scans/new`, { waitUntil: 'networkidle' });
  await page.locator('[data-scan-create][data-mounted="true"]').waitFor();
  assert.equal(await page.locator('select[name="zone_id"]').inputValue(), 'dmz', 'a single Zone should be selected automatically');

  await page.getByRole('link', { name: '导入 Nmap XML' }).click();
  await page.locator('input[name="xml_file"]').setInputFiles(xmlPath);
  await page.getByRole('button', { name: /导入/ }).click();
  await page.waitForURL(/\/runs\/run-/);
  const runID = page.url().split('/').pop();
  await page.goto(`${baseURL}/reports/${runID}`);
  await page.getByRole('button', { name: '端口与服务' }).click();
  await page.getByLabel(/端口/).fill('80');
  await page.getByRole('button', { name: '应用', exact: true }).click();
  await assert.doesNotReject(() => page.getByText('192.0.2.10').first().waitFor());
  await page.getByRole('link', { name: '下一页' }).first().click();
  await page.getByRole('button', { name: '复制 IP' }).first().click();

  await page.setViewportSize({ width: 1280, height: 960 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false);

  // Workbench regression: seed a completed run with a non-info finding and verify
  // the candidate renders, the verify dialog opens, and the first focusable control
  // inside the dialog is keyboard-reachable.
  await seedRun(`INSERT INTO scan_runs (run_id, project_id, zone_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir, include_in_report) VALUES
    ('browser-workbench', '${projectID}', 'dmz', '192.0.2.50', '6379', 'normal', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{}', '', 1);
    INSERT INTO fingerprints (run_id, ip, port, service, product, version, normalized, is_web, url, protocol, cpe, extrainfo, tunnel) VALUES
    ('browser-workbench', '192.0.2.50', 6379, 'redis', '', '', 'redis', 0, '', 'tcp', '', '', '');
    INSERT INTO findings (run_id, ip, port, source, finding_id, severity, summary, target, output, protocol, scope) VALUES
    ('browser-workbench', '192.0.2.50', 6379, 'nuclei', 'redis-default-login', 'high', 'Workbench smoke finding', '192.0.2.50:6379', '', 'tcp', '');`);
  await page.goto(`${baseURL}${projectURL}/workbench`, { waitUntil: 'networkidle' });
  await page.locator('[data-workbench][data-mounted="true"]').waitFor();
  await assert.doesNotReject(() => page.getByText('Workbench smoke finding').waitFor());
  await page.getByRole('button', { name: '验证 / 编辑' }).first().click();
  const dialog = page.locator('dialog.verify-dialog');
  await assert.doesNotReject(() => dialog.waitFor());
  await page.waitForFunction(() => {
    const active = document.activeElement;
    return active?.tagName === 'INPUT' || active?.tagName === 'SELECT' || active?.tagName === 'TEXTAREA';
  });
  const focusedName = await page.evaluate(() => document.activeElement?.getAttribute('name') || document.activeElement?.tagName || '');
  assert.ok(focusedName === 'title' || focusedName === 'INPUT', `verify dialog first focusable should be reachable, got ${focusedName}`);
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => dialog.waitFor({ state: 'hidden' }));

  await page.goto(`${baseURL}/config`);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark', 'theme preference should persist on config page');
  await page.getByRole('button', { name: '跟随系统' }).first().click();
  await page.waitForTimeout(150);
  assert.ok(['light', 'dark'].includes(await page.evaluate(() => document.documentElement.getAttribute('data-theme'))), 'system theme did not resolve');

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
