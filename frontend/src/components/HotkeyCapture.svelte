<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { CaptureHotkey, CancelCapture } from '../../bindings/github.com/UberMorgott/transcribation/services/presetservice.js';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';

  export let value: string = '';
  export let lang: Lang = 'en';
  let capturing = false;

  const dispatch = createEventDispatcher<{ change: string }>();

  async function startCapture() {
    if (capturing) return;
    capturing = true;
    try {
      const result = await CaptureHotkey();
      if (result) {
        value = result;
        dispatch('change', value);
      }
    } finally {
      capturing = false;
    }
  }

  function clear() {
    value = '';
    dispatch('change', '');
  }

  onDestroy(() => {
    if (capturing) {
      CancelCapture();
    }
  });
</script>

<div class="hk-wrap">
  <button class="hk-btn" class:hk-capturing={capturing} on:click={startCapture}>
    {#if capturing}
      <span class="hk-pulse">{t(lang, 'pressKey')}</span>
    {:else if value}
      <span class="hk-value">{value}</span>
    {:else}
      <span class="hk-placeholder">{t(lang, 'clickToSet')}</span>
    {/if}
  </button>
  {#if value && !capturing}
    <button class="hk-clear" on:click={clear} title={t(lang, 'clearHotkey')}>
      <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  {/if}
</div>

<style>
  .hk-wrap {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
  }

  .hk-btn {
    flex: 1;
    min-width: 0;
    padding: 6px 12px;
    border-radius: 6px;
    border: 1.5px solid var(--toggle-border);
    background: var(--bg-input);
    color: var(--text-secondary);
    font-size: 13px;
    font-family: ui-monospace, monospace;
    cursor: pointer;
    transition: all 0.2s;
    text-align: left;
  }
  .hk-btn:hover {
    border-color: var(--border-hover);
  }
  .hk-capturing {
    border-color: var(--border-active) !important;
    box-shadow: 0 0 16px var(--accent-dim);
  }

  .hk-pulse {
    color: var(--accent);
    animation: blink 1s ease-in-out infinite;
  }

  .hk-value {
    color: var(--text-secondary);
    text-transform: capitalize;
  }

  .hk-placeholder {
    color: var(--text-muted);
  }

  .hk-clear {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
    flex-shrink: 0;
    transition: color 0.2s;
  }
  .hk-clear:hover {
    color: var(--accent-red);
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
</style>
