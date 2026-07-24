<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';

type ScanEvent = { time: string; level: string; stage: string; message: string };

const output = ref('等待启动工具…');
const runID = ref('');
const busy = ref(false);
const feedback = ref('');
const followingOutput = ref(true);
const terminal = ref<HTMLElement>();
let form: HTMLFormElement | null = null;
let stopped = false;

function onScroll() {
  const box = terminal.value;
  if (box) followingOutput.value = box.scrollHeight - box.scrollTop - box.clientHeight < 24;
}

async function poll(run: string) {
  while (!stopped) {
    const [eventsResult, statusResult] = await Promise.all([
      fetch(`/api/runs/${encodeURIComponent(run)}/events`),
      fetch(`/api/runs/${encodeURIComponent(run)}/status`),
    ]);
    if (!eventsResult.ok || !statusResult.ok) throw new Error('poll failed');
    const events = await eventsResult.json() as ScanEvent[];
    output.value = events.map((event) => `[${event.time}] [${event.level || 'info'}] ${event.stage || 'tool'}: ${event.message}`).join('\n') || '工具已启动，等待输出…';
    const status = (await statusResult.json() as { status: string }).status;
    if (status !== 'running') {
      feedback.value = status === 'completed' ? '工具运行已完成。' : `工具运行已结束：${status}。`;
      return;
    }
    await new Promise((resolve) => window.setTimeout(resolve, 1000));
  }
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
    <div class="panel-heading"><div><p class="eyebrow">工具输出</p><h3>工具实时输出</h3><p class="meta-line" role="status">{{ feedback }}</p></div><a v-if="runID" class="button button-secondary" :href="`/runs/${runID}`">查看本次完整结果</a></div>
    <div class="terminal-window tool-preview-window">
      <div class="terminal-header"><div class="terminal-dots"><span class="terminal-dot dot-red"></span><span class="terminal-dot dot-yellow"></span><span class="terminal-dot dot-green"></span></div><div class="terminal-title">tool output</div></div>
      <pre ref="terminal" class="event-log tool-command-preview" @scroll="onScroll">{{ output }}</pre>
    </div>
  </section>
</template>
