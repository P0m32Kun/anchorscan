<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';

type ProjectAsset = {
  IP: string;
  Port: number;
  Protocol: string;
  Target: string;
  RunIDs: string[];
};

type ProjectCandidateSource = {
  RunID: string;
  Source: string;
  FindingID: string;
  IP: string;
  Port: number;
  Protocol: string;
};

type ProjectVulnerabilityCandidate = {
  GroupKey: string;
  Title: string;
  Severity: string;
  Description: string;
  Remediation: string;
  Assets: ProjectAsset[];
  Services: string[];
  SourceRuns: string[];
  Sources: ProjectCandidateSource[];
  IsPending: boolean;
  PendingKey: string;
};

type Candidate = ProjectVulnerabilityCandidate & { ZoneID: string };

type Verification = {
  ID: string;
  ProjectID: string;
  ZoneID: string;
  VulnerabilityKey: string;
  Outcome: string;
  Title: string;
  Severity: string;
  Description: string;
  Remediation: string;
  Notes: string;
  Included: boolean;
  Position: number;
};

type EvidenceItem = {
  ID: string;
  VerificationID: string;
  RelativePath: string;
  MediaType: string;
  SHA256: string;
  Width: number;
  Height: number;
  Caption: string;
  Position: number;
};

type VerificationDetail = {
  Verification: Verification;
  Assets: { IP: string; Port: number; Protocol: string; AssetName: string; Position: number }[];
  Sources: ProjectCandidateSource[];
  Evidence: EvidenceItem[];
};

type NegativeGroup = {
  Key: string;
  Title: string;
  Service: string;
  Product: string;
  ZoneID: string;
  Assets: ProjectAsset[];
  NmapCommand: string;
  NucleiCommand: string;
  PortsText: string;
};

type IncompleteItem = {
  Asset: ProjectAsset;
  Fingerprint: { Service: string; Product: string };
  RunID: string;
  ZoneID: string;
  Engines: { Engine: string; Status: string; ReasonCode: string; Detail: string }[];
};

type Zone = {
  ProjectID: string;
  ZoneID: string;
  Name: string;
  SortOrder: number;
};

type CommandResult = {
  commands: { Tool: string; ToolArgs: string; FullCommand: string; TargetFile: string }[];
  warning: string;
  tool_link: string;
};

type Toast = { id: number; message: string; type: 'error' | 'success' };

const props = defineProps<{
  project_id: string;
  project_name: string;
  zones: Zone[];
  zone_names: Record<string, string>;
  candidates: Candidate[];
  negative_groups: NegativeGroup[];
  incomplete_checks: IncompleteItem[];
  verifications: Verification[];
  counts: { positive: number; negative: number; incomplete: number };
  catalog_status: string;
  catalog_diagnostics: string[];
}>();

const tab = ref<'positive' | 'negative' | 'incomplete'>('positive');
const zoneFilter = ref('');
const statusFilter = ref('');
const keywordFilter = ref('');
const severityFilter = ref<string[]>([]);
const serviceFilter = ref('');
const portFilter = ref('');
const runFilter = ref('');
const moreOpen = ref(false);

const verificationMap = computed(() => {
  const map: Record<string, Verification> = {};
  for (const v of props.verifications) map[v.VulnerabilityKey] = v;
  return map;
});

const candidateStatus = (c: Candidate) => {
  const v = verificationMap.value[c.GroupKey];
  if (!v) return 'pending';
  if (v.Included) return 'included';
  return v.Outcome;
};

const candidateStatusLabel = (status: string) => {
  switch (status) {
    case 'included':
      return '已纳入';
    case 'confirmed':
      return '已确认';
    case 'not_observed':
      return '未发现';
    case 'inconclusive':
      return '无法判定';
    default:
      return '待确认';
  }
};

const services = computed(() => {
  const set = new Set<string>();
  for (const c of props.candidates) {
    for (const s of c.Services || []) set.add(s);
  }
  return [...set].sort();
});

const runs = computed(() => {
  const set = new Set<string>();
  for (const c of props.candidates) {
    for (const r of c.SourceRuns || []) set.add(r);
  }
  return [...set].sort();
});

const filteredCandidates = computed(() => {
  const kw = keywordFilter.value.trim().toLowerCase();
  return props.candidates.filter((c) => {
    if (zoneFilter.value && c.ZoneID !== zoneFilter.value) return false;
    const status = candidateStatus(c);
    if (statusFilter.value) {
      if (statusFilter.value === 'pending') {
        if (status !== 'pending') return false;
      } else if (statusFilter.value === 'confirmed') {
        if (status !== 'confirmed') return false;
      } else if (statusFilter.value === 'included') {
        if (status !== 'included') return false;
      }
    }
    if (severityFilter.value.length && !severityFilter.value.includes(c.Severity)) return false;
    if (serviceFilter.value && !(c.Services || []).includes(serviceFilter.value)) return false;
    if (portFilter.value && !(c.Assets || []).some((a) => String(a.Port) === portFilter.value)) return false;
    if (runFilter.value && !(c.SourceRuns || []).includes(runFilter.value)) return false;
    if (kw) {
      const hay = `${c.Title} ${c.Description || ''} ${(c.Assets || []).map((a) => `${a.IP}:${a.Port}`).join(' ')}`.toLowerCase();
      if (!hay.includes(kw)) return false;
    }
    return true;
  });
});

