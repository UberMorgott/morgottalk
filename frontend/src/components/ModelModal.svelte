<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';
  import ProgressBar from './ProgressBar.svelte';

  export let models: { name: string; fileName: string; size: string; sizeBytes: number; downloaded: boolean }[] = [];
  export let downloading: Record<string, number> = {};
  export let modelsDir: string = '';
  export let lang: Lang = 'en';

  const dispatch = createEventDispatcher();

  function onOverlayClick(e: MouseEvent) {
    if (e.target === e.currentTarget) dispatch('close');
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }
</script>

<svelte:window on:keydown={onKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<div class="overlay" on:click={onOverlayClick} role="dialog">
  <div class="modal">
    <div class="modal-header">
      <div class="header-left">
        <div class="header-accent"></div>
        <h2 class="header-title">{t(lang, 'models')}</h2>
      </div>
      <button class="close-btn" on:click={() => dispatch('close')} title={t(lang, 'tip_close')}>
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    {#if modelsDir}
      <div class="modal-dir" title={modelsDir}>{modelsDir}</div>
    {/if}

    <div class="modal-list">
      {#each models as model (model.name)}
        <div class="model-row" class:model-downloaded={model.downloaded}>
          <div class="model-info">
            <span class="model-name">{model.name}</span>
            {#if model.downloaded}
              <span class="model-badge">{t(lang, 'downloaded')}</span>
            {/if}
            <span class="model-size">{model.size}</span>
          </div>

          {#if downloading[model.name] !== undefined}
            <div class="model-actions">
              <div class="progress-wrap">
                <ProgressBar percent={downloading[model.name]} />
              </div>
              <button class="model-btn btn-cancel" on:click={() => dispatch('cancel', model.name)} title={t(lang, 'tip_modelCancel')}>
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          {:else if model.downloaded}
            <div class="model-actions">
              <button class="model-btn btn-del" on:click={() => dispatch('delete', model.name)} title={t(lang, 'tip_modelDelete')}>{t(lang, 'modelDel')}</button>
            </div>
          {:else}
            <button class="model-btn btn-dl" on:click={() => dispatch('download', model.name)} title={t(lang, 'tip_modelDownload')}>{t(lang, 'modelGet')}</button>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 110;
    backdrop-filter: blur(2px);
  }

  .modal {
    background: var(--bg-page);
    border: 1.5px solid var(--border-color);
    border-radius: 12px;
    width: 440px;
    max-height: 520px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 0 60px rgba(0, 0, 0, 0.6), 0 0 24px var(--accent-dim);
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px;
    border-bottom: 1px solid var(--accent-dim);
    flex-shrink: 0;
  }

  .header-left { display: flex; align-items: center; gap: 8px; }
  .header-accent {
    width: 3px; height: 14px; border-radius: 2px;
    background: linear-gradient(to bottom, var(--accent), var(--accent-red));
  }
  .header-title {
    font-size: 14px; color: var(--text-primary); letter-spacing: 0.12em;
    text-transform: uppercase; font-family: ui-monospace, monospace;
  }

  .close-btn {
    color: var(--text-muted); background: transparent; border: none;
    cursor: pointer; padding: 4px; border-radius: 4px;
    transition: color 0.2s; display: flex; align-items: center;
  }
  .close-btn:hover { color: var(--accent); }

  .modal-dir {
    font-size: 11px; color: var(--text-muted); padding: 6px 16px;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
    font-family: ui-monospace, monospace;
    border-bottom: 1px solid var(--border-subtle); flex-shrink: 0;
  }

  .modal-list {
    flex: 1; overflow-y: auto; padding: 8px 12px 12px;
    display: flex; flex-direction: column; gap: 4px; min-height: 0;
  }
  .modal-list::-webkit-scrollbar { width: 3px; }
  .modal-list::-webkit-scrollbar-track { background: transparent; }
  .modal-list::-webkit-scrollbar-thumb { background: var(--border-subtle); border-radius: 3px; }

  .model-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 8px;
    background: var(--toggle-bg);
    border: 1.5px solid var(--border-subtle);
    flex-shrink: 0; gap: 8px;
  }
  .model-downloaded { border-color: var(--border-color); }

  .model-info { display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1; }
  .model-name {
    font-size: 13px; color: var(--text-primary); font-family: ui-monospace, monospace;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .model-badge {
    font-size: 9px; text-transform: uppercase; letter-spacing: 0.06em;
    color: var(--text-tertiary); background: var(--toggle-bg);
    padding: 2px 6px; border-radius: 4px; flex-shrink: 0;
    font-family: ui-monospace, monospace;
  }
  .model-size {
    font-size: 11px; color: var(--text-muted); font-family: ui-monospace, monospace; flex-shrink: 0;
  }

  .model-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
  .progress-wrap { width: 80px; }

  .model-btn {
    font-size: 12px; font-family: ui-monospace, monospace;
    padding: 4px 10px; border-radius: 5px; border: none;
    cursor: pointer; transition: all 0.15s; text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .btn-del { color: var(--text-muted); background: transparent; }
  .btn-del:hover { color: var(--accent-red); background: rgba(220, 38, 38, 0.08); }
  .btn-dl { color: var(--text-tertiary); background: transparent; }
  .btn-dl:hover { color: var(--accent); background: var(--accent-dim); }
  .btn-cancel {
    color: var(--text-muted); background: transparent; padding: 4px;
    display: flex; align-items: center;
  }
  .btn-cancel:hover { color: var(--accent-red); }
</style>
