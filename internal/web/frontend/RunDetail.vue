<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';

type ScanEvent = { id: number; time: string; level: string; stage: string; message: string };
type StatusResponse = { status: string; detection_checks: Record<string, number> };

const props = defineProps<{
  run_id: string;
  project_id: string;
  status: string;
  target: string;
  ports: string;
  profile: string;
  full_target: string;
  full_ports: string;
  full_profile: string;
  can_rerun: boolean;
  is_tool_run: boolean;
  return_url: string;
  evidence_url: string;
}>();

const status = ref(props.status);
const checks = ref<Record<string, number>>({});
const events = ref<ScanEvent[]>([]);
const canceling = ref(false);
const copying = ref(false);
const feedback = ref('');
const metaOpen = ref(false);
const followingOutput = ref(true);
const eventLog = ref<HTMLElement>();
let timer = 0;

const active = computed(() => status.value === 'running');
const latestEvent = computed(() => events.value.at(-1));
const output = computed(() => events.value.map((event) => `[${event.time}] [${event.level || 'info'}] ${event.stage || 'engine'}: ${event.message}`).join('\n'));
const progress = computed(() => {
  const event = [...events.value].reverse().find(({ stage }) => stage.toLowerCase() === 'progress');
  const match = event?.message.match(/(\d+)\s*\/\s*(\d+)/);
  if (!match || Number(match[2]) === 0) return null;
  const done = Number(match[1]);
  const total = Number(match[2]);
  return { done, total, percent: Math.min(100, Math.round((done / total) * 100)), detail: event?.message || '' };
});

const lifecycleText = computed(() => ({
  running: '扫描正在运行，页面会在后台更新。',
  canceled: '扫描已取消。',
  interrupted: '扫描已中断，可确认参数后重新运行。',
  completed: '扫描已完成。',
  completed_with_errors: '扫描已完成，但部分检查发生错误。',
  failed: '扫描失败，请查看最新事件。',
}[status.value] || `当前状态：${status.value}`));

function runURL(suffix: string) {
  return `/api/runs/${encodeURIComponent(props.run_id)}${suffix}`;
}

async function refresh() {
  const [statusResult, eventResult] = await Promise.all([fetch(runURL('/status')), fetch(runURL('/events'))]);
  if (!statusResult.ok || !eventResult.ok) throw new Error('refresh failed');
  const statusData = await statusResult.json() as StatusResponse;
  status.value = statusData.status;
  checks.value = statusData.detection_checks || {};
  events.value = await eventResult.json() as ScanEvent[];
  if (!active.value) window.clearInterval(timer);
}

async function cancelRun() {
  canceling.value = true;
  feedback.value = '正在请求中止扫描…';
  try {
    const response = await fetch(`/runs/${encodeURIComponent(props.run_id)}/cancel`, { method: 'POST' });
    if (!response.ok) throw new Error('cancel failed');
    feedback.value = '已请求中止，正在等待引擎停止。';
    await refresh();
  } catch {
    feedback.value = '中止请求未成功，请稍后重试。';
  } finally {
    canceling.value = false;
  }
}

async function copyOutput() {
  copying.value = true;
  try {
    await navigator.clipboard.writeText(output.value);
    feedback.value = '已复制输出。';
  } catch {
    feedback.value = '复制失败，请手动选择输出内容。';
  } finally {
    copying.value = false;
  }
}

function onOutputScroll() {
  const box = eventLog.value;
  if (box) followingOutput.value = box.scrollHeight - box.scrollTop - box.clientHeight < 24;
}

watch(events, async () => {
  await nextTick();
  const selection = window.getSelection();
  const selectingOutput = selection?.rangeCount && eventLog.value?.contains(selection.anchorNode);
  if (followingOutput.value && !selectingOutput && eventLog.value) eventLog.value.scrollTop = eventLog.value.scrollHeight;
});

onMounted(async () => {
  try {
    await refresh();
  } catch {
    feedback.value = '暂时无法更新运行状态。';
  }
  if (active.value) timer = window.setInterval(() => { void refresh().catch(() => { feedback.value = '暂时无法更新运行状态。'; }); }, 1200);
});

onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <section class="page-header">
    <div>
      <p class="eyebrow">任务控制台</p>
      <h2 class="mono-heading run-title">{{ run_id }}</h2>
      <details class="run-meta-details" :open="metaOpen" @toggle="metaOpen = ($event.currentTarget as HTMLDetailsElement).open">
        <summary>
          <span>扫描参数</span>
          <span class="mono-value">目标: {{ target }} | 端口: {{ ports }} | 档位: {{ profile }}</span>
          <span class="meta-expand-text">展开全部扫描参数</span>
        </summary>
        <dl class="run-meta-full">
          <dt>目标</dt><dd class="mono-value">{{ full_target }}</dd>
          <dt>端口</dt><dd class="mono-value">{{ full_ports }}</dd>
          <dt>档位</dt><dd class="mono-value">{{ full_profile }}</dd>
        </dl>
      </details>
    </div>
    <div class="header-actions">
      <span :class="['status-badge', `status-${status}`, 'run-status-badge']">{{ status }}</span>
      <button v-if="active" class="button button-danger" type="button" :disabled="canceling" @click="cancelRun">{{ canceling ? '正在中止…' : '中止扫描' }}</button>
      <template v-else>
        <a v-if="is_tool_run && return_url" class="button button-secondary" :href="return_url">返回工作台</a>
        <a v-if="is_tool_run && evidence_url" class="button button-primary" :href="evidence_url">上传证据</a>
        <a class="button button-primary run-report-button" :href="`/reports/${run_id}`">查看扫描报告</a>
        <a v-if="can_rerun" class="button button-secondary" :href="`/projects/${project_id}/scans/new?rerun=${run_id}`">确认并重新运行</a>
        <button class="button button-secondary" type="button" :disabled="copying" @click="copyOutput">{{ copying ? '正在复制…' : '复制输出' }}</button>
      </template>
    </div>
  </section>

  <section class="panel run-monitor-panel">
    <div class="panel-heading run-monitor-heading"><div><p class="eyebrow">进程输出</p><h3 class="run-monitor-title">引擎级联流水线实时监控</h3></div></div>
    <p class="meta-line" role="status">{{ feedback || lifecycleText }}</p>
    <div class="scan-progress">
      <div class="scan-progress-header"><span class="scan-progress-title">主机扫描进度</span><span class="scan-progress-count">{{ progress ? `${progress.done} / ${progress.total} (${progress.percent}%)` : (latestEvent ? `当前阶段：${latestEvent.stage || 'engine'}` : '等待运行事件…') }}</span></div>
      <div v-if="progress" class="scan-progress-bar-wrap"><div class="scan-progress-bar" :style="{ width: `${progress.percent}%` }"></div></div>
      <div v-if="progress" class="scan-progress-detail">{{ progress.detail }}</div>
    </div>
    <p class="meta-line">检测检查：已完成 {{ checks.completed || 0 }}，运行中 {{ checks.running || 0 }}，失败 {{ checks.failed || 0 }}，跳过 {{ checks.skipped || 0 }}，已取消 {{ checks.canceled || 0 }}</p>
    <div class="terminal-window">
      <div class="terminal-header"><div class="terminal-dots"><span class="terminal-dot dot-red"></span><span class="terminal-dot dot-yellow"></span><span class="terminal-dot dot-green"></span></div><div class="terminal-title">anchorscan@engine: ~</div></div>
      <pre ref="eventLog" class="event-log" @scroll="onOutputScroll">{{ output || '等待运行事件…' }}</pre>
    </div>
  </section>
</template>