const filteredNegativeGroups = computed(() => {
  const kw = keywordFilter.value.trim().toLowerCase();
  return props.negative_groups.filter((g) => {
    if (zoneFilter.value && g.ZoneID !== zoneFilter.value) return false;
    if (kw) {
      const hay = `${g.Title} ${g.Service} ${g.Product} ${(g.Assets || []).map((a) => `${a.IP}:${a.Port}`).join(' ')}`.toLowerCase();
      if (!hay.includes(kw)) return false;
    }
    return true;
  });
});

const filteredIncomplete = computed(() => {
  const kw = keywordFilter.value.trim().toLowerCase();
  return props.incomplete_checks.filter((ic) => {
    if (zoneFilter.value && ic.ZoneID !== zoneFilter.value) return false;
    if (kw) {
      const hay = `${ic.Asset.IP}:${ic.Asset.Port} ${ic.Fingerprint.Service} ${ic.Fingerprint.Product} ${ic.RunID}`.toLowerCase();
      if (!hay.includes(kw)) return false;
    }
    return true;
  });
});

const activeFilterCount = computed(() => {
  let n = 0;
  if (zoneFilter.value) n++;
  if (statusFilter.value) n++;
  if (keywordFilter.value.trim()) n++;
  if (severityFilter.value.length) n++;
  if (serviceFilter.value) n++;
  if (portFilter.value) n++;
  if (runFilter.value) n++;
  return n;
});

function resetFilters() {
  zoneFilter.value = '';
  statusFilter.value = '';
  keywordFilter.value = '';
  severityFilter.value = [];
  serviceFilter.value = '';
  portFilter.value = '';
  runFilter.value = '';
}

function toggleSeverity(sev: string) {
  const i = severityFilter.value.indexOf(sev);
  if (i >= 0) severityFilter.value.splice(i, 1);
  else severityFilter.value.push(sev);
}

function hostPort(a: ProjectAsset) {
  return `${a.IP}:${a.Port}`;
}

// ---------- Command dialog ----------
const commandDialog = ref<HTMLDialogElement>();
const commandTitle = ref('生成命令');
const commandBody = ref('');
const commandWarning = ref('');
const commandToolLink = ref('');
const commandLoading = ref(false);

function openCommandDialog() {
  commandBody.value = '';
  commandWarning.value = '';
  commandToolLink.value = '';
  commandDialog.value?.showModal();
}

async function fetchCommand(key: string, tool: string, asset: string, verificationID: string) {
  const url = `/projects/${props.project_id}/candidates/${encodeURIComponent(key)}/commands`;
  const body = new URLSearchParams({ tool });
  if (asset && asset !== 'all') body.set('asset', asset);
  if (verificationID) body.set('verification_id', verificationID);
  body.set('return', `/projects/${props.project_id}/workbench`);
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body,
  });
  if (!res.ok) throw new Error((await res.text()).trim() || '命令不可用');
  return (await res.json()) as CommandResult;
}

async function runCommand(key: string, tool: string, asset: string) {
  commandLoading.value = true;
  commandTitle.value = `生成 ${tool} 命令`;
  openCommandDialog();
  const v = verificationMap.value[key];
  try {
    const data = await fetchCommand(key, tool, asset, v?.ID || '');
    commandBody.value = (data.commands || []).map((c) => c.FullCommand).join('\n\n');
    commandWarning.value = data.warning || '';
    commandToolLink.value = data.tool_link || '';
  } catch (e: any) {
    commandBody.value = '';
    commandWarning.value = e.message || String(e);
  } finally {
    commandLoading.value = false;
  }
}

async function copyCommandText() {
  if (!commandBody.value) return;
  try {
    await navigator.clipboard.writeText(commandBody.value);
    showToast('命令已复制', 'success');
  } catch {
    showToast('复制失败', 'error');
  }
}

// ---------- Verify dialog ----------
const verifyDialog = ref<HTMLDialogElement>();
const verifyKey = ref('');
const verifyZoneId = ref('');
const verifyId = ref('');
const verifyTitle = ref('');
const verifySeverity = ref('high');
const verifyOutcome = ref('confirmed');
const verifyDescription = ref('');
const verifyRemediation = ref('');
const verifyCurrent = ref<VerificationDetail | null>(null);
const verifyPendingFiles = ref<{ file: File; caption: string; objectUrl: string }[]>([]);
const verifySaving = ref(false);

function resetVerifyDialog() {
  verifyKey.value = '';
  verifyZoneId.value = '';
  verifyId.value = '';
  verifyTitle.value = '';
  verifySeverity.value = 'high';
  verifyOutcome.value = 'confirmed';
  verifyDescription.value = '';
  verifyRemediation.value = '';
  verifyCurrent.value = null;
  for (const f of verifyPendingFiles.value) URL.revokeObjectURL(f.objectUrl);
  verifyPendingFiles.value = [];
}

const activeCandidate = computed(() => props.candidates.find((c) => c.GroupKey === verifyKey.value));

async function openVerifyDialog(key: string) {
  resetVerifyDialog();
  const c = props.candidates.find((x) => x.GroupKey === key);
  if (!c) return;
  verifyKey.value = c.GroupKey;
  verifyZoneId.value = c.ZoneID;
  verifyTitle.value = c.Title;
  verifySeverity.value = c.Severity || 'high';
  verifyDescription.value = c.Description || '';
  verifyRemediation.value = c.Remediation || '';
  verifyOutcome.value = 'confirmed';

  const v = verificationMap.value[c.GroupKey];
  if (v?.ID) {
    verifyId.value = v.ID;
    try {
      const res = await fetch(`/projects/${props.project_id}/verifications/${v.ID}`);
      if (res.ok) verifyCurrent.value = await res.json();
      if (verifyCurrent.value?.Verification) {
        const ver = verifyCurrent.value.Verification;
        verifyTitle.value = ver.Title;
        verifySeverity.value = ver.Severity;
        verifyOutcome.value = ver.Outcome;
        verifyDescription.value = ver.Description || '';
        verifyRemediation.value = ver.Remediation || '';
      }
    } catch {
      // ignore
    }
  }
  verifyDialog.value?.showModal();
  nextTick(() => verifyDialog.value?.querySelector<HTMLInputElement>('input[name="title"]')?.focus());
}

