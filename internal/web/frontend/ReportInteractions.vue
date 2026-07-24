<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

type Panel = 'severity' | 'service' | 'source';
type CommandResponse = {
  commands?: Array<{ full_command?: string; target_file?: string; tool_args?: string } | string>;
  full_command?: string;
  tool_args?: string;
  warning?: string;
};
type CommandItem = { full_command?: string; target_file?: string; tool_args?: string } | string;

const root = ref<HTMLElement>();
const commandDialog = ref<HTMLDialogElement>();
const openPanel = ref<Panel | ''>('');
const lastTrigger = ref<HTMLButtonElement>();
const current = new URL(window.location.href);
const supportedSeverities = ['critical', 'high', 'medium', 'low', 'info'];
const viewOptions = [{ value: 'ports', label: '按端口' }, { value: 'hosts', label: '按主机' }, { value: 'vulnerabilities', label: '按漏洞' }] as const;
const searchText = ref(current.searchParams.get('ip') || current.searchParams.get('q') || '');
const port = ref(current.searchParams.get('port') || '');
const service = ref(current.searchParams.get('service') || '');
const source = ref(current.searchParams.get('source') || '');
const view = ref(['hosts', 'vulnerabilities'].includes(current.searchParams.get('view') || '') ? current.searchParams.get('view') || 'ports' : 'ports');
const severities = ref([...new Set(current.searchParams.getAll('severity').flatMap((item) => item.split(',')))].filter((item) => supportedSeverities.includes(item)));
const commandTitle = ref('生成检测命令');
const commandMessage = ref('');
const commandBody = ref('');
const commandToolLink = ref('');

const activeFilters = computed(() => {
  const filters: Array<{ key: string; value: string; label: string }> = [];
  const value = searchText.value.trim();
  if (value) filters.push({ key: 'search', value, label: current.searchParams.get('ip') ? 'IP' : '关键词' });
  if (port.value.trim()) filters.push({ key: 'port', value: port.value.trim(), label: '端口' });
  if (service.value.trim()) filters.push({ key: 'service', value: service.value.trim(), label: '服务' });
  if (source.value.trim()) filters.push({ key: 'source', value: source.value.trim(), label: '数据源' });
  for (const severity of severities.value) filters.push({ key: 'severity', value: severity, label: '级别' });
  if (view.value !== 'ports') filters.push({ key: 'view', value: view.value === 'hosts' ? '主机聚合' : '漏洞聚合', label: '视图' });
  return filters;
});

function reportPath() {
  return window.location.pathname.replace(/\/$/, '');
}

function isIPFilter(value: string) {
  return /^([0-9]{1,3}\.){3}[0-9]{1,3}(\/[0-9]{1,2})?$/.test(value) || /^([0-9]{1,3}\.){3}[0-9]{1,3}-[0-9]{1,3}$/.test(value) || value.includes(',');
}

function applyFilters() {
  const next = new URL(window.location.href);
  for (const key of ['ip', 'q', 'port', 'service', 'source', 'view', 'severity', 'assets_page', 'findings_page']) next.searchParams.delete(key);
  const search = searchText.value.trim();
  if (search) next.searchParams.set(isIPFilter(search) ? 'ip' : 'q', search);
  if (port.value.trim()) next.searchParams.set('port', port.value.trim());
  if (service.value.trim()) next.searchParams.set('service', service.value.trim());
  if (source.value.trim()) next.searchParams.set('source', source.value.trim());
  if (view.value !== 'ports') next.searchParams.set('view', view.value);
  for (const severity of severities.value) next.searchParams.append('severity', severity);
  window.location.assign(next.toString());
}

function togglePanel(panel: Panel, event: MouseEvent) {
  lastTrigger.value = event.currentTarget as HTMLButtonElement;
  openPanel.value = openPanel.value === panel ? '' : panel;
}

function closePanel() {
  const trigger = lastTrigger.value;
  openPanel.value = '';
  void nextTick(() => trigger?.focus());
}

function selectView(nextView: string) {
  view.value = nextView;
  applyFilters();
}

function handleViewKeydown(event: KeyboardEvent) {
  const index = viewOptions.findIndex((item) => item.value === view.value);
  const target = event.key === 'Home' ? 0 : event.key === 'End' ? viewOptions.length - 1 : event.key === 'ArrowRight' ? (index + 1) % viewOptions.length : event.key === 'ArrowLeft' ? (index - 1 + viewOptions.length) % viewOptions.length : -1;
  if (target < 0) return;
  event.preventDefault();
  selectView(viewOptions[target].value);
}

function removeFilter(key: string, value: string) {
  if (key === 'search') searchText.value = '';
  if (key === 'port') port.value = '';
  if (key === 'service') service.value = '';
  if (key === 'source') source.value = '';
  if (key === 'severity') severities.value = severities.value.filter((item) => item !== value);
  if (key === 'view') view.value = 'ports';
  applyFilters();
}

async function writeClipboard(text: string) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  const copied = document.execCommand('copy');
  textarea.remove();
  if (!copied) throw new Error('copy failed');
}

