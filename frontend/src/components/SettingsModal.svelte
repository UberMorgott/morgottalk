<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';
  import { Events } from '@wailsio/runtime';
  import { PickModelsDir, SaveGlobalSettings, InstallBackend, GetAllBackends, RestartApp } from '../../bindings/github.com/UberMorgott/transcribation/services/settingsservice.js';

  export let microphoneId: string = '';
  export let microphones: { id: string; name: string; isDefault: boolean }[] = [];
  export let theme: 'dark' | 'light' = 'dark';
  export let uiLang: Lang = 'en';
  export let modelsDir: string = '';
  export let closeAction: string = '';
  export let autoStart: boolean = false;
  export let startMinimized: boolean = false;
  export let backend: string = 'auto';
  export let backends: { id: string; name: string; compiled: boolean; systemAvailable: boolean; canInstall: boolean; installHint: string; unavailableReason: string; gpuDetected: string; recommended: boolean; downloadSizeMB: number }[] = [];
  export let onboardingDone: boolean = true;

  const dispatch = createEventDispatcher<{
    change: { microphoneId: string; modelsDir: string; theme: 'dark' | 'light'; uiLang: Lang; closeAction: string; autoStart: boolean; startMinimized: boolean; backend: string };
    close: void;
    openModels: void;
  }>();

  let localMicId = '';
  let localTheme: 'dark' | 'light' = 'dark';
  let localLang: Lang = 'en';
  let localModelsDir = '';
  let localCloseAction = '';
  let localAutoStart = false;
  let localStartMinimized = false;
  let localBackend = 'auto';
  let installingBackend = '';
  let backendMessage = '';
  let installProgress: number | null = null;
  let installStage: 'downloading' | 'installing' | 'downloading_runtime' | 'installing_runtime' | '' = '';
  let installStageText = '';
  let showRestartButton = false;
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

  let unsubInstallProgress: (() => void) | null = null;

  onMount(() => {
    localMicId = microphoneId;
    localTheme = theme;
    localLang = uiLang;
    localModelsDir = modelsDir;
    localCloseAction = closeAction;
    localAutoStart = autoStart;
    localStartMinimized = startMinimized;
    localBackend = backend;
    requestAnimationFrame(() => { initialized = true; });

    unsubInstallProgress = Events.On('backend:install:progress', (event: any) => {
      const d = event.data?.[0] || event.data || event;
      if (d.done) {
        installProgress = null;
        installStage = '';
        installStageText = '';
        if (d.error) {
          backendMessage = d.error;
        } else {
          backendMessage = t(displayLang, 'backendInstallDone');
          // Backend was hot-loaded — auto-switch without restart.
          localBackend = d.backendId;
        }
        showRestartButton = false;
        installingBackend = '';
        // Refresh backend list to reflect new availability.
        GetAllBackends().then(b => {
          backends = b || [];
        });
      } else {
        installStage = d.stage || 'downloading';
        installStageText = d.stageText || '';
        installProgress = (d.stage === 'installing' || d.stage === 'installing_runtime') ? null : (d.percent || 0);
      }
    });
  });

  onDestroy(() => {
    if (unsubInstallProgress) unsubInstallProgress();
  });

  // Auto-save on any change
  $: if (initialized) {
    document.documentElement.setAttribute('data-theme', localTheme);
    try { localStorage.setItem('morgottalk-theme', localTheme); } catch {}
    const detail = { microphoneId: localMicId, modelsDir: localModelsDir, theme: localTheme, uiLang: localLang, closeAction: localCloseAction, autoStart: localAutoStart, startMinimized: localStartMinimized, backend: localBackend, onboardingDone };
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
    // Usable: compiled and system available — just select it.
    if (b.compiled && b.systemAvailable) {
      localBackend = b.id;
      backendMessage = '';
      return;
    }
    // Can install (runtime missing or DLL missing) — trigger install + download.
    if (b.canInstall || b.unavailableReason === 'not_compiled') {
      installingBackend = b.id;
      backendMessage = '';
      try {
        const result = await InstallBackend(b.id);
        if (result === 'url') {
          backendMessage = t(displayLang, 'backendUrlOpened');
        } else if (result === 'installing') {
          // Async flow — progress events will update UI.
          return;
        } else {
          backendMessage = t(displayLang, 'backendInstalled');
        }
        backends = await GetAllBackends() || [];
      } catch (e: any) {
        backendMessage = e?.message || String(e);
      }
      installingBackend = '';
      return;
    }
  }

  async function handleRestart() {
    try {
      await RestartApp();
    } catch {}
  }

  function backendTooltip(b: typeof backends[0]): string {
    if (b.compiled && b.systemAvailable) {
      return b.recommended ? `${b.name} — ${t(displayLang, 'backendRecommended')}` : b.name;
    }
    if (b.canInstall || b.unavailableReason === 'not_compiled') {
      const gpu = b.gpuDetected ? ` (${b.gpuDetected})` : '';
      const size = b.downloadSizeMB > 0 ? ` · ~${b.downloadSizeMB} ${t(displayLang, 'mb')}` : '';
      return `${t(displayLang, 'backendClickToDownload')}${gpu}${size}`;
    }
    return t(displayLang, 'backendNotAvailable');
  }

  function backendHardwareLabel(id: string): string {
    switch (id) {
      case 'cuda': return 'NVIDIA';
      case 'vulkan': return t(displayLang, 'backendHwAnyGPU');
      case 'metal': return 'Apple';
      case 'cpu': return t(displayLang, 'backendHwProcessor');
      default: return '';
    }
  }

  $: recommendedBackend = visibleBackends.find(b => b.recommended);

  // When Auto is selected, determine which backend it actually uses.
  $: autoResolvedId = localBackend === 'auto'
    ? (visibleBackends.find(b => b.recommended && b.compiled && b.systemAvailable)?.id
      || visibleBackends.find(b => b.compiled && b.systemAvailable && b.id !== 'auto' && b.id !== 'cpu')?.id
      || 'cpu')
    : null;
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
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="field-label">{t(displayLang, 'backend')}</label>
        <div class="backend-group">
          {#each visibleBackends as b}
            <button
              class="backend-pill"
              class:backend-active={(localBackend === b.id || (localBackend === 'auto' && b.id === 'auto')) && (b.compiled && b.systemAvailable)}
              class:backend-auto-resolved={autoResolvedId === b.id && b.id !== 'auto'}
              class:backend-unavailable={!(b.compiled && b.systemAvailable) && !b.canInstall && b.unavailableReason !== 'not_compiled'}
              class:backend-installable={(b.canInstall || b.unavailableReason === 'not_compiled') && !(b.compiled && b.systemAvailable)}
              class:backend-installing={installingBackend === b.id}
              disabled={!(b.compiled && b.systemAvailable) && !b.canInstall && b.unavailableReason !== 'not_compiled'}
              on:click={() => handleBackendClick(b)}
              title={backendTooltip(b)}
            >
              {#if installingBackend === b.id}
                {#if (installStage === 'downloading' || installStage === 'downloading_runtime') && installProgress !== null}
                  <svg class="progress-ring" width="14" height="14" viewBox="0 0 14 14">
                    <circle class="progress-ring-bg" cx="7" cy="7" r="5" />
                    <circle class="progress-ring-fill" cx="7" cy="7" r="5"
                      stroke-dasharray={31.4}
                      stroke-dashoffset={31.4 * (1 - installProgress / 100)} />
                  </svg>
                {:else if installStage === 'installing' || installStage === 'installing_runtime'}
                  <svg class="progress-ring progress-ring-pulse" width="14" height="14" viewBox="0 0 14 14">
                    <circle class="progress-ring-bg" cx="7" cy="7" r="5" />
                    <circle class="progress-ring-fill" cx="7" cy="7" r="5"
                      stroke-dasharray={31.4}
                      stroke-dashoffset={31.4 * 0.25} />
                  </svg>
                {:else}
                  <span class="spinner"></span>
                {/if}
              {:else if (b.canInstall || b.unavailableReason === 'not_compiled') && !(b.compiled && b.systemAvailable)}
                <!-- Download icon for installable backends -->
                <svg class="backend-icon backend-icon-download" width="12" height="12" viewBox="0 0 12 12">
                  <path d="M6 2v6M3.5 5.5L6 8l2.5-2.5M2 10h8" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              {/if}
              <!-- Name + hardware tag -->
              <span class="backend-name">{b.name}</span>
              {#if b.id !== 'auto' && b.id !== 'cpu'}
                <span class="backend-hw-tag" class:hw-nvidia={b.id === 'cuda'} class:hw-universal={b.id === 'vulkan'} class:hw-apple={b.id === 'metal'}>{backendHardwareLabel(b.id)}</span>
              {/if}
              <!-- Recommended star -->
              {#if b.recommended}
                <span class="backend-star" title={t(displayLang, 'backendRecommended')}>&#9733;</span>
              {/if}
              <!-- Auto-resolved indicator -->
              {#if autoResolvedId === b.id && b.id !== 'auto'}
                <span class="backend-auto-badge">&#8592; auto</span>
              {/if}
              <!-- Download size -->
              {#if !b.compiled && b.downloadSizeMB > 0 && installingBackend !== b.id}
                <span class="backend-size">~{b.downloadSizeMB}{t(displayLang, 'mb')}</span>
              {/if}
            </button>
          {/each}
        </div>
        <!-- Recommendation line -->
        {#if recommendedBackend && !recommendedBackend.compiled}
          <div class="backend-recommend-line">
            &#9733; {t(displayLang, 'backendRecommendedHint')}{#if recommendedBackend.gpuDetected}: {recommendedBackend.gpuDetected}{/if}
          </div>
        {/if}
        <!-- CUDA driver hint — clickable link to NVIDIA driver page -->
        {#if visibleBackends.some(b => b.installHint === 'cuda_driver_525' && !b.compiled)}
          <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
          <div class="backend-driver-link" on:click={() => window.open('https://www.nvidia.com/download/index.aspx')}>
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M6 1v4.5m0 0L3.5 3M6 5.5L8.5 3M1 8l1.5 2h7L11 8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            {t(displayLang, 'backendCudaDriverHint')}
            <svg width="10" height="10" viewBox="0 0 10 10" fill="none" style="opacity:0.6">
              <path d="M3 1h6v6M9 1L4 6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </div>
        {/if}
        <!-- Download progress / messages -->
        {#if (installStage === 'downloading' || installStage === 'downloading_runtime') && installProgress !== null}
          <div class="backend-message">{t(displayLang, installStage === 'downloading_runtime' ? 'backendDownloadingRuntime' : 'backendDownloading')} {Math.round(installProgress)}%</div>
        {:else if installStage === 'installing' || installStage === 'installing_runtime'}
          <div class="backend-message">{installStageText || t(displayLang, 'backendInstalling')}</div>
        {:else if backendMessage}
          <div class="backend-message">
            {backendMessage}
            {#if showRestartButton}
              <button class="restart-btn" on:click={handleRestart}>{t(displayLang, 'backendRestart')}</button>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Theme -->
      <div class="field" title={t(displayLang, 'tip_theme')}>
        <!-- svelte-ignore a11y-label-has-associated-control -->
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
        <label class="field-label" for="settings-ui-lang">{t(displayLang, 'uiLanguage')}</label>
        <select id="settings-ui-lang" class="field-select" bind:value={localLang}>
          {#each langOptions as opt}
            <option value={opt.code}>{opt.label}</option>
          {/each}
        </select>
      </div>

      <!-- Close Action -->
      <div class="field" title={t(displayLang, 'tip_closeAction')}>
        <!-- svelte-ignore a11y-label-has-associated-control -->
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

      <!-- Auto Start + Start Minimized -->
      <div class="field-row">
        <div class="field" title={t(displayLang, 'tip_autoStart')}>
          <!-- svelte-ignore a11y-label-has-associated-control -->
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
        <div class="field" title={t(displayLang, 'tip_startMinimized')}>
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(displayLang, 'startMinimized')}</label>
          <div class="pill-group">
            <button
              class="pill-btn"
              class:pill-active={localStartMinimized}
              on:click={() => localStartMinimized = true}
            >{t(displayLang, 'on')}</button>
            <button
              class="pill-btn"
              class:pill-active={!localStartMinimized}
              on:click={() => localStartMinimized = false}
            >{t(displayLang, 'off')}</button>
          </div>
        </div>
      </div>

      <!-- Microphone -->
      <div class="field" title={t(displayLang, 'tip_microphone')}>
        <label class="field-label" for="settings-mic">{t(displayLang, 'microphone')}</label>
        <select id="settings-mic" class="field-select" bind:value={localMicId}>
          <option value="">{t(displayLang, 'default_mic')}</option>
          {#each microphones as mic}
            <option value={mic.id}>{mic.name}{mic.isDefault ? ' *' : ''}</option>
          {/each}
        </select>
      </div>

      <!-- Models Directory -->
      <div class="field" title={t(displayLang, 'tip_modelsDir')}>
        <!-- svelte-ignore a11y-label-has-associated-control -->
        <label class="field-label">{t(displayLang, 'modelsDirectory')}</label>
        <div class="dir-row">
          <input class="dir-input" type="text" readonly value={localModelsDir} />
          <button class="browse-btn" on:click={handleBrowse} title={t(displayLang, 'tip_browse')}>{t(displayLang, 'browse')}</button>
        </div>
      </div>

      <!-- Models -->
      <div class="field">
        <!-- svelte-ignore a11y-label-has-associated-control -->
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

  /* Auto-resolved backend highlight */
  .backend-pill.backend-auto-resolved {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    border-style: solid;
    opacity: 1;
  }

  /* "← auto" badge */
  .backend-auto-badge {
    font-size: 8px;
    opacity: 0.5;
    font-style: italic;
  }

  /* Backend status icons */
  .backend-icon { flex-shrink: 0; }
  .backend-icon-download { color: var(--accent); opacity: 0.7; }

  /* Hardware type tags */
  .backend-hw-tag {
    font-size: 9px;
    font-weight: 600;
    letter-spacing: 0.04em;
    padding: 1px 4px;
    border-radius: 3px;
    text-transform: uppercase;
    line-height: 1;
  }
  .hw-nvidia { background: #76b900; color: #fff; }
  .hw-universal { background: #3b82f6; color: #fff; }
  .hw-apple { background: #a3a3a3; color: #fff; }

  /* Recommended star */
  .backend-star {
    color: #f59e0b;
    font-size: 11px;
    line-height: 1;
  }

  /* Download size badge */
  .backend-size {
    font-size: 9px;
    opacity: 0.6;
    font-family: ui-monospace, monospace;
  }

  /* Recommendation line */
  .backend-recommend-line {
    font-size: 11px;
    color: #f59e0b;
    padding: 3px 0 0;
  }

  .backend-driver-link {
    font-size: 11px;
    color: var(--accent);
    padding: 4px 0 0;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    text-decoration: underline;
    text-decoration-style: dashed;
    text-underline-offset: 2px;
    transition: opacity 0.2s;
  }
  .backend-driver-link:hover {
    opacity: 0.8;
  }

  .backend-message {
    font-size: 11px;
    color: var(--accent);
    font-family: ui-monospace, monospace;
    padding: 2px 0;
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .restart-btn {
    font-size: 11px;
    font-family: ui-monospace, monospace;
    padding: 2px 10px;
    border-radius: 4px;
    border: 1px solid var(--accent);
    background: transparent;
    color: var(--accent);
    cursor: pointer;
    transition: background 0.2s, color 0.2s;
  }
  .restart-btn:hover {
    background: var(--accent);
    color: var(--bg-page);
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

  /* Circular progress ring for download */
  .progress-ring { transform: rotate(-90deg); }
  .progress-ring-bg {
    fill: none;
    stroke: var(--text-muted);
    stroke-width: 2;
    opacity: 0.3;
  }
  .progress-ring-fill {
    fill: none;
    stroke: var(--accent);
    stroke-width: 2;
    stroke-linecap: round;
    transition: stroke-dashoffset 0.2s ease;
  }
  .progress-ring-pulse {
    animation: pulse-ring 1.2s ease-in-out infinite;
  }
  @keyframes pulse-ring {
    0%, 100% { opacity: 0.5; }
    50% { opacity: 1; }
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

  .field-row {
    display: flex;
    gap: 16px;
  }
  .field-row > .field {
    flex: 1;
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
