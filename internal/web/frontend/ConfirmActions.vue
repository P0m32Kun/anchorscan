<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

const dialog = ref<HTMLDialogElement>();
const title = ref('确认操作');
const message = ref('此操作不可撤销。');
let pendingForm: HTMLFormElement | null = null;
let pendingSubmitter: HTMLElement | null = null;
let confirmedForm: HTMLFormElement | null = null;

function onSubmit(event: SubmitEvent) {
  const form = event.target;
  if (!(form instanceof HTMLFormElement) || !form.matches('[data-confirm-form]')) return;
  if (confirmedForm === form) {
    confirmedForm = null;
    return;
  }
  event.preventDefault();
  pendingForm = form;
  pendingSubmitter = event.submitter instanceof HTMLElement ? event.submitter : null;
  title.value = form.dataset.confirmTitle || '确认操作';
  message.value = form.dataset.confirmMessage || '此操作不可撤销。';
  dialog.value?.showModal();
}

function confirm() {
  if (!pendingForm) return;
  confirmedForm = pendingForm;
  dialog.value?.close();
}

function onClose() {
  const form = pendingForm;
  const submitter = pendingSubmitter;
  pendingForm = null;
  pendingSubmitter = null;
  if (form && confirmedForm === form) {
    form.requestSubmit();
    return;
  }
  void nextTick(() => submitter?.focus());
}

onMounted(() => document.addEventListener('submit', onSubmit));
onBeforeUnmount(() => document.removeEventListener('submit', onSubmit));
</script>

<template>
  <Teleport to="body">
    <dialog ref="dialog" class="panel confirmation-dialog" aria-labelledby="confirmation-dialog-title" @close="onClose">
      <div class="panel-heading"><h3 id="confirmation-dialog-title">{{ title }}</h3></div>
      <p class="meta-line confirmation-dialog-message">{{ message }}</p>
      <div class="header-actions confirmation-dialog-actions">
        <button class="button button-secondary" type="button" @click="dialog?.close()">取消</button>
        <button class="button button-danger" type="button" @click="confirm">删除</button>
      </div>
    </dialog>
  </Teleport>
</template>

<style scoped>
.confirmation-dialog { width: min(30rem, calc(100vw - 2rem)); color: var(--text); box-shadow: var(--shadow); }
.confirmation-dialog::backdrop { background: rgba(0, 0, 0, 0.42); }
.confirmation-dialog-message { margin: 0; }
.confirmation-dialog-actions { justify-content: flex-end; margin-top: 1.25rem; }
</style>
