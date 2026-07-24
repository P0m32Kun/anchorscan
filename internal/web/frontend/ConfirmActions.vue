<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

type ConfirmationRequest = {
  title?: string;
  message?: string;
  trigger?: HTMLElement;
  resolve: (confirmed: boolean) => void;
};

const dialog = ref<HTMLDialogElement>();
const title = ref('确认操作');
const message = ref('此操作不可撤销。');
let pendingForm: HTMLFormElement | null = null;
let pendingSubmitter: HTMLElement | null = null;
let confirmedForm: HTMLFormElement | null = null;
let pendingRequest: ConfirmationRequest | null = null;
let requestConfirmed = false;

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
  if (pendingForm) confirmedForm = pendingForm;
  if (pendingRequest) requestConfirmed = true;
  dialog.value?.close();
}

function onClose() {
  const form = pendingForm;
  const submitter = pendingSubmitter;
  const request = pendingRequest;
  pendingForm = null;
  pendingSubmitter = null;
  if (form && confirmedForm === form) {
    form.requestSubmit();
    return;
  }
  if (request) {
    const confirmed = requestConfirmed;
    pendingRequest = null;
    requestConfirmed = false;
    request.resolve(confirmed);
    if (confirmed) return;
  }
  void nextTick(() => submitter?.focus());
}

function onConfirmationRequest(event: Event) {
  const request = (event as CustomEvent<ConfirmationRequest>).detail;
  if (!request?.resolve) return;
  if (pendingForm || pendingRequest) {
    request.resolve(false);
    return;
  }
  pendingRequest = request;
  pendingSubmitter = request.trigger || null;
  title.value = request.title || '确认操作';
  message.value = request.message || '此操作不可撤销。';
  dialog.value?.showModal();
}

onMounted(() => {
  document.addEventListener('submit', onSubmit);
  document.addEventListener('anchorscan:confirm', onConfirmationRequest);
});
onBeforeUnmount(() => {
  document.removeEventListener('submit', onSubmit);
  document.removeEventListener('anchorscan:confirm', onConfirmationRequest);
});
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
