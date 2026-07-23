<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { getTheme, setTheme, type ThemePreference } from './theme';

const current = ref<ThemePreference>('system');

function select(theme: ThemePreference) {
  current.value = theme;
  setTheme(theme);
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
const labels: Record<ThemePreference, string> = {
  light: '浅色',
  dark: '深色',
  system: '跟随系统',
};
</script>

<template>
  <fieldset class="theme-toggle" aria-label="外观主题">
    <button
      v-for="theme of (['light', 'system', 'dark'] as ThemePreference[])"
      :key="theme"
      type="button"
      class="theme-toggle-btn"
      :class="{ active: current === theme }"
      :aria-pressed="current === theme"
      @click="select(theme)"
    >
      <span class="theme-toggle-label">{{ labels[theme] }}</span>
    </button>
  </fieldset>
</template>

<style scoped>
.theme-toggle {
  border: 0;
  margin: 0;
  padding: 0;
  display: flex;
  gap: 0.25rem;
  background: var(--bg-accent);
  border-radius: var(--radius-md);
  padding: 0.2rem;
}

.theme-toggle-btn {
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.35rem 0.5rem;
  border-radius: calc(var(--radius-md) - 2px);
  cursor: pointer;
  transition: all 0.15s ease;
}

.theme-toggle-btn:hover {
  color: var(--heading);
}

.theme-toggle-btn.active {
  background: var(--panel);
  color: var(--heading);
  box-shadow: 0 1px 2px var(--shadow-color, rgba(0, 0, 0, 0.08));
}
</style>
