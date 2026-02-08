<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';
  import { PickModelsDir, SaveGlobalSettings, InstallBackend, GetAllBackends } from '../../bindings/github.com/UberMorgott/transcribation/services/settingsservice.js';

  export let microphoneId: string = '';
  export let microphones: { id: string; name: string; isDefault: boolean }[] = [];
  export let theme: 'dark' | 'light' = 'dark';
  export let uiLang: Lang = 'en';
  export let modelsDir: string = '';
  export let closeAction: string = '';
  export let autoStart: boolean = false;
  export let backend: string = 'auto';
  export let backends: { id: string; name: string; compiled: boolean; systemAvailable: boolean; canInstall: boolean; installHint: string }[] = [];

  const dispatch = createEventDispatcher<{
    change: { microphoneId: string; modelsDir: string; theme: 'dark' | 'light'; uiLang: Lang; closeAction: string; autoStart: boolean; backend: string };
    close: void;
    openModels: void;
  }>();

  let localMicId = '';
  let localTheme: 'dark' | 'light' = 'dark';
  let localLang: Lang = 'en';
  let localModelsDir = '';
  let localCloseAction = '';
  let localAutoStart = false;
  let localBackend = 'auto';
  let installingBackend = '';
  let backendMessage = '';
  let initialized = false;

  const langOptions: { code: Lang; label: string }[] = [
    { code: 'en', label: 'English' },
    { code: 'ru', label: 'Русский' },
    { code: 'de', label: 'Deutsch' },
    { code: 'es', label: 'Español' },
    { code: 'fr', label: 'Français' },
    { code: 'zh', label: '中文' },
    { code: 'ja', label: '日本語' },
    { code: 'pt', label: 'Português' },
    { code: 'ko', label: '한국어' },
  ];

  // Filter out Metal on non-macOS (it will have compiled=false and canInstall=false)
  $: visibleBackends = backends.filter(b =>
    b.id === 'auto' || b.id === 'cpu' ||
    b.compiled || b.systemAvailable || b.canInstall
  );

  onMount(() => {
    localMicId = microphoneId;
    localTheme = theme;
    localLang = uiLang;
    localModelsDir = modelsDir;
    localCloseAction = closeAction;
    localAutoStart = autoStart;
    localBackend = backend;
    requestAnimationFrame(() => { initialized = true; });
  });

  // Auto-save on any change
  $: if (initialized) {
    document.documentElement.setAttribute('data-theme', localTheme);
    const detail = { microphoneId: localMicId, modelsDir: localModelsDir, theme: localTheme, uiLang: localLang, closeAction: localCloseAction, autoStart: localAutoStart, backend: localBackend };
    SaveGlobalSettings(detail).catch(() => {});
    dispatch('change', detail);
  }

  $: displayLang = localLang;

  function onOverlayClick(e: MouseEvent) {
    if (e.target === e.currentTarget) dispatch('close');
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }

  async function handleBrowse() {
    try {
      const dir = await PickModelsDir();
      if (dir) localModelsDir = dir;
    } catch {}
  }

  async function handleBackendClick(b: typeof backends[0]) {
    // Usable: compiled and system available
    if (b.compiled && b.systemAvailable) {
      localBackend = b.id;
      backendMessage = '';
      return;
    }
    // Can install: trigger install
    if (b.canInstall) {
      installingBackend = b.id;
      backendMessage = '';
      try {
        const result = await InstallBackend(b.id);
        if (result === 'url') {
          backendMessage = t(displayLang, 'backendUrlOpened');
        } else {
          backendMessage = t(displayLang, 'backendInstalled');
        }
        // Refresh backends to pick up new availability
        backends = await GetAllBackends() || [];
      } catch (e: any) {
        backendMessage = e?.message || String(e);
      }
      installingBackend = '';
      return;
    }
  }

  function backendTooltip(b: typeof backends[0]): string {
    if (b.compiled && b.systemAvailable) return b.name;
    if (b.canInstall && !b.compiled) return `${b.installHint} — ${t(displayLang, 'backendNeedsRebuild')}`;
    if (b.canInstall) return `${t(displayLang, 'backendInstalling').replace('...', '')}: ${b.installHint}`;
    return t(displayLang, 'backendNotAvailable');
  }
</script>

