<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ansiToHtml } from './ansi';

type ScanEvent = { id: number; time: string; level: string; stage: string; message: string };

// This terminal intentionally does NOT share the run-monitor page's event-log
// rendering: the monitor shows the processed pipeline ("[time] [level] stage:
// message"), while this page mirrors an external terminal — the echoed command
// (level "command") plus the complete raw tool output (level "raw") with ANSI
// colors rendered as-is.
const output = ref('等待启动工具…');
const events = ref<ScanEvent[]>([]);
const runID = ref('');
const status = ref('');
const busy = ref(false);
const canceling = ref(false);
const feedback = ref('');
const followingOutput = ref(true);
const terminal = ref<HTMLElement>();
let form: HTMLFormElement | null = null;
let stopped = false;
const eventPageSize = 1000;

// Keep the end-of-run wording aligned with the run monitor (RunDetail.vue) so
// both surfaces report the same lifecycle semantics.
const lifecycleText = computed(() => {
  if (status.value === 'interrupted') return '工具运行已中断，可在任务控制台查看已保留的输出。';
  return ({
    running: '工具已启动，正在接收输出。',
    canceled: '工具运行已取消。',
    completed: '工具运行已完成。',
    completed_with_errors: '工具运行已完成，但部分检查发生错误。',
    failed: '工具运行失败，请查看输出定位原因。',
  }[status.value] || '');
});
const latestStage = computed(() => {
  const event = [...events.value].reverse().find(({ level }) => level !== 'command' && level !== 'raw');
  return event?.stage || '';
});

function onScroll() {
  const box = terminal.value;
  if (box) followingOutput.value = box.scrollHeight - box.scrollTop - box.clientHeight < 24;
}

async function poll(run: string) {
  while (!stopped) {
    const afterID = events.value.at(-1)?.id ?? 0;
    const [eventsResult, statusResult] = await Promise.all([
      fetch(`/api/runs/${encodeURIComponent(run)}/events?after_id=${afterID}`),
      fetch(`/api/runs/${encodeURIComponent(run)}/status`),
    ]);
    if (!eventsResult.ok || !statusResult.ok) throw new Error('poll failed');
    const newEvents = await eventsResult.json() as ScanEvent[];
    if (newEvents.length > 0) events.value = [...events.value, ...newEvents];
    const rendered = renderTerminal(events.value);
    output.value = rendered || '工具已启动，等待输出…';
    const statusValue = (await statusResult.json() as { status: string }).status;
    status.value = statusValue;
    const hasMoreEvents = newEvents.length === eventPageSize;
    if (statusValue !== 'running' && !hasMoreEvents) {
      feedback.value = lifecycleText.value;
      return;
    }
    if (statusValue === 'running') await new Promise((resolve) => window.setTimeout(resolve, 1000));
  }
}

function renderTerminal(list: ScanEvent[]): string {
  return list
    .filter((event) => event.level === 'command' || event.level === 'raw')
    .map((event) => (event.level === 'command' ? `$ ${event.message}` : event.message))
    .join('\n');
}

async function submit(event: SubmitEvent) {
  event.preventDefault();
  if (!form || busy.value) return;
  if (!form.checkValidity()) {
    form.reportValidity();
    return;
  }
  busy.value = true;
  feedback.value = '正在启动工具…';
  output.value = '正在创建工具运行…';
  const button = form.querySelector<HTMLButtonElement>('button[type="submit"]');
  if (button) button.disabled = true;
  try {
    const body = new URLSearchParams();
    new FormData(form).forEach((value, key) => body.append(key, String(value)));
    const response = await fetch(form.action, { method: 'POST', body, headers: { 'X-Requested-With': 'fetch' } });
    if (!response.ok) throw new Error('start failed');
    runID.value = (await response.json() as { run_id: string }).run_id;
    feedback.value = '工具已启动，正在接收输出。';
    await poll(runID.value);
  } catch {
    feedback.value = '工具未能启动，请检查参数后重试。';
    output.value = '未收到工具输出。';
  } finally {
    busy.value = false;
    if (button) button.disabled = false;
  }
}

async function cancelRun() {
  if (!runID.value || canceling.value) return;
  canceling.value = true;
  feedback.value = '正在请求中止工具…';
  try {
    const response = await fetch(`/runs/${encodeURIComponent(runID.value)}/cancel`, { method: 'POST' });
    if (!response.ok) throw new Error('cancel failed');
    feedback.value = '已请求中止，正在等待工具停止。';
  } catch {
    feedback.value = '中止请求未成功，请稍后重试。';
  } finally {
    canceling.value = false;
  }
}

const outputHtml = computed(() => ansiToHtml(output.value));

watch(output, async () => {
  await nextTick();
  const selection = window.getSelection();
  const selectingOutput = selection?.rangeCount && terminal.value && (terminal.value.contains(selection.anchorNode) || terminal.value.contains(selection.focusNode));
  if (followingOutput.value && !selectingOutput && terminal.value) terminal.value.scrollTop = terminal.value.scrollHeight;
});

onMounted(() => {
  form = document.querySelector<HTMLFormElement>('[data-tool-form]');
  form?.addEventListener('submit', submit);
});

onBeforeUnmount(() => {
  stopped = true;
  form?.removeEventListener('submit', submit);
});
</script>

<template>
  <section class="panel tool-output-panel" :aria-busy="busy">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">工具输出</p>
        <h3>工具实时输出</h3>
        <p class="meta-line" role="status">{{ feedback || lifecycleText }}{{ latestStage && status === 'running' ? ` · 当前阶段：${latestStage}` : '' }}</p>
      </div>
      <div class="header-actions">
        <button v-if="runID && status === 'running'" class="button button-danger" type="button" :disabled="canceling" @click="cancelRun">{{ canceling ? '正在中止…' : '中止工具' }}</button>
        <a v-if="runID" class="button button-secondary" :href="`/runs/${runID}`">查看本次完整结果</a>
      </div>
    </div>
    <div class="terminal-window tool-preview-window">
      <div class="terminal-header"><div class="terminal-dots"><span class="terminal-dot dot-red"></span><span class="terminal-dot dot-yellow"></span><span class="terminal-dot dot-green"></span></div><div class="terminal-title">tool output</div></div>
      <pre ref="terminal" class="event-log tool-command-preview" @scroll="onScroll" v-html="outputHtml"></pre>
    </div>
  </section>
</template>