function stageVerifyFile(file: File) {
  verifyPendingFiles.value.push({ file, caption: '', objectUrl: URL.createObjectURL(file) });
}

function removeVerifyPending(idx: number) {
  const f = verifyPendingFiles.value[idx];
  if (f) URL.revokeObjectURL(f.objectUrl);
  verifyPendingFiles.value.splice(idx, 1);
}

async function deleteEvidence(verificationID: string, evidenceID: string) {
  if (!confirm('确定删除这张截图？')) return;
  const res = await fetch(`/projects/${props.project_id}/verifications/${verificationID}/evidence/${evidenceID}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ _method: 'delete' }),
  });
  if (res.ok) {
    if (verifyCurrent.value) {
      verifyCurrent.value.Evidence = verifyCurrent.value.Evidence.filter((e) => e.ID !== evidenceID);
    }
    showToast('截图已删除', 'success');
  } else {
    showToast('删除失败', 'error');
  }
}

async function uploadEvidence(file: File, caption: string, verificationID: string) {
  const form = new FormData();
  form.append('file', file);
  form.append('caption', caption);
  const res = await fetch(`/projects/${props.project_id}/verifications/${verificationID}/evidence`, {
    method: 'POST',
    body: form,
  });
  if (!res.ok) throw new Error((await res.text()).trim() || '上传失败');
}

function buildVerificationPayload(c: Candidate): VerificationDetail['Verification'] & { assets: any[]; sources: any[] } {
  const assets = (c.Assets || []).map((a, i) => ({
    ip: a.IP,
    port: a.Port,
    protocol: a.Protocol,
    asset_name: hostPort(a),
    position: i,
  }));
  const sources = (c.Sources || []).map((s) => ({
    run_id: s.RunID,
    source: s.Source,
    finding_id: s.FindingID,
    ip: s.IP,
    port: s.Port,
    protocol: s.Protocol,
  }));
  return {
    ID: verifyId.value,
    ProjectID: props.project_id,
    ZoneID: verifyZoneId.value,
    VulnerabilityKey: c.GroupKey,
    Outcome: verifyOutcome.value,
    Title: verifyTitle.value.trim(),
    Severity: verifySeverity.value,
    Description: verifyDescription.value.trim(),
    Remediation: verifyRemediation.value.trim(),
    Notes: '',
    Included: verifyOutcome.value === 'confirmed' || verifyOutcome.value === 'not_observed',
    Position: 0,
    assets,
    sources,
  } as any;
}

async function saveVerification() {
  const c = activeCandidate.value;
  if (!c) return;
  verifySaving.value = true;
  try {
    const payload = buildVerificationPayload(c);
    let res: Response;
    let vid = verifyId.value;
    if (vid) {
      res = await fetch(`/projects/${props.project_id}/verifications/${vid}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          zone_id: payload.ZoneID,
          vulnerability_key: payload.VulnerabilityKey,
          outcome: payload.Outcome,
          title: payload.Title,
          severity: payload.Severity,
          description: payload.Description,
          remediation: payload.Remediation,
          notes: '',
          included: payload.Included,
          position: payload.Position,
        }),
      });
    } else {
      res = await fetch(`/projects/${props.project_id}/verifications`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (res.ok) {
        const created = (await res.json()) as Verification;
        vid = created.ID;
        verifyId.value = vid;
      }
    }
    if (!res.ok) throw new Error((await res.text()).trim() || '保存失败');
    for (const f of verifyPendingFiles.value) {
      await uploadEvidence(f.file, f.caption, vid);
    }
    showToast('验证已保存', 'success');
    verifyDialog.value?.close();
    window.location.reload();
  } catch (e: any) {
    showToast(e.message || '保存失败', 'error');
  } finally {
    verifySaving.value = false;
  }
}

// ---------- Negative dialog ----------
const negativeDialog = ref<HTMLDialogElement>();
const negGroup = ref<NegativeGroup | null>(null);
const negTitle = ref('');
const negSeverity = ref('low');
const negDescription = ref('');
const negZoneId = ref('');
const negPendingFiles = ref<{ file: File; caption: string; objectUrl: string }[]>([]);
const negSaving = ref(false);

function resetNegativeDialog() {
  negGroup.value = null;
  negTitle.value = '';
  negSeverity.value = 'low';
  negDescription.value = '';
  negZoneId.value = '';
  for (const f of negPendingFiles.value) URL.revokeObjectURL(f.objectUrl);
  negPendingFiles.value = [];
}

function openNegativeDialog(group: NegativeGroup) {
  resetNegativeDialog();
  negGroup.value = group;
  negZoneId.value = group.ZoneID;
  negTitle.value = group.Title || group.Service || '服务';
  negativeDialog.value?.showModal();
  nextTick(() => negativeDialog.value?.querySelector<HTMLInputElement>('input[name="negative-title"]')?.focus());
}

function stageNegFile(file: File) {
  negPendingFiles.value.push({ file, caption: '', objectUrl: URL.createObjectURL(file) });
}

function removeNegPending(idx: number) {
  const f = negPendingFiles.value[idx];
  if (f) URL.revokeObjectURL(f.objectUrl);
  negPendingFiles.value.splice(idx, 1);
}

