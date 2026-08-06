<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

type Panel = 'severity' | 'service' | 'source';
type ServiceFacet = { raw_value: string; label: string; count: number };
type PivotFacet = { dimension: string; raw_value: string; label: string; count: number };
type ServiceMatrix = { row_dimension: string; col_dimension: string; rows: string[]; cols: string[]; cells: number[][] };
type CommandGate = {
  level: string;
  review_status: string;
  safety_mode: string;
  effects: string[];
  cleanup: string;
  message: string;
  acknowledge_label: string;
  challenge: string;
};
type CommandResponse = {
  commands?: Array<{ full_command?: string; target_file?: string; tool_args?: string } | string>;
  full_command?: string;
  tool_args?: string;
  tool_link?: string;
  warning?: string;
};
type CommandItem = { full_command?: string; target_file?: string; tool_args?: string } | string;
type PendingCommandRequest = { endpoint: string; body: string; batch: boolean; tool: string };

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
const props = defineProps<{ serviceFacets?: ServiceFacet[]; pivotFacets?: PivotFacet[]; serviceMatrix?: ServiceMatrix | null }>();
const serviceFacets = computed(() => props.serviceFacets || []);
const pivotDimensions = [
  { key: 'host', label: '主机', param: 'ip' },
  { key: 'port', label: '端口', param: 'port' },
  { key: 'service', label: '服务', param: 'service' },
  { key: 'product', label: '产品', param: 'product' },
  { key: 'vulnerability', label: '漏洞级别', param: 'severity' },
] as const;
const pivotByDimension = computed(() => {
  const groups: Record<string, PivotFacet[]> = {};
  for (const dim of pivotDimensions) groups[dim.key] = [];
  for (const facet of props.pivotFacets || []) {
    if (groups[facet.dimension]) groups[facet.dimension].push(facet);
  }
  return groups;
});
const hasPivots = computed(() => (props.pivotFacets || []).length > 0);
// Go serializes nil slices as JSON null (e.g. an empty matrix on a run with no
// fingerprints); normalize so every access is safe.
const matrix = computed<ServiceMatrix>(() => {
  const m = props.serviceMatrix;
  if (!m || !m.rows || !m.cols || !m.cells) return { row_dimension: '', col_dimension: '', rows: [], cols: [], cells: [] };
  return m;
});
const hasMatrix = computed(() => matrix.value.rows.length > 0);

function applyPivot(dimension: string, rawValue: string) {
  const meta = pivotDimensions.find((item) => item.key === dimension);
  if (!meta) return;
  const next = new URL(window.location.href);
  next.searchParams.delete('assets_page');
  next.searchParams.delete('findings_page');
  next.searchParams.set(meta.param, rawValue);
  window.location.assign(next.toString());
}
const selectableServiceFacets = computed(() => serviceFacets.value.filter((facet) => facet.raw_value !== ''));
const emptyServiceFacet = computed(() => serviceFacets.value.find((facet) => facet.raw_value === ''));
const excludeUnidentified = ref(current.searchParams.get('exclude_unidentified') === '1');
const commandTitle = ref('生成检测命令');
const commandMessage = ref('');
const commandBody = ref('');
const commandToolLink = ref('');
const commandGate = ref<CommandGate | null>(null);
const pendingCommandRequest = ref<PendingCommandRequest | null>(null);
const commandConfirming = ref(false);
type FindingDetail = { severity: string; source: string; id: string; target: string; summary: string; output: string };
const detailDialog = ref<HTMLDialogElement>();
const detailFinding = ref<FindingDetail | null>(null);
let detailTrigger: HTMLButtonElement | undefined;