async function copyButton(button: HTMLElement, text: string | Promise<string>) {
  const original = button.innerHTML;
  button.setAttribute('disabled', '');
  try {
    await writeClipboard((await text).trimEnd());
    button.textContent = '已复制';
  } catch {
    button.textContent = '复制失败';
  }
  window.setTimeout(() => {
    button.removeAttribute('disabled');
    button.innerHTML = original;
  }, 1200);
}

async function openCommand(button: HTMLElement, batch: boolean) {
  const tool = button.dataset[batch ? 'batchTool' : 'commandTool'] || '';
  const key = button.dataset[batch ? 'batchGroup' : 'commandKey'] || '';
  if (!tool || !key) return;
  commandTitle.value = button.textContent?.trim() || '生成检测命令';
  commandMessage.value = batch ? '正在生成批量命令，不会启动扫描。' : '正在生成，不会启动扫描。';
  commandBody.value = '';
  commandToolLink.value = '';
  commandDialog.value?.showModal();
  try {
    const endpoint = new URL(`${reportPath()}/commands${batch ? '/batch' : ''}`, window.location.origin);
    endpoint.search = window.location.search;
    const body = new URLSearchParams(batch ? { group_key: key, tool } : { finding_key: key, tool });
    const response = await fetch(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body });
    if (!response.ok) throw new Error((await response.text()).trim() || '命令不可用');
    const result = await response.json() as CommandResponse;
    const commands: CommandItem[] = result.commands || [{ full_command: result.full_command, tool_args: result.tool_args }];
    const commandLines = commands.map((item) => typeof item === 'string' ? item : item.full_command || '').filter(Boolean);
    const files = commands.flatMap((item) => typeof item === 'string' || !item.target_file ? [] : [item.target_file]);
    commandBody.value = commandLines.join('\n');
    commandMessage.value = batch
      ? `${result.warning ? `${result.warning}；` : ''}共 ${commands.length} 条命令${files.length ? `；目标文件：${files.join('、')}` : ''}；请人工确认后运行。`
      : '请人工确认后运行；此操作未启动扫描。';
    const first = commands[0];
    const toolArgs = typeof first === 'string' ? '' : first.tool_args || result.tool_args || '';
    if (tool !== 'msf' && commands.length === 1 && toolArgs) commandToolLink.value = `/tools/${tool}?raw_args=${encodeURIComponent(toolArgs)}`;
  } catch (error) {
    commandMessage.value = error instanceof Error ? error.message : String(error);
  }
}

async function copyCommand() {
  if (!commandBody.value) return;
  await writeClipboard(commandBody.value);
}

function handleDocumentClick(event: MouseEvent) {
  const target = event.target instanceof Element ? event.target : null;
  if (!target) return;
  if (root.value && !root.value.contains(target)) openPanel.value = '';
  const command = target.closest<HTMLElement>('.command-generate-btn');
  if (command) {
    event.preventDefault();
    void openCommand(command, false);
    return;
  }
  const batch = target.closest<HTMLElement>('.batch-command-btn');
  if (batch) {
    event.preventDefault();
    void openCommand(batch, true);
    return;
  }
  const details = target.closest<HTMLButtonElement>('[data-finding-details]');
  if (details) {
    const row = document.getElementById(`finding-details-${details.dataset.findingDetails}`);
    if (!row) return;
    row.hidden = !row.hidden;
    details.classList.toggle('active-toggle', !row.hidden);
    details.setAttribute('aria-expanded', String(!row.hidden));
    return;
  }
  const copy = target.closest<HTMLElement>('[data-copy-text],[data-copy-url],[data-copy-target-id]');
  if (!copy) return;
  void copyButton(copy, (async () => {
    let text = copy.dataset.copyText || '';
    if (copy.dataset.copyUrl) {
      const response = await fetch(copy.dataset.copyUrl);
      if (!response.ok) throw new Error('copy fetch failed');
      text = await response.text();
    }
    if (copy.dataset.copyTargetId) text = document.getElementById(copy.dataset.copyTargetId)?.textContent || '';
    return text;
  })());
}

function handleDocumentChange(event: Event) {
  const target = event.target instanceof HTMLSelectElement ? event.target : null;
  if (target?.matches('[data-page-size]') && target.value) window.location.assign(target.value);
}

function handleDocumentSubmit(event: SubmitEvent) {
  const form = event.target instanceof HTMLFormElement ? event.target : null;
  if (!form?.matches('form.page-jump')) return;
  event.preventDefault();
  const input = form.querySelector<HTMLInputElement>('input[type="number"]');
  if (!input) return;
  const page = Number(input.value);
  const max = Number(input.max);
  if (!Number.isInteger(page) || page < 1 || (Number.isFinite(max) && page > max)) return;
  const next = new URL(window.location.href);
  next.searchParams.set(input.name, String(page));
  window.location.assign(next.toString());
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick);
  document.addEventListener('change', handleDocumentChange);
  document.addEventListener('submit', handleDocumentSubmit);
});

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick);
  document.removeEventListener('change', handleDocumentChange);
  document.removeEventListener('submit', handleDocumentSubmit);
});
</script>

