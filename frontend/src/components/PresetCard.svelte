<script lang="ts">
  import { createEventDispatcher } from 'svelte';
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
  };
  export let state: string = 'idle';
  export let lang: Lang = 'en';
  export let models: { name: string; downloaded: boolean }[] = [];
  export let languages: { code: string; name: string }[] = [];
  export let expanded: boolean = false;

  const dispatch = createEventDispatcher<{
    toggle: { id: string; enabled: boolean };
    expand: string;
    collapse: void;
    save: typeof form;
    delete: string;
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

  let initialized = false;
  let confirmingDelete = false;
  let _saveTimer: ReturnType<typeof setTimeout>;
  let _openedId: string | null = null;

  function debouncedSave(data: typeof form) {
    clearTimeout(_saveTimer);
    _saveTimer = setTimeout(() => dispatch('save', data), 400);
  }

  $: isActive = state === 'recording' || state === 'processing';
  $: downloadedModels = models.filter(m => m.downloaded);
  $: languageDisabled = languages.length <= 1;

  // Initialize form ONLY when card first opens (not on preset prop updates)
  $: if (expanded && _openedId !== preset.id) {
    _openedId = preset.id;
    form = { ...preset };
    requestAnimationFrame(() => { initialized = true; });
  } else if (!expanded) {
    initialized = false;
    _openedId = null;
    confirmingDelete = false;
  }

  // Auto-save on form change (debounced)
  $: if (initialized && expanded) {
    const data = { ...form };
    if (!data.name.trim()) data.name = 'Untitled';
    debouncedSave(data);
  }

  function handleHeaderClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (target.closest('.toggle-sw')) return;
    if (target.closest('.drag-grip')) return;
    if (expanded) {
      dispatch('collapse');
    } else {
      dispatch('expand', preset.id);
    }
  }

  function onModelChange() {
    dispatch('modelChanged', form.modelName);
  }

  function handleDelete() {
    if (form.id) dispatch('delete', form.id);
  }
</script>

<div
  class="card"
  class:card-active={isActive}
  class:card-enabled={preset.enabled}
  class:card-disabled={!preset.enabled}
  class:card-expanded={expanded}
