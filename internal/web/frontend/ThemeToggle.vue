<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { getTheme, setTheme, type ThemePreference } from './theme';

const current = ref<ThemePreference>('system');

function toggleTheme() {
  const next = current.value === 'dark' ? 'light' : 'dark';
  current.value = next;
  setTheme(next);
}

function onThemeChange(e: Event) {
  current.value = (e as CustomEvent).detail as ThemePreference;
}

onMounted(() => {
  current.value = getTheme();
  window.addEventListener('anchor-theme-changed', onThemeChange);
});

onUnmounted(() => {
  window.removeEventListener('anchor-theme-changed', onThemeChange);
});
</script>

<template>
  <button
    type="button"
    class="theme-toggle-btn-single"
    :aria-label="current === 'dark' ? '切换至浅色模式' : '切换至深色模式'"
    :title="current === 'dark' ? '切换至浅色模式' : '切换至深色模式'"
    @click="toggleTheme"
  >
    <svg v-if="current === 'dark'" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="theme-icon">
      <!-- 太阳图标 -->
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" />
    </svg>
    <svg v-else xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="theme-icon">
      <!-- 月亮图标 -->
      <path stroke-linecap="round" stroke-linejoin="round" d="M21.75 12.83A9.5 9.5 0 0112 2.25 9.75 9.75 0 0011.07 22A9.75 9.75 0 0021.75 12.83z" />
    </svg>
  </button>
</template>

<style scoped>
.theme-toggle-btn-single {
  border: 0;
  background: var(--bg-accent);
  color: var(--muted);
  padding: 0.5rem;
  border-radius: var(--radius-md);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  border: 1px solid var(--border);
  box-shadow: var(--surface-shadow);
}

.theme-toggle-btn-single:hover {
  background: var(--surface-overlay-hover);
  color: var(--heading);
  transform: scale(1.05);
}

.theme-icon {
  width: 1.2rem;
  height: 1.2rem;
}
</style>