<svelte:window on:keydown={onKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<div class="overlay" on:click={onOverlayClick} role="dialog">
  <div class="modal">
    <div class="modal-header">
      <div class="header-left">
        <div class="header-accent"></div>
        <h2 class="header-title">{t(displayLang, 'settings')}</h2>
      </div>
      <button class="close-btn" on:click={() => dispatch('close')} title={t(displayLang, 'tip_close')}>
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <div class="modal-body">
      <!-- Backend -->
      <div class="field" title={t(displayLang, 'tip_backend')}>
        <label class="field-label">{t(displayLang, 'backend')}</label>
        <div class="backend-group">
          {#each visibleBackends as b}
            <button
              class="backend-pill"
              class:backend-active={localBackend === b.id && (b.compiled && b.systemAvailable)}
              class:backend-unavailable={!(b.compiled && b.systemAvailable)}
              class:backend-installable={b.canInstall && !(b.compiled && b.systemAvailable)}
              class:backend-installing={installingBackend === b.id}
              disabled={!(b.compiled && b.systemAvailable) && !b.canInstall}
              on:click={() => handleBackendClick(b)}
              title={backendTooltip(b)}
            >
              {#if installingBackend === b.id}
                <span class="spinner"></span>
              {/if}
              {b.name}
            </button>
          {/each}
        </div>
        {#if backendMessage}
          <div class="backend-message">{backendMessage}</div>
        {/if}
      </div>

      <!-- Theme -->
      <div class="field" title={t(displayLang, 'tip_theme')}>
        <label class="field-label">{t(displayLang, 'theme')}</label>
        <div class="pill-group">
          <button
            class="pill-btn"
            class:pill-active={localTheme === 'dark'}
            on:click={() => localTheme = 'dark'}
          >{t(displayLang, 'dark')}</button>
          <button
            class="pill-btn"
            class:pill-active={localTheme === 'light'}
            on:click={() => localTheme = 'light'}
          >{t(displayLang, 'light')}</button>
        </div>
      </div>

      <!-- UI Language -->
      <div class="field" title={t(displayLang, 'tip_uiLanguage')}>
        <label class="field-label">{t(displayLang, 'uiLanguage')}</label>
        <select class="field-select" bind:value={localLang}>
          {#each langOptions as opt}
            <option value={opt.code}>{opt.label}</option>
          {/each}
        </select>
      </div>

      <!-- Close Action -->
      <div class="field" title={t(displayLang, 'tip_closeAction')}>
        <label class="field-label">{t(displayLang, 'closeAction')}</label>
        <div class="pill-group">
          <button
            class="pill-btn"
            class:pill-active={localCloseAction === 'tray'}
            on:click={() => localCloseAction = 'tray'}
          >{t(displayLang, 'closeToTray')}</button>
          <button
            class="pill-btn"
            class:pill-active={localCloseAction === 'quit'}
            on:click={() => localCloseAction = 'quit'}
          >{t(displayLang, 'closeQuit')}</button>
        </div>
      </div>

      <!-- Auto Start -->
      <div class="field" title={t(displayLang, 'tip_autoStart')}>
        <label class="field-label">{t(displayLang, 'autoStart')}</label>
        <div class="pill-group">
          <button
            class="pill-btn"
            class:pill-active={localAutoStart}
            on:click={() => localAutoStart = true}
          >{t(displayLang, 'on')}</button>
          <button
            class="pill-btn"
            class:pill-active={!localAutoStart}
            on:click={() => localAutoStart = false}
          >{t(displayLang, 'off')}</button>
        </div>
      </div>

      <!-- Microphone -->
      <div class="field" title={t(displayLang, 'tip_microphone')}>
        <label class="field-label">{t(displayLang, 'microphone')}</label>
        <select class="field-select" bind:value={localMicId}>
          <option value="">{t(displayLang, 'default_mic')}</option>
          {#each microphones as mic}
            <option value={mic.id}>{mic.name}{mic.isDefault ? ' *' : ''}</option>
          {/each}
        </select>
      </div>

      <!-- Models Directory -->
      <div class="field" title={t(displayLang, 'tip_modelsDir')}>
        <label class="field-label">{t(displayLang, 'modelsDirectory')}</label>
        <div class="dir-row">
          <input class="dir-input" type="text" readonly value={localModelsDir} />
          <button class="browse-btn" on:click={handleBrowse} title={t(displayLang, 'tip_browse')}>{t(displayLang, 'browse')}</button>
        </div>
      </div>

      <!-- Models -->
      <div class="field">
        <label class="field-label">{t(displayLang, 'models')}</label>
        <button class="models-btn" on:click={() => dispatch('openModels')} title={t(displayLang, 'tip_manageModels')}>
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
          </svg>
          {t(displayLang, 'manageModels')}
        </button>
      </div>
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
    width: 400px;
    max-height: 90vh;
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

  .modal-body {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    overflow-y: auto;
  }
  .field-label {
    font-size: 12px; color: var(--text-tertiary); letter-spacing: 0.08em;
    text-transform: uppercase; font-family: ui-monospace, monospace;
  }
  .field-select {
    background: var(--bg-input); border: 1.5px solid var(--toggle-border);
    border-radius: 6px; padding: 8px 12px; font-size: 13px; color: var(--text-secondary);
    outline: none; transition: border-color 0.2s; font-family: ui-monospace, monospace;
  }
  .field-select:focus { border-color: var(--border-hover); }
  .field-select option { background: var(--bg-page); color: var(--text-secondary); }

  /* Backend pill buttons */
  .backend-group {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .backend-pill {
    padding: 6px 14px;
    font-size: 12px;
    font-family: ui-monospace, monospace;
    letter-spacing: 0.05em;
    border-radius: 6px;
    border: 1.5px solid var(--toggle-border);
    background: var(--toggle-bg);
    color: var(--text-muted);
    cursor: pointer;
    transition: color 0.2s, background 0.2s, border-color 0.2s;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .backend-pill:hover:not(:disabled) {
    color: var(--text-secondary);
    border-color: var(--border-hover);
  }
  .backend-pill.backend-active {
    background: var(--accent-dim);
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
  }
  .backend-pill.backend-unavailable {
    opacity: 0.35;
    cursor: not-allowed;
  }
  .backend-pill.backend-installable {
    opacity: 0.55;
    cursor: pointer;
    border-style: dashed;
  }
  .backend-pill.backend-installable:hover {
    opacity: 0.8;
    color: var(--accent);
    border-color: var(--accent);
  }
  .backend-pill.backend-installing {
    opacity: 0.7;
    cursor: wait;
  }

  .backend-message {
    font-size: 11px;
    color: var(--accent);
    font-family: ui-monospace, monospace;
    padding: 2px 0;
  }

  /* Spinner for installing state */
  .spinner {
    width: 12px;
    height: 12px;
    border: 2px solid var(--text-muted);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Theme pill buttons */
  .pill-group {
    display: flex;
    gap: 0;
    border-radius: 6px;
    overflow: hidden;
    border: 1.5px solid var(--toggle-border);
    width: fit-content;
  }
  .pill-btn {
    padding: 6px 16px;
    font-size: 13px;
    font-family: ui-monospace, monospace;
    background: var(--toggle-bg);
    color: var(--text-muted);
    border: none;
    cursor: pointer;
    transition: color 0.2s, background 0.2s, border-color 0.2s;
  }
  .pill-btn:not(:last-child) {
    border-right: 1.5px solid var(--toggle-border);
  }
  .pill-btn:hover {
    color: var(--text-secondary);
  }
  .pill-btn.pill-active {
    background: var(--accent-dim);
    color: var(--accent);
  }

  /* Models directory row */
  .dir-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .dir-input {
    flex: 1;
    background: var(--bg-input);
    border: 1.5px solid var(--toggle-border);
    border-radius: 6px;
    padding: 8px 12px;
    font-size: 12px;
    color: var(--text-secondary);
    font-family: ui-monospace, monospace;
    outline: none;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .browse-btn {
    padding: 8px 14px;
    border-radius: 6px;
    border: 1.5px solid var(--toggle-border);
    background: var(--toggle-bg);
    color: var(--text-tertiary);
    font-size: 13px;
    font-family: ui-monospace, monospace;
    cursor: pointer;
    transition: color 0.2s, background 0.2s, border-color 0.2s;
    flex-shrink: 0;
  }
  .browse-btn:hover {
    color: var(--accent);
    border-color: var(--border-hover);
  }

  .models-btn {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 14px; border-radius: 6px;
    border: 1.5px solid var(--toggle-border);
    background: var(--toggle-bg); color: var(--text-tertiary);
    font-size: 13px; cursor: pointer; transition: color 0.2s, background 0.2s, border-color 0.2s;
  }
  .models-btn:hover { color: var(--accent); border-color: var(--border-hover); }

</style>
