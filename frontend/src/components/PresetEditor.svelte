<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';
  import HotkeyCapture from './HotkeyCapture.svelte';

  export let preset: {
    id: string;
    name: string;
    modelName: string;
    keepModelLoaded: boolean;
    inputMode: string;
    hotkey: string;
    language: string;
    useKBLayout: boolean;
    keepHistory: boolean;
    enabled: boolean;
  } | null = null;

  export let models: { name: string; downloaded: boolean }[] = [];
  export let languages: { code: string; name: string }[] = [];
  export let isNew = false;
  export let lang: Lang = 'en';

  const dispatch = createEventDispatcher<{
    save: typeof form;
    delete: string;
    close: void;
    openModels: void;
    modelChanged: string;
  }>();

  let form = {
    id: '',
    name: '',
    modelName: '',
    keepModelLoaded: false,
    inputMode: 'hold',
    hotkey: '',
    language: 'auto',
    useKBLayout: false,
    keepHistory: true,
    enabled: false,
  };

  $: downloadedModels = models.filter(m => m.downloaded);
  $: languageDisabled = languages.length <= 1;

  // When model changes, notify parent to reload languages
  function onModelChange() {
    dispatch('modelChanged', form.modelName);
  }

  onMount(() => {
    if (preset) {
      form = { ...preset };
    }
  });

  function onOverlayClick(e: MouseEvent) {
    if (e.target === e.currentTarget) dispatch('close');
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }

  function handleSave() {
    if (!form.name.trim()) form.name = 'Untitled';
    dispatch('save', { ...form });
  }

  function handleDelete() {
    if (form.id) dispatch('delete', form.id);
  }
</script>