async function saveNegative() {
  if (!negGroup.value?.Assets?.length) return;
  if (negPendingFiles.value.length === 0) {
    showToast('请至少上传一张截图作为证据', 'error');
    return;
  }
  negSaving.value = true;
  try {
    const assets = negGroup.value.Assets.map((a, i) => ({
      ip: a.IP,
      port: a.Port,
      protocol: a.Protocol || 'tcp',
      asset_name: hostPort(a),
      position: i,
    }));
    const key = `neg:${negTitle.value.trim().toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')}`;
    const res = await fetch(`/projects/${props.project_id}/verifications`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        zone_id: negZoneId.value,
        vulnerability_key: key,
        outcome: 'not_observed',
        title: negTitle.value.trim(),
        severity: negSeverity.value,
        description: negDescription.value.trim(),
        remediation: '',
        notes: '',
        included: false,
        position: 0,
        assets,
        sources: [],
      }),
    });
    if (!res.ok) throw new Error((await res.text()).trim() || '创建失败');
    const created = (await res.json()) as Verification;
    for (const f of negPendingFiles.value) {
      await uploadEvidence(f.file, f.caption, created.ID);
    }
    showToast('负向验证已提交', 'success');
    negativeDialog.value?.close();
    window.location.reload();
  } catch (e: any) {
    showToast(e.message || '提交失败', 'error');
  } finally {
    negSaving.value = false;
  }
}

// ---------- Clipboard / copy ----------
async function copyText(text: string, label = '内容') {
  try {
    await navigator.clipboard.writeText(text);
    showToast(`${label}已复制`, 'success');
  } catch {
    showToast('复制失败', 'error');
  }
}

async function copyCard(c: Candidate) {
  const lines = [
    `漏洞名\n${c.Title}`,
    `漏洞简介\n${c.Description || ''}`,
    `漏洞资产\n${(c.Assets || []).map(hostPort).join('\n')}`,
    `修复建议\n${c.Remediation || ''}`,
  ];
  await copyText(lines.join('\n\n').trimEnd(), '候选');
}

// ---------- Paste / drag-drop ----------
function imagesFromClipboard(items?: DataTransferItemList | null) {
  const files: File[] = [];
  if (!items) return files;
  for (const item of items) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const f = item.getAsFile();
      if (f) files.push(f);
    }
  }
  return files;
}

function onPaste(e: ClipboardEvent, target: 'verify' | 'negative') {
  const files = imagesFromClipboard(e.clipboardData?.items);
  if (!files.length) return;
  e.preventDefault();
  for (const f of files) {
    if (target === 'verify') stageVerifyFile(f);
    else stageNegFile(f);
  }
}

function onDrop(e: DragEvent, target: 'verify' | 'negative') {
  e.preventDefault();
  for (const f of e.dataTransfer?.files || []) {
    if (f.type.startsWith('image/')) {
      if (target === 'verify') stageVerifyFile(f);
      else stageNegFile(f);
    }
  }
}

// ---------- Toast ----------
const toasts = ref<Toast[]>([]);
let toastId = 0;
function showToast(message: string, type: 'error' | 'success' = 'success') {
  const id = ++toastId;
  toasts.value.push({ id, message, type });
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }, 3000);
}

// ---------- Tabs keyboard ----------
const tabRefs = ref<HTMLButtonElement[]>([]);
function onTabKey(e: KeyboardEvent) {
  const tabs: Array<'positive' | 'negative' | 'incomplete'> = ['positive', 'negative', 'incomplete'];
  const idx = tabs.indexOf(tab.value);
  if (e.key === 'ArrowRight') {
    e.preventDefault();
    const next = tabs[(idx + 1) % tabs.length];
    tab.value = next;
    nextTick(() => tabRefs.value[tabs.indexOf(next)]?.focus());
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault();
    const prev = tabs[(idx - 1 + tabs.length) % tabs.length];
    tab.value = prev;
    nextTick(() => tabRefs.value[tabs.indexOf(prev)]?.focus());
  }
}

// ---------- Hash-based queue tab ----------
const queueNameFromHash = (hash: string) => hash.replace('#', '');
const validTabs = new Set(['positive', 'negative', 'incomplete']);
watch(tab, (t) => {
  history.replaceState(null, '', `#${t}`);
});
if (typeof window !== 'undefined') {
  const initial = queueNameFromHash(window.location.hash);
  if (validTabs.has(initial)) tab.value = initial as any;
}

function onFileChange(target: 'verify' | 'negative', files: FileList | null) {
  if (!files) return;
  for (const f of files) {
    if (target === 'verify') stageVerifyFile(f);
    else stageNegFile(f);
  }
}
</script>