>
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="card-header" on:click={handleHeaderClick}>
    {#if !expanded}
      <svg class="drag-grip" viewBox="0 0 10 16" fill="currentColor">
        <circle cx="3" cy="2" r="1.5"/><circle cx="7" cy="2" r="1.5"/>
        <circle cx="3" cy="8" r="1.5"/><circle cx="7" cy="8" r="1.5"/>
        <circle cx="3" cy="14" r="1.5"/><circle cx="7" cy="14" r="1.5"/>
      </svg>
    {/if}
    <span class="card-name">{preset.name}</span>
    <div class="card-header-right">
      {#if isActive}
        <span class="state-badge state-{state}">
          {state === 'recording' ? t(lang, 'rec') : t(lang, 'processingDots')}
        </span>
      {/if}
      <button
        class="toggle-sw"
        class:toggle-on={preset.enabled}
        on:click={() => dispatch('toggle', { id: preset.id, enabled: !preset.enabled })}
        title={t(lang, 'tip_togglePreset')}
      >
        <div class="toggle-thumb" class:toggle-thumb-on={preset.enabled}></div>
      </button>
    </div>
  </div>

  <!-- Collapsed summary -->
  {#if !expanded}
    <div class="card-details">
      <span class="detail">{preset.modelName}</span>
      <span class="detail-sep"></span>
      <span class="detail">{preset.inputMode === 'hold' ? t(lang, 'hold') : t(lang, 'toggle')}</span>
      {#if preset.hotkey}
        <span class="detail-sep"></span>
        <span class="detail hotkey">{preset.hotkey}</span>
      {/if}
    </div>

    <div class="card-footer">
      <div class="footer-info">
        {#if preset.useKBLayout}
          <span class="detail">{t(lang, 'kb')}</span>
        {:else}
          <span class="detail">{preset.language === 'auto' ? t(lang, 'autoDetect') : preset.language.toUpperCase()}</span>
        {/if}
        {#if preset.keepHistory}
          <span class="detail-sep"></span>
          <span class="detail">{t(lang, 'history')}</span>
        {/if}
        {#if preset.keepModelLoaded}
          <span class="detail-sep"></span>
          <span class="detail">{t(lang, 'pinned')}</span>
        {/if}
      </div>
      <svg class="chevron" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
      </svg>
    </div>
  {/if}

  <!-- Expanded editor -->
  <div class="expand-body" class:expand-body-open={expanded}>
    <div class="expand-inner">
      {#if expanded}
        <div class="expand-fields">
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
              <button class="gear-btn" on:click|stopPropagation={() => dispatch('openModels')} title={t(lang, 'tip_openModels')}>
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
              <button class="pill" class:pill-active={form.inputMode === 'hold'} on:click|stopPropagation={() => form.inputMode = 'hold'}>{t(lang, 'hold')}</button>
              <button class="pill" class:pill-active={form.inputMode === 'toggle'} on:click|stopPropagation={() => form.inputMode = 'toggle'}>{t(lang, 'toggle')}</button>
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

        <!-- Action buttons -->
        <div class="expand-actions">
          {#if confirmingDelete}
            <span class="confirm-text">{t(lang, 'delete')} «{form.name}»?</span>
            <div class="confirm-btns">
              <button class="btn-confirm-yes" on:click|stopPropagation={handleDelete}>{t(lang, 'confirm')}</button>
              <button class="btn-confirm-no" on:click|stopPropagation={() => confirmingDelete = false}>{t(lang, 'cancel')}</button>
            </div>
          {:else}
            <button class="btn-delete" on:click|stopPropagation={() => confirmingDelete = true} title={t(lang, 'tip_delete')}>{t(lang, 'delete')}</button>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .card {
    background: var(--bg-card);
    border: 2px solid var(--border-color);
    border-radius: 10px;
    padding: 12px 14px;
    transition: all 0.25s ease;
  }
  .card-expanded {
    border-color: var(--border-active);
    box-shadow: 0 0 24px color-mix(in srgb, var(--accent) 15%, transparent);
  }
  .card-enabled {
    border-color: var(--border-hover);
    box-shadow: 0 0 12px color-mix(in srgb, var(--accent) 10%, transparent);
  }
  .card-disabled .card-name,
  .card-disabled .card-details,
  .card-disabled .card-footer,
  .card-disabled .expand-body {
    opacity: 0.5;
  }
  .card-active {
    border-color: var(--border-active) !important;
    box-shadow: 0 0 28px color-mix(in srgb, var(--accent) 25%, transparent), 0 0 8px color-mix(in srgb, var(--accent) 20%, transparent);
    opacity: 1 !important;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 6px;
    cursor: pointer;
  }

  .card-name {
    font-size: 15px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .card-header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .state-badge {
    font-size: 10px;
    font-family: ui-monospace, monospace;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 2px 8px;
    border-radius: 4px;
  }
  .state-recording {
    color: var(--accent-red);
    background: color-mix(in srgb, var(--accent-red) 12%, transparent);
    animation: pulse 1.5s ease-in-out infinite;
  }
  .state-processing {
    color: var(--accent);
    background: color-mix(in srgb, var(--accent) 12%, transparent);
  }

  /* Toggle switch */
  .toggle-sw {
    width: 36px;
    height: 20px;
    border-radius: 10px;
    background: var(--toggle-bg);
    border: 1.5px solid var(--toggle-border);
    position: relative;
    cursor: pointer;
    transition: all 0.2s;
    flex-shrink: 0;
  }
  .toggle-on {
    background: color-mix(in srgb, var(--accent) 20%, transparent) !important;
    border-color: color-mix(in srgb, var(--accent) 50%, transparent) !important;
    box-shadow: 0 0 12px color-mix(in srgb, var(--accent) 20%, transparent);
  }
  .toggle-thumb {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--text-muted);
    position: absolute;
    top: 1px;
    left: 1px;
    transition: all 0.2s;
  }
  .toggle-thumb-on {
    left: 17px;
    background: var(--accent);
    box-shadow: 0 0 6px color-mix(in srgb, var(--accent) 40%, transparent);
  }

  /* Collapsed summary */
  .card-details {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 6px;
  }

  .detail {
    font-size: 12px;
    color: var(--text-secondary);
    font-family: ui-monospace, monospace;
  }
  .hotkey {
    color: var(--text-tertiary);
    text-transform: capitalize;
  }
  .detail-sep {
    width: 3px;
    height: 3px;
    border-radius: 50%;
    background: var(--sep-color);
    flex-shrink: 0;
  }

  .card-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .footer-info {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .chevron {
    width: 16px;
    height: 16px;
    color: var(--text-muted);
    transition: transform 0.25s ease;
    flex-shrink: 0;
  }

  /* ── Expandable editor section ── */
  .expand-body {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows 0.3s ease;
  }
  .expand-body-open {
    grid-template-rows: 1fr;
  }
  .expand-inner {
    overflow: hidden;
  }

  .expand-fields {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--accent-dim);
    margin-top: 6px;
  }

  /* Field styles (matching PresetEditor) */
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

  /* Action buttons */
  .expand-actions {
    display: flex;
    gap: 8px;
    padding-top: 14px;
    margin-top: 14px;
    border-top: 1px solid var(--accent-dim);
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

  .confirm-text {
    font-size: 13px;
    color: var(--accent-red);
    font-family: ui-monospace, monospace;
  }
  .confirm-btns {
    display: flex;
    gap: 8px;
    margin-left: auto;
  }
  .btn-confirm-yes {
    padding: 6px 14px;
    border-radius: 6px;
    border: 1.5px solid rgba(220, 38, 38, 0.4);
    background: rgba(220, 38, 38, 0.1);
    color: var(--accent-red);
    font-size: 13px;
    font-family: ui-monospace, monospace;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-confirm-yes:hover {
    background: rgba(220, 38, 38, 0.2);
    border-color: rgba(220, 38, 38, 0.6);
  }
  .btn-confirm-no {
    padding: 6px 14px;
    border-radius: 6px;
    border: 1.5px solid var(--toggle-border);
    background: transparent;
    color: var(--text-tertiary);
    font-size: 13px;
    font-family: ui-monospace, monospace;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-confirm-no:hover {
    color: var(--text-secondary);
    border-color: var(--border-hover);
  }

  .drag-grip {
    width: 8px;
    height: 14px;
    color: var(--text-muted);
    flex-shrink: 0;
    cursor: grab;
    opacity: 0.4;
    transition: opacity 0.2s;
  }
  .card-header:hover .drag-grip {
    opacity: 0.8;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.6; }
  }
</style>