<svelte:window on:keydown={onKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<div class="overlay" on:click={onOverlayClick} role="dialog">
  <div class="modal">
    <div class="modal-header">
      <div class="header-left">
        <div class="header-accent"></div>
        <h2 class="header-title">{isNew ? t(lang, 'newPresetTitle') : t(lang, 'editPreset')}</h2>
      </div>
      <button class="close-btn" on:click={() => dispatch('close')} title={t(lang, 'tip_close')}>
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <div class="modal-body">
      <!-- Name -->
      <div class="field" title={t(lang, 'tip_name')}>
        <label class="field-label">{t(lang, 'name')}</label>
        <input class="field-input" type="text" bind:value={form.name} placeholder={t(lang, 'presetName')} />
      </div>

      <!-- Model -->
      <div class="field" title={t(lang, 'tip_model')}>
        <label class="field-label">{t(lang, 'model')}</label>
        <div class="field-row">
          <select class="field-select" bind:value={form.modelName} on:change={onModelChange}>
            {#each downloadedModels as m}
              <option value={m.name}>{m.name}</option>
            {/each}
          </select>
          <button class="gear-btn" on:click={() => dispatch('openModels')} title={t(lang, 'tip_openModels')}>
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 010 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 010-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28z" />
              <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Keep model loaded -->
      <div class="field-check" title={t(lang, 'tip_keepModelLoaded')}>
        <label class="check-label">
          <input type="checkbox" bind:checked={form.keepModelLoaded} />
          <span>{t(lang, 'keepModelLoaded')}</span>
        </label>
      </div>

      <!-- Mode -->
      <div class="field" title={t(lang, 'tip_inputMode')}>
        <label class="field-label">{t(lang, 'inputMode')}</label>
        <div class="pill-group">
          <button class="pill" class:pill-active={form.inputMode === 'hold'} on:click={() => form.inputMode = 'hold'}>{t(lang, 'hold')}</button>
          <button class="pill" class:pill-active={form.inputMode === 'toggle'} on:click={() => form.inputMode = 'toggle'}>{t(lang, 'toggle')}</button>
        </div>
      </div>

      <!-- Hotkey -->
      <div class="field" title={t(lang, 'tip_hotkey')}>
        <label class="field-label">{t(lang, 'hotkey')}</label>
        <HotkeyCapture bind:value={form.hotkey} {lang} />
      </div>

      <!-- Use KB layout -->
      <div class="field-check" title={t(lang, 'tip_langByKBLayout')}>
        <label class="check-label">
          <input type="checkbox" bind:checked={form.useKBLayout} />
          <span>{t(lang, 'langByKBLayout')}</span>
        </label>
      </div>

      <!-- Language -->
      <div class="field" title={t(lang, 'tip_language')}>
        <label class="field-label">{t(lang, 'language')}</label>
        <select class="field-select" class:field-disabled={languageDisabled || form.useKBLayout} bind:value={form.language} disabled={languageDisabled || form.useKBLayout}>
          {#each languages as lng}
            <option value={lng.code}>{lng.name}</option>
          {/each}
        </select>
      </div>

      <!-- Keep history -->
      <div class="field-check" title={t(lang, 'tip_saveHistory')}>
        <label class="check-label">
          <input type="checkbox" bind:checked={form.keepHistory} />
          <span>{t(lang, 'saveHistory')}</span>
        </label>
      </div>
    </div>

    <div class="modal-footer">
      <button class="btn-save" on:click={handleSave} title={t(lang, 'tip_save')}>{t(lang, 'save')}</button>
      {#if !isNew}
        <button class="btn-delete" on:click={handleDelete} title={t(lang, 'tip_delete')}>{t(lang, 'delete')}</button>
      {/if}
      <button class="btn-cancel" on:click={() => dispatch('close')} title={t(lang, 'tip_cancel')}>{t(lang, 'cancel')}</button>
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
    z-index: 100;
    backdrop-filter: blur(2px);
  }

  .modal {
    background: var(--bg-page);
    border: 1.5px solid var(--border-color);
    border-radius: 12px;
    width: 420px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 0 60px rgba(0, 0, 0, 0.6), 0 0 30px var(--accent-dim);
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px;
    border-bottom: 1px solid var(--accent-dim);
    flex-shrink: 0;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .header-accent {
    width: 3px;
    height: 14px;
    border-radius: 2px;
    background: linear-gradient(to bottom, var(--accent), var(--accent-red));
  }
  .header-title {
    font-size: 14px;
    color: var(--text-primary);
    letter-spacing: 0.12em;
    text-transform: uppercase;
    font-family: ui-monospace, monospace;
  }

  .close-btn {
    color: var(--text-muted);
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    transition: color 0.2s;
    display: flex;
    align-items: center;
  }
  .close-btn:hover { color: var(--accent); }

  .modal-body {
    padding: 16px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .modal-body::-webkit-scrollbar { width: 3px; }
  .modal-body::-webkit-scrollbar-track { background: transparent; }
  .modal-body::-webkit-scrollbar-thumb { background: var(--border-subtle); border-radius: 3px; }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .field-label {
    font-size: 12px;
    color: var(--text-tertiary);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-family: ui-monospace, monospace;
  }

  .field-input {
    background: var(--bg-input);
    border: 1.5px solid var(--toggle-border);
    border-radius: 6px;
    padding: 8px 12px;
    font-size: 14px;
    color: var(--text-secondary);
    outline: none;
    transition: border-color 0.2s;
  }
  .field-input:focus { border-color: var(--border-hover); }
  .field-input::placeholder { color: var(--text-muted); }

  .field-select {
    flex: 1;
    min-width: 0;
    background: var(--bg-input);
    border: 1.5px solid var(--toggle-border);
    border-radius: 6px;
    padding: 8px 12px;
    font-size: 13px;
    color: var(--text-secondary);
    outline: none;
    transition: border-color 0.2s;
    font-family: ui-monospace, monospace;
  }
  .field-select:focus { border-color: var(--border-hover); }
  .field-select option { background: var(--bg-page); color: var(--text-secondary); }
  .field-disabled {
    opacity: 0.4;
    cursor: not-allowed;
    pointer-events: none;
  }

  .field-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .gear-btn {
    width: 34px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    border: 1.5px solid var(--toggle-border);
    background: transparent;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.2s;
    flex-shrink: 0;
  }
  .gear-btn:hover { color: var(--accent); border-color: var(--border-hover); }

  .field-check {
    display: flex;
    align-items: center;
  }
  .check-label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 13px;
    color: var(--text-secondary);
  }
  .check-label input[type="checkbox"] {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
  }

  .pill-group { display: flex; gap: 4px; }
  .pill {
    padding: 6px 16px;
    border-radius: 6px;
    font-size: 13px;
    font-family: ui-monospace, monospace;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    border: 1.5px solid var(--pill-border);
    background: var(--pill-bg);
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.2s;
  }
  .pill:hover { color: var(--text-secondary); border-color: var(--border-hover); }
  .pill-active {
    color: var(--accent) !important;
    background: var(--accent-dim) !important;
    border-color: var(--border-color) !important;
    box-shadow: 0 0 10px var(--accent-dim);
  }

  .modal-footer {
    display: flex;
    gap: 8px;
    padding: 14px 16px;
    border-top: 1px solid var(--accent-dim);
    flex-shrink: 0;
  }

  .btn-save {
    flex: 1;
    padding: 8px;
    border-radius: 8px;
    border: 1.5px solid var(--border-hover);
    background: var(--accent-dim);
    color: var(--accent);
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-save:hover {
    background: rgba(249, 115, 22, 0.18);
    box-shadow: 0 0 20px var(--border-subtle);
  }

  .btn-delete {
    padding: 8px 16px;
    border-radius: 8px;
    border: 1.5px solid rgba(220, 38, 38, 0.25);
    background: transparent;
    color: #e07070;
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-delete:hover {
    color: var(--accent-red);
    border-color: rgba(220, 38, 38, 0.4);
    background: rgba(220, 38, 38, 0.06);
  }

  .btn-cancel {
    padding: 8px 16px;
    border-radius: 8px;
    border: 1.5px solid var(--toggle-border);
    background: transparent;
    color: var(--text-tertiary);
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-cancel:hover { color: var(--text-secondary); border-color: var(--border-hover); }
</style>