<template>
  <div class="workbench">
    <!-- Queue tabs -->
    <div class="workbench-queue-tabs" role="tablist" aria-label="验证队列">
      <button
        v-for="(t, i) in [
          { key: 'positive', label: '待确认漏洞', count: counts?.positive ?? 0 },
          { key: 'negative', label: '待负向验证', count: counts?.negative ?? 0 },
          { key: 'incomplete', label: '检查未完成', count: counts?.incomplete ?? 0 },
        ]"
        :key="t.key"
        ref="tabRefs"
        type="button"
        class="queue-tab"
        :class="{ active: tab === t.key }"
        role="tab"
        :aria-selected="tab === t.key"
        :aria-controls="`queue-${t.key}`"
        :id="`tab-${t.key}`"
        @click="tab = t.key as any"
        @keydown="onTabKey"
      >
        {{ t.label }} <span class="queue-count">{{ t.count }}</span>
      </button>
    </div>

    <!-- Positive queue -->
    <div v-show="tab === 'positive'" :id="'queue-positive'" role="tabpanel" aria-labelledby="tab-positive">
      <section class="panel workbench-filter-panel">
        <div class="workbench-filter-form">
          <label>
            <span>分区</span>
            <select v-model="zoneFilter">
              <option value="">全部</option>
              <option v-for="z in zones" :key="z.ZoneID" :value="z.ZoneID">{{ z.Name }}</option>
            </select>
          </label>
          <label>
            <span>状态</span>
            <select v-model="statusFilter">
              <option value="">全部</option>
              <option value="pending">待确认</option>
              <option value="confirmed">已确认</option>
              <option value="included">已纳入</option>
            </select>
          </label>
          <label class="filter-keyword">
            <span>关键词</span>
            <input v-model="keywordFilter" type="text" placeholder="漏洞名称 / 资产" />
          </label>
          <button type="button" class="button button-secondary filter-more-btn" @click="moreOpen = !moreOpen" :aria-expanded="moreOpen">
            {{ moreOpen ? '收起筛选' : '更多筛选' }}
            <span v-if="activeFilterCount" class="filter-active-count">{{ activeFilterCount }}</span>
          </button>
          <button type="button" class="button button-secondary" @click="resetFilters">重置</button>
        </div>
        <div v-show="moreOpen" class="filter-advanced-panel">
          <div class="workbench-filter-form">
            <label>
              <span>等级</span>
              <span class="checkbox-row severity-row">
                <label v-for="sev in ['critical','high','medium','low']" :key="sev" :class="`severity-filter-chip ${sev}`">
                  <input type="checkbox" :value="sev" :checked="severityFilter.includes(sev)" @change="toggleSeverity(sev)">
                  <span class="severity-dot" :class="sev"></span>{{ sev }}
                </label>
              </span>
            </label>
            <label>
              <span>服务</span>
              <select v-model="serviceFilter">
                <option value="">全部</option>
                <option v-for="s in services" :key="s" :value="s">{{ s }}</option>
              </select>
            </label>
            <label>
              <span>端口</span>
              <input v-model="portFilter" type="text" placeholder="80" />
            </label>
            <label>
              <span>Run</span>
              <select v-model="runFilter">
                <option value="">全部</option>
                <option v-for="r in runs" :key="r" :value="r">{{ r }}</option>
              </select>
            </label>
          </div>
        </div>
      </section>

      <section class="panel workbench-candidates">
        <article v-for="c in filteredCandidates" :key="c.GroupKey" class="candidate-card" :data-key="c.GroupKey">
          <div class="candidate-header">
            <h3>{{ c.Title }} <span class="severity-badge" :class="`sev-${c.Severity}`">{{ c.Severity }}</span></h3>
            <div class="candidate-meta">
              <span>分区：{{ zone_names[c.ZoneID] || c.ZoneID }}</span>
              <span>资产：{{ c.Assets?.length ?? 0 }}</span>
              <span>Runs：{{ c.SourceRuns?.length ?? 0 }}</span>
              <span>服务：{{ c.Services?.length ? c.Services.join(' ') : '—' }}</span>
              <span class="candidate-status">状态：{{ candidateStatusLabel(candidateStatus(c)) }}</span>
            </div>
          </div>
          <div class="candidate-body">
            <p class="candidate-description">{{ c.Description }}</p>
            <details class="asset-details">
              <summary>受影响资产 ({{ c.Assets?.length ?? 0 }})</summary>
              <ul class="asset-list">
                <li v-for="a in c.Assets" :key="`${a.IP}:${a.Port}`" class="asset-item">
                  <code>{{ hostPort(a) }}</code>
                  <a v-if="a.Target" :href="a.Target" target="_blank" class="asset-link">{{ a.Target }}</a>
                  <button type="button" class="button button-small" @click="copyText(hostPort(a), 'IP:PORT')">复制 IP:PORT</button>
                  <button v-if="!c.IsPending" type="button" class="button button-small" @click="runCommand(c.GroupKey, 'nuclei', hostPort(a))">Nuclei</button>
                  <button v-if="!c.IsPending" type="button" class="button button-small" @click="runCommand(c.GroupKey, 'nmap', hostPort(a))">Nmap</button>
                </li>
              </ul>
            </details>
            <div class="candidate-actions">
              <button type="button" class="button button-secondary" @click="openVerifyDialog(c.GroupKey)">验证 / 编辑</button>
              <details class="context-actions">
                <summary type="button" class="button button-secondary">更多操作</summary>
                <div class="context-menu">
                  <button v-if="!c.IsPending" type="button" class="button button-small" @click="runCommand(c.GroupKey, 'nuclei', 'all')">Nuclei 命令</button>
                  <button v-if="!c.IsPending" type="button" class="button button-small" @click="runCommand(c.GroupKey, 'nmap', 'all')">Nmap 命令</button>
                  <button v-if="!c.IsPending" type="button" class="button button-small" @click="runCommand(c.GroupKey, 'msf', 'all')">MSF 命令</button>
                  <button type="button" class="button button-small" @click="copyText(c.Title, '标题')">复制标题</button>
                  <button type="button" class="button button-small" @click="copyCard(c)">复制整条</button>
                </div>
              </details>
            </div>
          </div>
        </article>
        <p v-if="!filteredCandidates.length" class="no-data">没有符合筛选条件的正向漏洞候选。</p>
      </section>
    </div>

    <!-- Negative queue -->
    <div v-show="tab === 'negative'" id="queue-negative" role="tabpanel" aria-labelledby="tab-negative">
      <section class="panel">
        <p class="meta-line">以下按服务指纹分组。同分区、同服务/产品的不同 IP 和端口合并为一组；点击该组的“提交负向验证”后上传截图。</p>
      </section>
      <section class="panel workbench-candidates">
        <article v-for="g in filteredNegativeGroups" :key="g.Key" class="candidate-card negative-card" :data-key="g.Key">
          <div class="candidate-header">
            <h3 style="font-size: 0.95rem">{{ g.Title }}相关漏洞不存在证明，端口（{{ g.PortsText || '—' }}）</h3>
            <div class="candidate-meta">
              <span>分区：{{ zone_names[g.ZoneID] || g.ZoneID }}</span>
              <span>端点：{{ g.Assets?.length ?? 0 }}</span>
            </div>
          </div>
          <details class="asset-details">
            <summary>覆盖端点 ({{ g.Assets?.length ?? 0 }})</summary>
            <ul class="asset-list">
              <li v-for="a in g.Assets" :key="`${a.IP}:${a.Port}`" class="asset-item"><code>{{ hostPort(a) }}</code></li>
            </ul>
          </details>
          <div class="candidate-actions">
            <button v-if="g.NmapCommand" type="button" class="button button-small" @click="copyText(g.NmapCommand, 'Nmap 命令')">复制 Nmap</button>
            <button v-if="g.NucleiCommand" type="button" class="button button-small" @click="copyText(g.NucleiCommand, 'Nuclei 命令')">复制 Nuclei</button>
            <button type="button" class="button button-primary" @click="openNegativeDialog(g)">提交负向验证 / 粘贴截图</button>
          </div>
        </article>
        <p v-if="!filteredNegativeGroups.length" class="no-data">当前没有满足双引擎已完成且无非 info 发现的待负向验证指纹组。</p>
      </section>
    </div>

    <!-- Incomplete queue -->
    <div v-show="tab === 'incomplete'" id="queue-incomplete" role="tabpanel" aria-labelledby="tab-incomplete">
      <section class="panel">
        <p class="meta-line">以下端点存在 DetectionCheck 缺失、失败、非“规则不适用”的跳过、取消或中断，无法作为负向候选。</p>
      </section>
      <section class="panel workbench-candidates">
        <article v-for="ic in filteredIncomplete" :key="`${ic.Asset.IP}:${ic.Asset.Port}:${ic.RunID}`" class="candidate-card incomplete-card">
          <div class="candidate-header">
            <h3 style="font-size: 0.95rem">
              <code>{{ hostPort(ic.Asset) }}</code>
              <span v-if="ic.Fingerprint.Service" class="meta-line" style="margin-left: 0.5rem">{{ ic.Fingerprint.Service }}{{ ic.Fingerprint.Product ? ' / ' + ic.Fingerprint.Product : '' }}</span>
            </h3>
            <div class="candidate-meta">
              <span>分区：{{ zone_names[ic.ZoneID] || ic.ZoneID }}</span>
              <span>协议：{{ ic.Asset.Protocol }}</span>
              <span>Run：{{ ic.RunID }}</span>
            </div>
          </div>
          <ul class="asset-list" style="margin-top: 0.5rem">
            <li v-for="e in ic.Engines" :key="e.Engine" class="asset-item">
              <code>{{ e.Engine }}</code>
              <span class="severity-badge" :class="`sev-${e.Status === 'failed' ? 'high' : e.Status === 'missing' ? 'medium' : 'low'}`">{{ e.Status }}</span>
              <span v-if="e.ReasonCode" class="meta-line">{{ e.ReasonCode }}</span>
              <span v-if="e.Detail" class="meta-line" :title="e.Detail">详情…</span>
            </li>
          </ul>
        </article>
        <p v-if="!filteredIncomplete.length" class="no-data">当前没有检查未完成的端点。</p>
      </section>
    </div>

    <!-- Command dialog -->
    <dialog ref="commandDialog" class="panel" aria-labelledby="command-dialog-title" @close="commandBody = ''">
      <div class="panel-heading">
        <h3 id="command-dialog-title">{{ commandTitle }}</h3>
        <button type="button" class="button button-secondary" @click="commandDialog?.close()">关闭</button>
      </div>
      <p class="meta-line" v-if="commandLoading">正在生成，不会启动扫描。</p>
      <p class="meta-line" v-else-if="commandWarning">{{ commandWarning }}</p>
      <pre class="command-pre">{{ commandBody || (commandLoading ? '生成中…' : '') }}</pre>
      <div class="header-actions">
        <button type="button" class="button button-secondary" @click="copyCommandText" :disabled="!commandBody">复制完整命令</button>
        <a v-if="commandToolLink" class="button button-primary" :href="commandToolLink">带参数打开工具页</a>
      </div>
    </dialog>

    <!-- Verify dialog -->
    <dialog ref="verifyDialog" class="panel verify-dialog" aria-labelledby="verify-dialog-title" @paste="(e) => onPaste(e, 'verify')" @dragover.prevent @drop="(e) => onDrop(e, 'verify')">
      <form class="form-grid" @submit.prevent="saveVerification">
        <div class="full-width panel-heading" style="margin-bottom: 0; padding-bottom: 0.75rem; border-bottom: 1px solid var(--border);">
          <h3 id="verify-dialog-title">验证 / 编辑</h3>
          <button type="button" class="button button-secondary" @click="verifyDialog?.close()">关闭</button>
        </div>
        <input type="hidden" :value="verifyKey" />
        <input type="hidden" :value="verifyZoneId" />
        <input type="hidden" :value="verifyId" />
        <label class="full-width">
          <span>标题</span>
          <input v-model="verifyTitle" name="title" type="text" required />
        </label>
        <label>
          <span>危险等级</span>
          <select v-model="verifySeverity">
            <option value="critical">critical</option>
            <option value="high">high</option>
            <option value="medium">medium</option>
            <option value="low">low</option>
          </select>
        </label>
        <label>
          <span>结论</span>
          <select v-model="verifyOutcome">
            <option value="confirmed">confirmed（已确认）</option>
            <option value="inconclusive">inconclusive（无法判定）</option>
            <option value="not_observed">not_observed（本次未发现）</option>
          </select>
        </label>
        <label class="full-width">
          <span>漏洞描述</span>
          <textarea v-model="verifyDescription" rows="4"></textarea>
        </label>
        <label class="full-width">
          <span>修复建议</span>
          <textarea v-model="verifyRemediation" rows="4"></textarea>
        </label>
        <div class="full-width">
          <p class="eyebrow">受影响资产</p>
          <ul class="verify-asset-list">
            <li v-for="a in activeCandidate?.Assets" :key="`${a.IP}:${a.Port}`"><code>{{ hostPort(a) }}</code><a v-if="a.Target" :href="a.Target" target="_blank">{{ a.Target }}</a></li>
          </ul>
        </div>
        <div class="full-width">
          <p class="eyebrow">证据截图</p>
          <p class="meta-line">confirmed / not_observed 验证必须至少上传一张截图。</p>
          <label class="file-input-label">
            <input type="file" accept="image/png,image/jpeg" multiple @change="(e) => onFileChange('verify', (e.target as HTMLInputElement).files)">
            <span>选择一张或多张 PNG/JPEG 截图</span>
          </label>
          <div class="paste-hint" tabindex="0" @click="verifyDialog?.querySelector<HTMLInputElement>('input[type=file]')?.click()">点击这里后按 Ctrl/Cmd+V 粘贴截图；也可直接拖入本弹窗</div>
          <ul class="evidence-list">
            <li v-for="(f, i) in verifyPendingFiles" :key="f.objectUrl" class="evidence-item">
              <img :src="f.objectUrl" alt="" />
              <span>{{ f.caption || '待上传' }}</span>
              <button type="button" class="button button-small" @click="removeVerifyPending(i)">删除</button>
            </li>
            <li v-for="e in verifyCurrent?.Evidence" :key="e.ID" class="evidence-item">
              <img :src="`/projects/${project_id}/verifications/${verifyId}/evidence/${e.ID}`" alt="" loading="lazy" />
              <span>{{ e.Caption || '无说明' }}</span>
              <button type="button" class="button button-small" @click="deleteEvidence(verifyId, e.ID)">删除</button>
            </li>
          </ul>
        </div>
        <div class="form-actions full-width">
          <button type="button" class="button button-secondary" @click="verifyDialog?.close()">取消</button>
          <button type="submit" class="button button-primary" :disabled="verifySaving">{{ verifySaving ? '保存中…' : '保存验证' }}</button>
        </div>
      </form>
    </dialog>

    <!-- Negative dialog -->
    <dialog ref="negativeDialog" class="panel negative-dialog" aria-labelledby="negative-dialog-title" @paste="(e) => onPaste(e, 'negative')" @dragover.prevent @drop="(e) => onDrop(e, 'negative')">
      <form class="form-grid" @submit.prevent="saveNegative">
        <div class="full-width panel-heading" style="margin-bottom: 0; padding-bottom: 0.75rem; border-bottom: 1px solid var(--border);">
          <h3 id="negative-dialog-title">提交负向验证</h3>
          <button type="button" class="button button-secondary" @click="negativeDialog?.close()">关闭</button>
        </div>
        <div class="full-width">
          <p class="meta-line">为以下已选端点提交“本次验证未发现”结论。请填写验证项名称并上传共享截图后提交。</p>
        </div>
        <input type="hidden" :value="negZoneId" />
        <label class="full-width">
          <span>验证项名称 <small class="meta-line">（说明本次验证的漏洞或检查类型，例如“未检出弱口令”）</small></span>
          <input v-model="negTitle" name="negative-title" type="text" required placeholder="例：ssh / OpenSSH" />
        </label>
        <label>
          <span>危险等级</span>
          <select v-model="negSeverity">
            <option value="high">high</option>
            <option value="medium">medium</option>
            <option value="low">low</option>
            <option value="critical">critical</option>
          </select>
        </label>
        <label class="full-width">
          <span>验证过程 / 结果说明</span>
          <textarea v-model="negDescription" rows="3" placeholder="本次验证执行了…，结论：本次验证未发现该漏洞"></textarea>
        </label>
        <div class="full-width">
          <p class="eyebrow">已选端点</p>
          <ul class="verify-asset-list">
            <li v-for="a in negGroup?.Assets" :key="`${a.IP}:${a.Port}`"><code>{{ hostPort(a) }}</code></li>
          </ul>
        </div>
        <div class="full-width">
          <p class="eyebrow">共享截图（必须至少一张，可多张）</p>
          <p class="meta-line">截图将进入正式报告。可以点击选择，也可以把图片拖到本弹窗内，或在下方区域按 Ctrl/Cmd+V 粘贴。</p>
          <label class="file-input-label">
            <input type="file" accept="image/png,image/jpeg" multiple @change="(e) => onFileChange('negative', (e.target as HTMLInputElement).files)">
            <span>选择一张或多张 PNG/JPEG 截图</span>
          </label>
          <div class="paste-hint" tabindex="0">点击这里后按 Ctrl/Cmd+V 粘贴截图；也可直接拖入本弹窗</div>
          <ul class="evidence-list">
            <li v-for="(f, i) in negPendingFiles" :key="f.objectUrl" class="evidence-item">
              <img :src="f.objectUrl" alt="" />
              <span>{{ f.caption || '无说明' }}</span>
              <button type="button" class="button button-small" @click="removeNegPending(i)">删除</button>
            </li>
          </ul>
        </div>
        <div class="form-actions full-width">
          <button type="button" class="button button-secondary" @click="negativeDialog?.close()">取消</button>
          <button type="submit" class="button button-primary" :disabled="negSaving">{{ negSaving ? '提交中…' : '提交本次验证未发现' }}</button>
        </div>
      </form>
    </dialog>

    <!-- Toasts -->
    <div class="toast-container" aria-live="polite" aria-atomic="true">
      <div v-for="t in toasts" :key="t.id" class="toast" :class="`toast-${t.type}`" role="status">{{ t.message }}</div>
    </div>
  </div>