<template>
  <section ref="root" class="panel report-filter" @keydown.esc.stop="closePanel">
    <form class="search-console-form" @submit.prevent="applyFilters">
      <div class="search-console-bar">
        <div class="search-input-wrapper">
          <span class="search-icon" aria-hidden="true">⌕</span>
          <input v-model="searchText" type="search" :placeholder="view === 'vulnerabilities' ? '输入漏洞名称或漏洞 ID 筛选' : '输入主机 IP、网段或漏洞关键词检索'" aria-label="报告搜索">
        </div>
        <button class="button button-primary search-submit-btn" type="submit">应用检索</button>
      </div>

      <div class="filter-popover-row">
        <div class="popover-wrapper">
          <button class="popover-trigger-btn" type="button" aria-controls="report-severity-filter" :aria-expanded="openPanel === 'severity'" @click="togglePanel('severity', $event)">
            危险级别 <span v-if="severities.length" class="trigger-active-count">{{ severities.length }}</span>
          </button>
          <div v-show="openPanel === 'severity'" id="report-severity-filter" class="popover-panel" role="dialog" aria-label="过滤危险级别">
            <div class="popover-panel-body popover-checkbox-list">
              <label v-for="severity in supportedSeverities" :key="severity" class="popover-checkbox-item">
                <input v-model="severities" type="checkbox" :value="severity">
                <span :class="['severity-dot', severity]"></span>{{ severity }}
              </label>
            </div>
            <div class="popover-panel-footer"><button class="button button-primary" type="button" @click="applyFilters">应用</button></div>
          </div>
        </div>

        <div class="popover-wrapper">
          <button class="popover-trigger-btn" type="button" aria-controls="report-service-filter" :aria-expanded="openPanel === 'service'" @click="togglePanel('service', $event)">端口与服务</button>
          <div v-show="openPanel === 'service'" id="report-service-filter" class="popover-panel" role="dialog" aria-label="端口与服务过滤">
            <div class="popover-panel-body popover-form-group">
              <label>特定端口<input v-model="port" inputmode="numeric" placeholder="例如: 80"></label>
              <label>服务精确匹配<input v-model="service" placeholder="例如: redis"></label>
            </div>
            <div class="popover-panel-footer"><button class="button button-primary" type="button" @click="applyFilters">应用</button></div>
          </div>
        </div>

        <div class="popover-wrapper">
          <button class="popover-trigger-btn" type="button" aria-controls="report-source-filter" :aria-expanded="openPanel === 'source'" @click="togglePanel('source', $event)">数据源</button>
          <div v-show="openPanel === 'source'" id="report-source-filter" class="popover-panel" role="dialog" aria-label="数据源过滤">
            <div class="popover-panel-body popover-form-group"><label>探针数据源<input v-model="source" placeholder="例如: nuclei"></label></div>
            <div class="popover-panel-footer"><button class="button button-primary" type="button" @click="applyFilters">应用</button></div>
          </div>
        </div>

        <a :href="reportPath()" class="button button-secondary filter-reset">重置筛选</a>
      </div>

      <div class="report-view-tabs" role="tablist" aria-label="报告视图">
        <button v-for="item in viewOptions" :key="item.value" class="report-view-tab" type="button" role="tab" :aria-selected="view === item.value" @click="selectView(item.value)" @keydown="handleViewKeydown">{{ item.label }}</button>
      </div>

      <div v-if="activeFilters.length" class="active-filter-badges">
        <span class="active-badges-label">活动过滤器：</span>
        <div class="badges-row-content">
          <button v-for="filter in activeFilters" :key="`${filter.key}-${filter.value}`" class="filter-badge-tag" type="button" :aria-label="`移除${filter.label} ${filter.value}`" @click="removeFilter(filter.key, filter.value)">{{ filter.label }}: {{ filter.value }} ×</button>
        </div>
      </div>
    </form>
  </section>

  <Teleport to="body">
    <dialog ref="commandDialog" class="panel" aria-labelledby="report-command-dialog-title" @close="commandBody = ''">
      <div class="panel-heading"><h3 id="report-command-dialog-title">{{ commandTitle }}</h3><button class="button button-secondary" type="button" @click="commandDialog?.close()">关闭</button></div>
      <p class="meta-line">{{ commandMessage }}</p>
      <pre class="command-pre">{{ commandBody }}</pre>
      <div class="header-actions"><button class="button button-secondary" type="button" :disabled="!commandBody" @click="copyCommand">复制完整命令</button><a v-if="commandToolLink" class="button button-primary" :href="commandToolLink">带参数打开工具页</a></div>
    </dialog>
  </Teleport>
</template>

<style scoped>
.report-view-tabs { display: flex; gap: 0.35rem; margin-top: 1rem; border-bottom: 1px solid var(--border); }
.report-view-tab { padding: 0.55rem 0.85rem; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--muted); cursor: pointer; font: inherit; }
.report-view-tab[aria-selected="true"] { border-color: var(--primary); color: var(--primary); font-weight: 700; }
.report-view-tab:focus-visible { outline: 2px solid var(--primary); outline-offset: -2px; }
.filter-badge-tag { cursor: pointer; font: inherit; }
</style>