const activeFilters = computed(() => {
  const filters: Array<{ key: string; value: string; label: string }> = [];
  const value = searchText.value.trim();
  if (value) filters.push({ key: 'search', value, label: current.searchParams.get('ip') ? 'IP' : '关键词' });
  if (port.value.trim()) filters.push({ key: 'port', value: port.value.trim(), label: '端口' });
  if (service.value.trim()) filters.push({ key: 'service', value: service.value.trim(), label: '服务' });
  if (excludeUnidentified.value) filters.push({ key: 'exclude_unidentified', value: '1', label: '排除未识别服务' });
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
  for (const key of ['ip', 'q', 'port', 'service', 'exclude_unidentified', 'source', 'view', 'severity', 'assets_page', 'findings_page']) next.searchParams.delete(key);
  const search = searchText.value.trim();
  if (search) next.searchParams.set(isIPFilter(search) ? 'ip' : 'q', search);
  if (port.value.trim()) next.searchParams.set('port', port.value.trim());
  if (service.value.trim()) next.searchParams.set('service', service.value.trim());
  if (excludeUnidentified.value) next.searchParams.set('exclude_unidentified', '1');
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
  if (key === 'exclude_unidentified') excludeUnidentified.value = false;
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

async function requestCommand(request: PendingCommandRequest, gateToken = '') {
  const body = new URLSearchParams(request.body);
  if (gateToken) {
    body.set('gate_token', gateToken);
    body.set('acknowledge', '1');
  }
  const response = await fetch(request.endpoint, { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body });
  if (response.status === 428) {
    const result = await response.json() as { gate?: CommandGate };
    if (!result.gate?.challenge) throw new Error('命令确认挑战无效');
    commandGate.value = result.gate;
    commandMessage.value = result.gate.message;
    return;
  }
  if (!response.ok) throw new Error((await response.text()).trim() || '命令不可用');
  const result = await response.json() as CommandResponse;
  const commands: CommandItem[] = result.commands || [{ full_command: result.full_command, tool_args: result.tool_args }];
  const commandLines = commands.map((item) => typeof item === 'string' ? item : item.full_command || '').filter(Boolean);
  const files = commands.flatMap((item) => typeof item === 'string' || !item.target_file ? [] : [item.target_file]);
  commandBody.value = commandLines.join('\n');
  commandMessage.value = request.batch
    ? `${result.warning ? `${result.warning}；` : ''}共 ${commands.length} 条命令${files.length ? `；目标文件：${files.join('、')}` : ''}；请人工确认后运行。`
    : '请人工确认后运行；此操作未启动扫描。';
  commandToolLink.value = result.tool_link || '';
  if (commandGate.value) commandGate.value = { ...commandGate.value, challenge: '' };
  pendingCommandRequest.value = null;
}

async function openCommand(button: HTMLElement, batch: boolean) {
  const tool = button.dataset[batch ? 'batchTool' : 'commandTool'] || '';
  const key = button.dataset[batch ? 'batchGroup' : 'commandKey'] || '';
  if (!tool || !key) return;
  commandTitle.value = button.textContent?.trim() || '生成检测命令';
  commandMessage.value = batch ? '正在生成批量命令，不会启动扫描。' : '正在生成，不会启动扫描。';
  commandBody.value = '';
  commandToolLink.value = '';
  commandGate.value = null;
  const endpoint = new URL(`${reportPath()}/commands${batch ? '/batch' : ''}`, window.location.origin);
  endpoint.search = window.location.search;
  const body = new URLSearchParams(batch ? { group_key: key, tool } : { finding_key: key, tool });
  const request = { endpoint: endpoint.toString(), body: body.toString(), batch, tool };
  pendingCommandRequest.value = request;
  commandDialog.value?.showModal();
  try {
    await requestCommand(request);
  } catch (error) {
    commandMessage.value = error instanceof Error ? error.message : String(error);
  }
}

async function confirmCommand() {
  const request = pendingCommandRequest.value;
  const gate = commandGate.value;
  if (!request || !gate?.challenge || commandConfirming.value) return;
  commandConfirming.value = true;
  commandMessage.value = '正在校验一次性确认并生成命令。';
  try {
    await requestCommand(request, gate.challenge);
  } catch (error) {
    commandGate.value = null;
    commandMessage.value = error instanceof Error ? error.message : String(error);
  } finally {
    commandConfirming.value = false;
  }
}

async function copyCommand() {
  if (!commandBody.value) return;
  await writeClipboard(commandBody.value);
}

function openFindingDetail(button: HTMLButtonElement) {
  const ds = button.dataset;
  detailFinding.value = {
    severity: ds.detailSeverity || '',
    source: ds.detailSource || '',
    id: ds.detailId || '',
    target: ds.detailTarget || '',
    summary: ds.detailSummary || '',
    output: ds.detailOutput || '',
  };
  detailTrigger = button;
  button.setAttribute('aria-expanded', 'true');
  detailDialog.value?.showModal();
  void nextTick(() => {
    (window as unknown as { highlightAllEvidences?: () => void }).highlightAllEvidences?.();
  });
}

function closeFindingDetail() {
  detailDialog.value?.close();
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
    openFindingDetail(details);
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
              <label>服务精确匹配
                <select v-model="service">
                  <option value="">全部服务</option>
                  <option v-for="facet in selectableServiceFacets" :key="facet.raw_value" :value="facet.raw_value">{{ facet.label }} ({{ facet.count }})</option>
                </select>
                <span v-if="emptyServiceFacet" class="meta-line">{{ emptyServiceFacet.label }} ({{ emptyServiceFacet.count }})</span>
              </label>
              <label class="popover-checkbox-item"><input v-model="excludeUnidentified" type="checkbox">一键排除未识别服务</label>
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

  <section v-if="hasPivots" class="panel report-pivots">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">多维分析</p>
        <h3>主机 · 端口 · 服务 · 产品 · 漏洞</h3>
        <p class="meta-line">基于当前筛选范围的去重统计，点击任一维度值可下钻筛选。</p>
      </div>
    </div>
    <div class="pivot-grid">
      <div v-for="dim in pivotDimensions" :key="dim.key" class="pivot-dimension">
        <p class="eyebrow pivot-dimension-label">{{ dim.label }}</p>
        <div class="pivot-chips">
          <button v-for="facet in pivotByDimension[dim.key]" :key="`${dim.key}-${facet.raw_value}`" class="pivot-chip" type="button" :aria-label="`按${dim.label} ${facet.label} 下钻筛选`" @click="applyPivot(dim.key, facet.raw_value)">
            <span class="pivot-chip-label">{{ facet.label }}</span>
            <span class="pivot-chip-count">{{ facet.count }}</span>
          </button>
          <p v-if="!pivotByDimension[dim.key].length" class="meta-line">无数据</p>
        </div>
      </div>
    </div>
    <div v-if="hasMatrix" class="pivot-matrix-wrapper">
      <p class="eyebrow">主机 × 服务矩阵</p>
      <div class="scroll-table">
        <table class="data-table pivot-matrix-table">
          <thead>
            <tr>
              <th>主机</th>
              <th v-for="(col, ci) in matrix.cols" :key="`col-${ci}`">{{ col || '未识别' }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, ri) in matrix.rows" :key="`row-${ri}`">
              <th scope="row" class="mono-value">{{ row }}</th>
              <td v-for="(col, ci) in matrix.cols" :key="`cell-${ri}-${ci}`" :class="{ 'matrix-filled': matrix.cells[ri][ci] > 0 }">{{ matrix.cells[ri][ci] || '' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>

  <Teleport to="body">
    <dialog ref="commandDialog" class="panel" aria-labelledby="report-command-dialog-title" @close="commandBody = ''; commandGate = null; pendingCommandRequest = null">
      <div class="panel-heading"><h3 id="report-command-dialog-title">{{ commandTitle }}</h3><button class="button button-secondary" type="button" @click="commandDialog?.close()">关闭</button></div>
      <p class="meta-line">{{ commandMessage }}</p>
      <div v-if="commandGate" class="panel command-gate-panel" role="alert">
        <p><strong>{{ commandGate.level }}</strong> · review={{ commandGate.review_status }} · safety={{ commandGate.safety_mode }}</p>
        <p>{{ commandGate.message }}</p>
        <p v-if="commandGate.level === 'legacy-unknown'"><strong>Effects / Cleanup：</strong>旧 Markdown 未声明</p>
        <div v-if="commandGate.effects.length">
          <p class="eyebrow">Effects</p>
          <ul><li v-for="effect in commandGate.effects" :key="effect"><code>{{ effect }}</code></li></ul>
        </div>
        <p v-if="commandGate.cleanup"><strong>Cleanup：</strong>{{ commandGate.cleanup }}</p>
        <button v-if="commandGate.challenge" class="button button-primary" type="button" :disabled="commandConfirming" @click="confirmCommand">{{ commandGate.acknowledge_label }}</button>
      </div>
      <pre v-if="commandBody" class="command-pre">{{ commandBody }}</pre>
      <div class="header-actions"><button class="button button-secondary" type="button" :disabled="!commandBody" @click="copyCommand">复制完整命令</button><a v-if="commandToolLink" class="button button-primary" :href="commandToolLink">带参数打开工具页</a></div>
    </dialog>
  </Teleport>

  <Teleport to="body">
    <dialog ref="detailDialog" class="report-detail-dialog" aria-labelledby="report-detail-title" @close="detailFinding = null; detailTrigger?.setAttribute('aria-expanded', 'false')">
      <div v-if="detailFinding" class="report-detail-body">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">漏洞详情</p>
            <h3 id="report-detail-title">{{ detailFinding.id || '证据与详情' }}</h3>
          </div>
          <button class="button button-secondary" type="button" @click="closeFindingDetail">关闭</button>
        </div>
        <dl class="report-detail-meta">
          <div><dt>严重级别</dt><dd><span :class="['severity-badge', `sev-${detailFinding.severity}`]">{{ detailFinding.severity }}</span></dd></div>
          <div><dt>安全探针</dt><dd>{{ detailFinding.source }}</dd></div>
          <div><dt>受影响目标</dt><dd class="mono-value">{{ detailFinding.target }}</dd></div>
        </dl>
        <div><p class="eyebrow">漏洞摘要</p><p class="meta-line">{{ detailFinding.summary }}</p></div>
        <div class="panel-heading"><h4>验证证据 / 原始输出</h4><button class="button button-secondary" type="button" data-copy-target-id="report-detail-evidence">复制证据</button></div>
        <pre v-if="detailFinding.output" class="evidence-pre" id="report-detail-evidence">{{ detailFinding.output }}</pre>
        <p v-else class="meta-line">暂无原始证据输出。</p>
      </div>
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