</template>

<style scoped>
.workbench { display: flex; flex-direction: column; gap: 1rem; }
.workbench-queue-tabs { display: flex; gap: 0.5rem; margin-bottom: 0; }
.queue-tab { display: inline-flex; align-items: center; gap: 0.4rem; padding: 0.5rem 1rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel); color: var(--muted); font-weight: 500; cursor: pointer; transition: background 0.15s ease, color 0.15s ease; }
.queue-tab:hover, .queue-tab.active { background: var(--primary-soft); color: var(--primary); border-color: var(--primary); }
.queue-count { display: inline-block; min-width: 1.4em; padding: 0.1em 0.4em; font-size: 0.78rem; border-radius: 999px; background: var(--bg-accent); color: var(--muted); text-align: center; }
.queue-tab.active .queue-count { background: var(--primary); color: #fff; }
.workbench-filter-panel { margin-bottom: 0; }
.workbench-filter-form { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 0.75rem 1rem; align-items: end; }
.workbench-filter-form label { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.85rem; color: var(--muted); }
.workbench-filter-form label.filter-keyword { grid-column: span 2; }
.filter-keyword input { width: 100%; }
.filter-more-btn { position: relative; }
.filter-active-count { position: absolute; top: -0.35rem; right: -0.35rem; background: var(--primary); color: #fff; font-size: 0.65rem; font-weight: 700; padding: 0.05rem 0.35rem; border-radius: 999px; }
.severity-row { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.severity-row label { flex-direction: row; align-items: center; font-size: 0.78rem; }
.filter-advanced-panel { margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px dashed var(--border); }
.candidate-card { border: 1px solid var(--border); border-radius: var(--radius-md); padding: 1rem; background: var(--panel); margin-bottom: 1rem; }
.candidate-header { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 0.75rem; }
.candidate-header h3 { font-size: 1.05rem; display: flex; align-items: center; gap: 0.5rem; margin: 0; }
.candidate-meta { display: flex; flex-wrap: wrap; gap: 0.75rem; font-size: 0.8rem; color: var(--muted); }
.candidate-description { color: var(--text); font-size: 0.9rem; margin-bottom: 0.75rem; }
.candidate-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.75rem; align-items: center; }
.context-actions { position: relative; display: inline-block; }
.context-actions > summary { list-style: none; display: inline-flex; }
.context-actions > summary::-webkit-details-marker { display: none; }
.context-menu { position: absolute; top: calc(100% + 0.35rem); right: 0; min-width: 10rem; background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-md); box-shadow: var(--shadow); padding: 0.5rem; display: flex; flex-direction: column; gap: 0.35rem; z-index: 50; }
.context-menu .button { justify-content: flex-start; }
.asset-details { margin: 0.75rem 0; padding: 0.75rem; background: var(--bg-accent); border-radius: var(--radius-sm); }
.asset-list { list-style: none; padding: 0; margin: 0.5rem 0 0; display: flex; flex-direction: column; gap: 0.35rem; }
.asset-item { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem; font-size: 0.85rem; }
.verify-asset-list { list-style: none; padding: 0; margin: 0.5rem 0 0; display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.85rem; }
.evidence-list { list-style: none; padding: 0; margin: 0.75rem 0 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 0.75rem; }
.evidence-item { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.8rem; }
.evidence-item img { width: 100%; height: 90px; object-fit: cover; border-radius: var(--radius-sm); border: 1px solid var(--border); }
.paste-hint { padding: 0.75rem; border: 2px dashed var(--border-strong); border-radius: var(--radius-sm); text-align: center; color: var(--muted); font-size: 0.85rem; margin-top: 0.5rem; cursor: pointer; outline: none; }
.paste-hint:focus { border-color: var(--primary); }
.file-input-label input { display: none; }
.file-input-label span { display: inline-block; padding: 0.5rem 0.75rem; background: var(--primary-soft); color: var(--primary); border-radius: var(--radius-sm); cursor: pointer; font-size: 0.85rem; }
.command-pre { white-space: pre-wrap; background: var(--code-bg); padding: 0.75rem; border-radius: var(--radius-sm); overflow: auto; max-height: 40vh; font-family: var(--mono); font-size: 0.85rem; }
.toast-container { position: fixed; bottom: 1.25rem; right: 1.25rem; display: flex; flex-direction: column; gap: 0.5rem; z-index: 200; }
.toast { padding: 0.65rem 1rem; border-radius: var(--radius-md); font-size: 0.85rem; font-weight: 600; box-shadow: var(--shadow); }
.toast-success { background: var(--success-soft); color: #167546; border: 1px solid var(--success-border); }
.toast-error { background: var(--danger-soft); color: #b42318; border: 1px solid var(--danger-border); }
</style>
