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
const expectedVersion = process.env.ANCHORSCAN_EXPECTED_VERSION;
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
  let config = ['rustscan', 'nmap', 'httpx', 'nuclei'].reduce(
    (text, name) => text.replace(new RegExp(`^(\\s*${name}:).*$`, 'm'), `$1 ${quotedFixture}`),
    source,
  );
  config = config.replace(/^(\s*knowledge_base:\s*\n\s*path:).*/m, '$1 "catalog.md"');
  const catalog = `<!-- anchorscan-catalog version: 1 -->\n\n### SMB 签名未启用（严重）\n\n<!-- anchorscan-entry\nid: smb-signing\naliases: []\nmatch:\n  nuclei: [smb-signing]\n  nse: []\n  manual-review: []\n  cve: []\n-->\n\n#### 漏洞描述\n\nSMB 签名未启用描述。\n\n#### 验证命令\n\n##### Nuclei\n\n\`\`\`\nnuclei -t smb-signing -u {{host}}:{{port}}\n\`\`\`\n\n##### Nmap NSE\n\n\`\`\`\nnmap -p {{port}} --script smb-security-mode {{host}}\n\`\`\`\n\n#### 修复建议\n\n启用 SMB 签名。\n`;
  const configPath = path.join(workDir, 'config.yaml');
  await fs.writeFile(configPath, config);
  await fs.writeFile(path.join(workDir, 'catalog.md'), catalog);
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
  if (expectedVersion) {
    assert.match(expectedVersion, /^\S+$/, 'ANCHORSCAN_EXPECTED_VERSION must be a non-empty display version');
    assert.equal(expectedVersion.startsWith('v'), false, 'ANCHORSCAN_EXPECTED_VERSION must not contain a tag prefix');
    await assert.doesNotReject(() => page.getByText(`AnchorScan Console ${expectedVersion}`, { exact: true }).waitFor());
  }

  // Theme smoke: toggle should apply data-theme immediately and persist across pages.
  assert.ok(['light', 'dark'].includes(await page.evaluate(() => document.documentElement.getAttribute('data-theme'))), 'initial theme not set');
  assert.equal(await page.evaluate(() => document.documentElement.style.colorScheme), await page.evaluate(() => document.documentElement.getAttribute('data-theme')));

  const initialTheme = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
  const toggleBtn = page.locator('.theme-toggle-btn-single').first();

  if (initialTheme === 'light') {
    await toggleBtn.click();
    await page.waitForTimeout(150);
    assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');
    await captureThemeScreenshot('dark');

    // Keyboard path: Space/Enter on the toggle button
    await toggleBtn.focus();
    await toggleBtn.press('Space');
    await page.waitForTimeout(150);
    assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'light');
    await captureThemeScreenshot('light');

    await toggleBtn.focus();
    await toggleBtn.press('Enter');
    await page.waitForTimeout(150);
    assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');
  } else {
    await toggleBtn.click();
    await page.waitForTimeout(150);
    assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'light');
    await captureThemeScreenshot('light');

    // Keyboard path: Space/Enter on the toggle button
    await toggleBtn.focus();
    await toggleBtn.press('Space');
    await page.waitForTimeout(150);
    assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');
    await captureThemeScreenshot('dark');

    await toggleBtn.focus();
    await toggleBtn.press('Enter');
    await page.waitForTimeout(150);
    assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'light');

    // Switch to dark for subsequent tests
    await toggleBtn.click();
    await page.waitForTimeout(150);
  }

  // Explicit preference survives a page refresh.
  await page.reload({ waitUntil: 'networkidle' });
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');

  await page.getByRole('link', { name: '项目管理' }).click();
  await page.getByRole('link', { name: '新建项目' }).click();
  await page.getByLabel(/任务名称/).fill('Browser gate project');
  await page.getByLabel(/被测单位/).fill('Browser gate client');
  await page.getByRole('button', { name: '保存项目' }).click();
  await page.waitForURL(/\/projects\/project-/);
  const projectURL = new URL(page.url()).pathname;

  // Destructive actions use the shared, keyboard-accessible confirmation dialog.
  await page.locator('input[name="name"]').fill('Smoke deletion zone');
  await page.getByRole('button', { name: '添加' }).click();
  const smokeZone = page.locator('.project-zone-item', { hasText: 'Smoke deletion zone' });
  await assert.doesNotReject(() => smokeZone.waitFor());
  const deleteZoneButton = smokeZone.getByRole('button', { name: '删除' });
  let browserConfirmOpened = false;
  page.once('dialog', async (dialog) => {
    browserConfirmOpened = true;
    await dialog.dismiss();
  });
  await deleteZoneButton.click();
  const confirmDialog = page.getByRole('dialog', { name: '删除分区' });
  await assert.doesNotReject(() => confirmDialog.waitFor({ timeout: 2_000 }));
  assert.equal(browserConfirmOpened, false, 'destructive actions must not use browser confirm');
  await page.screenshot({ path: path.join(artifactDir, 'confirmation-dark.png'), fullPage: true });
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => confirmDialog.waitFor({ state: 'hidden' }));
  assert.equal(await deleteZoneButton.evaluate((button) => document.activeElement === button), true, 'closing confirmation should restore focus');
  await deleteZoneButton.click();
  await confirmDialog.getByRole('button', { name: '删除' }).click();
  await assert.doesNotReject(() => smokeZone.waitFor({ state: 'hidden' }));

  // Zone Tabs filter the run tables; "all" is the default and restores every zone.
  const zoneTabs = page.locator('[data-zone-tabs]');
  await assert.doesNotReject(() => zoneTabs.waitFor());
  const zoneGroups = page.locator('.project-zone-runs[data-zone]');
  assert.equal(await zoneTabs.locator('[data-zone-target="all"]').getAttribute('aria-pressed'), 'true', 'all zones should be visible by default');
  const singleZoneTab = zoneTabs.locator('[data-zone-target]').nth(1);
  await singleZoneTab.click();
  assert.equal(await singleZoneTab.getAttribute('aria-pressed'), 'true');
  assert.equal(await zoneGroups.evaluateAll((groups) => groups.filter((group) => !group.hidden).length), 1, 'selecting a zone should show exactly one zone table');
  await zoneTabs.locator('[data-zone-target="all"]').click();
  assert.equal(await zoneGroups.evaluateAll((groups) => groups.filter((group) => !group.hidden).length), await zoneGroups.count(), 'all zones should reappear');

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
  await page.locator('[data-run-detail][data-mounted="true"]').waitFor();
  assert.equal(await page.locator('.run-monitor-panel').evaluate((element) => getComputedStyle(element).backgroundImage.includes('255')), false, 'dark run panel must not use a white surface');
  await page.screenshot({ path: path.join(artifactDir, 'run-dark.png'), fullPage: true });
  const cancelButton = page.getByRole('button', { name: '中止扫描' });
  await assert.doesNotReject(() => cancelButton.waitFor({ timeout: 5_000 }));
  const runURL = page.url();
  await page.getByRole('link', { name: '项目管理' }).click();
  await page.goBack({ waitUntil: 'networkidle' });
  await page.locator('[data-run-detail][data-mounted="true"]').waitFor();
  await assert.doesNotReject(() => page.getByRole('button', { name: '中止扫描' }).waitFor({ timeout: 5_000 }));
  await cancelButton.focus();
  await cancelButton.click();
  await assert.doesNotReject(() => page.getByText('已请求中止，正在等待引擎停止。').waitFor({ timeout: 5_000 }));
  assert.equal(page.url(), runURL, 'cancel should not navigate away from the run detail');
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
  await page.goto(`${baseURL}/tools/nmap?project_id=${projectID}`, { waitUntil: 'networkidle' });
  await page.locator('[data-tool-run-feedback][data-mounted="true"]').waitFor();
  await page.locator('textarea[name="raw_args"]').fill('-sn 192.0.2.20');
  await page.getByRole('button', { name: '启动 nmap' }).click();
  await assert.doesNotReject(() => page.getByRole('link', { name: '查看本次完整结果' }).waitFor({ timeout: 5_000 }));
  await assert.doesNotReject(() => page.getByText(/工具运行已完成|工具运行已结束/).waitFor({ timeout: 5_000 }));

  await seedRun(`INSERT INTO scan_runs (run_id, project_id, zone_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir) VALUES
    ('browser-errors', '${projectID}', 'I', '192.0.2.30', '443', 'normal', 'completed_with_errors', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{"zone_id":"I","target":"192.0.2.30","ports":"443","profile":"normal"}', ''),
    ('browser-failed', '${projectID}', 'I', '192.0.2.32', '443', 'normal', 'failed', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', 'fixture failure', '{"zone_id":"I","target":"192.0.2.32","ports":"443","profile":"normal"}', ''),
    ('browser-interrupted', '${projectID}', 'I', '192.0.2.31', '80,443', 'normal', 'interrupted', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{"zone_id":"I","target":"192.0.2.31","ports":"80,443","profile":"fast"}', '');
    INSERT INTO detection_checks (run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at) VALUES
    ('browser-errors', '192.0.2.30', 443, 'tcp', 'nuclei', 'failed', 'command_failed', 'fixture failure', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z'),
    ('browser-interrupted', '192.0.2.31', 443, 'tcp', 'nuclei', 'interrupted', 'lease_expired', 'fixture interruption', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z');`);
  await page.goto(`${baseURL}/runs/browser-errors`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('completed_with_errors').waitFor());
  const statusBackgrounds = await page.locator('.status-badge').evaluate((element) => {
    const completed = document.createElement('span');
    completed.className = 'status-badge status-completed';
    document.body.append(completed);
    const colors = [getComputedStyle(element).backgroundColor, getComputedStyle(completed).backgroundColor];
    completed.remove();
    return colors;
  });
  assert.notEqual(statusBackgrounds[0], 'rgba(0, 0, 0, 0)', 'completed_with_errors needs a status color');
  assert.notEqual(statusBackgrounds[0], statusBackgrounds[1], 'completed_with_errors must remain distinguishable from completed');
  await assert.doesNotReject(() => page.getByText(/检测检查：.*失败 1/).waitFor({ timeout: 5_000 }));
  await page.goto(`${baseURL}/runs/browser-failed`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('failed', { exact: true }).waitFor());
  await assert.doesNotReject(() => page.getByText('扫描失败，请查看最新事件。').waitFor());
  await page.goto(`${baseURL}/runs/browser-interrupted`, { waitUntil: 'networkidle' });
  await assert.doesNotReject(() => page.getByText('interrupted', { exact: true }).waitFor());
  await assert.doesNotReject(() => page.getByText(/检测检查：.*已中断 1/).waitFor({ timeout: 5_000 }));
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
  await page.locator('[data-report-interactions][data-mounted="true"]').waitFor();
  const reportOutline = page.locator('.report-outline');
  await assert.doesNotReject(() => reportOutline.waitFor());
  assert.ok((await reportOutline.locator('a[href^="#"]').count()) >= 2, 'report outline should list section anchors');
  await page.waitForFunction(() => document.querySelector('.report-outline a')?.classList.contains('active'), undefined, { timeout: 5_000 });
  const firstOutlineLink = reportOutline.locator('a[href^="#"]').first();
  assert.equal(await firstOutlineLink.evaluate((link) => link.classList.contains('active')), true, 'scroll-spy should activate the first outline link');
  await page.screenshot({ path: path.join(artifactDir, 'report-dark.png'), fullPage: true });
  const serviceFilter = page.getByRole('button', { name: '端口与服务' });
  await serviceFilter.focus();
  await serviceFilter.press('Enter');
  await assert.doesNotReject(() => page.getByRole('dialog', { name: '端口与服务过滤' }).waitFor());
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => page.getByRole('dialog', { name: '端口与服务过滤' }).waitFor({ state: 'hidden' }));
  await serviceFilter.click();
  await page.getByRole('textbox', { name: '特定端口' }).fill('80');
  await page.getByRole('button', { name: '应用', exact: true }).click();
  await page.waitForURL(/port=80/);
  await page.locator('[data-report-interactions][data-mounted="true"]').waitFor();
  await assert.doesNotReject(() => page.getByText('192.0.2.10').first().waitFor());
  await page.getByRole('button', { name: /移除端口 80/ }).click();
  await page.waitForURL((url) => !url.searchParams.has('port'));
  await page.getByRole('tab', { name: '按主机' }).click();
  await page.waitForURL(/view=hosts/);
  const hostTab = page.getByRole('tab', { name: '按主机' });
  await hostTab.focus();
  await hostTab.press('ArrowRight');
  await page.waitForURL(/view=vulnerabilities/);
  await page.getByRole('tab', { name: '按主机' }).click();
  await page.waitForURL(/view=hosts/);
  await page.getByRole('link', { name: '下一页' }).first().click();
  await page.getByRole('button', { name: '复制 IP' }).first().click();

  await page.setViewportSize({ width: 1280, height: 960 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false);

  const outline = page.locator('.report-outline');
  assert.equal(await outline.isVisible(), true, 'report outline must remain visible at 1280px');
  const firstLink = outline.locator('a[href^="#"]').first();
  await firstLink.focus();
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('href')?.startsWith('#')), true, 'first outline link must be focusable at 1280px');


  // Workbench regression: seed a completed run with a non-info finding and verify
  // the candidate renders, the verify dialog opens, and the first focusable control
  // inside the dialog is keyboard-reachable.
  await seedRun(`INSERT INTO scan_runs (run_id, project_id, zone_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir, include_in_report) VALUES
    ('browser-workbench', '${projectID}', 'dmz', '192.0.2.50', '445', 'normal', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{}', '', 1);
    INSERT INTO fingerprints (run_id, ip, port, service, product, version, normalized, is_web, url, protocol, cpe, extrainfo, tunnel) VALUES
    ('browser-workbench', '192.0.2.50', 445, 'smb', '', '', 'smb', 0, '', 'tcp', '', '', '');
    INSERT INTO findings (run_id, ip, port, source, finding_id, severity, summary, target, output, protocol, scope) VALUES
    ('browser-workbench', '192.0.2.50', 445, 'nuclei', 'smb-signing', 'high', 'Workbench smoke finding', '192.0.2.50:445', '', 'tcp', '');
    INSERT INTO report_verifications (id, project_id, zone_id, vulnerability_key, outcome, title, severity, description, remediation, notes, included, position, created_at, updated_at) VALUES
    ('browser-evidence', '${projectID}', 'dmz', 'smb-signing', 'confirmed', 'Browser evidence', 'high', '', '', '', 0, 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
    INSERT INTO verification_evidence (id, verification_id, relative_path, media_type, sha256, width, height, caption, position, created_at) VALUES
    ('browser-evidence-image', 'browser-evidence', 'missing.png', 'image/png', '', 1, 1, 'Browser evidence', 0, '2026-01-01T00:00:00Z');`);
  await page.goto(`${baseURL}/reports/browser-workbench`, { waitUntil: 'networkidle' });
  await page.locator('[data-report-interactions][data-mounted="true"]').waitFor();
  await page.getByRole('button', { name: '生成 Nuclei 命令' }).click();
  const reportCommandDialog = page.getByRole('dialog', { name: '生成 Nuclei 命令' });
  await assert.doesNotReject(() => reportCommandDialog.waitFor());
  await assert.doesNotReject(() => reportCommandDialog.locator('pre.command-pre').filter({ hasText: /nuclei/i }).waitFor({ timeout: 10_000 }));
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => reportCommandDialog.waitFor({ state: 'hidden' }));

  await page.goto(`${baseURL}${projectURL}/workbench`, { waitUntil: 'networkidle' });
  await page.locator('[data-workbench][data-mounted="true"]').waitFor();
  await assert.doesNotReject(() => page.getByRole('heading', { name: 'SMB 签名未启用' }).waitFor());
  const verifyButton = page.getByRole('button', { name: '验证 / 编辑' }).first();
  await verifyButton.focus();
  await verifyButton.click();
  const dialog = page.locator('dialog.verify-dialog');
  await assert.doesNotReject(() => dialog.waitFor());
  await page.waitForFunction(() => {
    const active = document.activeElement;
    return active?.tagName === 'INPUT' || active?.tagName === 'SELECT' || active?.tagName === 'TEXTAREA';
  });
  const focusedName = await page.evaluate(() => document.activeElement?.getAttribute('name') || document.activeElement?.tagName || '');
  assert.ok(focusedName === 'title' || focusedName === 'INPUT', `verify dialog first focusable should be reachable, got ${focusedName}`);
  let evidenceUploadAttempts = 0;
  await page.route(`**/projects/${projectID}/verifications/browser-evidence/evidence`, async (route) => {
    if (route.request().method() === 'POST' && ++evidenceUploadAttempts === 1) {
      await route.fulfill({ status: 500, body: 'simulated evidence upload failure' });
      return;
    }
    await route.continue();
  });
  await dialog.locator('input[type=file]').setInputFiles([
    {
      name: 'retry-failed.png',
      mimeType: 'image/png',
      buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL1XwAAAABJRU5ErkJggg==', 'base64'),
    },
    {
      name: 'retry-succeeds.png',
      mimeType: 'image/png',
      buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL1XwAAAABJRU5ErkJggg==', 'base64'),
    },
  ]);
  await dialog.getByRole('button', { name: '保存验证' }).click();
  await assert.doesNotReject(() => dialog.getByRole('alert').waitFor());
  const retryButton = dialog.getByRole('button', { name: '重试' });
  await assert.doesNotReject(() => retryButton.waitFor());
  assert.equal(await dialog.locator('.evidence-item').getByRole('button', { name: '删除' }).count(), 2, 'a successful sibling upload must remain visible while another file can be retried');
  await retryButton.click();
  await assert.doesNotReject(() => retryButton.waitFor({ state: 'hidden' }));
  await page.unroute(`**/projects/${projectID}/verifications/browser-evidence/evidence`);
  const evidenceDeleteButtons = dialog.locator('.evidence-item').getByRole('button', { name: '删除' });
  const evidenceDeleteCount = await evidenceDeleteButtons.count();
  const evidenceDeleteButton = evidenceDeleteButtons.first();
  await evidenceDeleteButton.click();
  const evidenceConfirmDialog = page.getByRole('dialog', { name: '删除截图' });
  await assert.doesNotReject(() => evidenceConfirmDialog.waitFor());
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => evidenceConfirmDialog.waitFor({ state: 'hidden' }));
  assert.equal(await evidenceDeleteButton.evaluate((button) => document.activeElement === button), true, 'closing evidence confirmation should restore focus');
  await evidenceDeleteButton.click();
  await assert.doesNotReject(() => evidenceConfirmDialog.waitFor());
  await evidenceConfirmDialog.getByRole('button', { name: '删除' }).click();
  await assert.doesNotReject(() => page.getByText('截图已删除').waitFor());
  await page.waitForFunction((expected) => document.querySelectorAll('dialog.verify-dialog .evidence-item button').length === expected, evidenceDeleteCount - 1);
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => dialog.waitFor({ state: 'hidden' }));
  assert.equal(await verifyButton.evaluate((button) => document.activeElement === button), true, 'closing the verification dialog should restore focus to its trigger');

  // New-verification regression: two included runs in one zone must create one
  // snake_case payload, preserve both sources/assets, and upload PNG evidence.
  await seedRun(`DELETE FROM verification_evidence WHERE verification_id = 'browser-evidence';
    DELETE FROM report_verifications WHERE id = 'browser-evidence';
    INSERT INTO scan_runs (run_id, project_id, zone_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir, include_in_report) VALUES
    ('browser-workbench-2', '${projectID}', 'dmz', '192.0.2.51', '445', 'normal', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z', '', '{}', '', 1);
    INSERT INTO fingerprints (run_id, ip, port, service, product, version, normalized, is_web, url, protocol, cpe, extrainfo, tunnel) VALUES
    ('browser-workbench-2', '192.0.2.51', 445, 'smb', '', '', 'smb', 0, '', 'tcp', '', '', '');
    INSERT INTO findings (run_id, ip, port, source, finding_id, severity, summary, target, output, protocol, scope) VALUES
    ('browser-workbench-2', '192.0.2.51', 445, 'nuclei', 'smb-signing', 'high', 'Workbench smoke finding', '192.0.2.51:445', '', 'tcp', '');`);
  await page.reload({ waitUntil: 'networkidle' });
  await page.getByRole('button', { name: '验证 / 编辑' }).first().click();
  await assert.doesNotReject(() => dialog.waitFor());
  await dialog.locator('input[type=file]').setInputFiles({
    name: 'same-zone-proof.png',
    mimeType: 'image/png',
    buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL1XwAAAABJRU5ErkJggg==', 'base64'),
  });
  const createResponse = page.waitForResponse((response) =>
    response.request().method() === 'POST' && response.url() === `${baseURL}/projects/${projectID}/verifications`,
  );
  const evidenceResponse = page.waitForResponse((response) =>
    response.request().method() === 'POST' && /\/evidence$/.test(new URL(response.url()).pathname),
  );
  await dialog.getByRole('button', { name: '保存验证' }).click();
  const createdResponse = await createResponse;
  assert.equal(createdResponse.status(), 201);
  const createPayload = createdResponse.request().postDataJSON();
  assert.equal(createPayload.zone_id, 'dmz');
  assert.equal(createPayload.ZoneID, undefined, 'create payload must use the public snake_case contract');
  assert.equal(createPayload.assets.length, 2);
  assert.deepEqual(new Set(createPayload.sources.map((source) => source.run_id)), new Set(['browser-workbench', 'browser-workbench-2']));
  const created = await createdResponse.json();
  assert.equal((await evidenceResponse).status(), 201, 'PNG evidence must be uploaded after creation');
  await page.waitForURL(`${baseURL}${projectURL}/workbench`);

  await page.getByRole('button', { name: '验证 / 编辑' }).first().click();
  await assert.doesNotReject(() => dialog.waitFor());
  await dialog.locator('input[name="title"]').fill('Updated browser verification');
  const updateRequest = page.waitForRequest((request) =>
    request.method() === 'POST' && request.url() === `${baseURL}/projects/${projectID}/verifications/${created.ID}`,
  );
  await dialog.getByRole('button', { name: '保存验证' }).click();
  const updatePayload = (await updateRequest).postDataJSON();
  assert.deepEqual(
    Object.keys(updatePayload).sort(),
    ['description', 'included', 'notes', 'outcome', 'position', 'remediation', 'severity', 'title', 'vulnerability_key', 'zone_id'],
    'update payload must use only the public snake_case contract',
  );
  assert.equal(updatePayload.zone_id, 'dmz');
  assert.equal(updatePayload.assets, undefined, 'update payload must not leak create-only relations');
  assert.equal(updatePayload.sources, undefined, 'update payload must not leak create-only relations');
  await page.waitForURL(`${baseURL}${projectURL}/workbench`);
  const updatedVerification = await page.evaluate(async ({ projectID, verificationID }) => {
    const response = await fetch(`/projects/${projectID}/verifications/${verificationID}`);
    if (!response.ok) throw new Error(await response.text());
    return await response.json();
  }, { projectID, verificationID: created.ID });
  assert.equal(updatedVerification.Verification.Title, 'Updated browser verification');
  assert.equal(updatedVerification.Verification.ZoneID, 'dmz');

  // Workbench command dialog regression: generated command text should render.
  await page.locator('details.context-actions summary').first().click();
  await page.getByRole('button', { name: 'Nuclei 命令' }).first().click();
  const commandDialog = page.locator('dialog').filter({ has: page.locator('pre.command-pre') });
  await assert.doesNotReject(() => commandDialog.waitFor());
  const commandPre = commandDialog.locator('pre.command-pre');
  await assert.doesNotReject(() => commandPre.filter({ hasText: /nuclei|nmap/i }).waitFor({ timeout: 10_000 }));
  const commandText = await commandPre.textContent();
  assert.ok(
    /nuclei|nmap/i.test(commandText || ''),
    `command dialog should show a non-empty nuclei/nmap command, got: ${commandText}`,
  );
  await page.keyboard.press('Escape');
  await assert.doesNotReject(() => commandDialog.waitFor({ state: 'hidden' }));

  await page.setViewportSize({ width: 1440, height: 960 });
  await page.goto(`${baseURL}/config`, { waitUntil: 'networkidle' });
  await page.locator('.settings-nav').waitFor();
  await page.evaluate(() => document.getElementById('config-appearance').scrollIntoView({ block: 'center' }));
  await page.waitForTimeout(500);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark', 'theme preference should persist on config page');

  assert.equal(await page.locator('.settings-nav a[href="#config-appearance"]').evaluate(el => el.classList.contains('active')), true, 'appearance should be active initially');

  await page.evaluate(() => document.getElementById('config-timeouts').scrollIntoView({ block: 'center' }));
  await page.waitForTimeout(500);

  assert.equal(await page.locator('.settings-nav a[href="#config-timeouts"]').evaluate(el => el.classList.contains('active')), true, 'timeouts should be active after scroll');
  assert.equal(await page.locator('.settings-nav a[href="#config-engine"]').evaluate(el => el.classList.contains('active')), false, 'engine should not be active after scroll');

  await page.evaluate(() => document.getElementById('config-appearance').scrollIntoView({ block: 'center' }));
  await page.waitForTimeout(500);
  const toggleBtnConfig = page.locator('.theme-toggle-btn-single').first();
  await toggleBtnConfig.click();
  await page.waitForTimeout(150);
  assert.equal(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'light');

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
