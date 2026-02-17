<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { SaveGlobalSettings, InstallBackend } from '../../bindings/github.com/UberMorgott/transcribation/services/settingsservice.js';
  import { CreatePreset } from '../../bindings/github.com/UberMorgott/transcribation/services/presetservice.js';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';
  import HotkeyCapture from './HotkeyCapture.svelte';

  export let microphones: { id: string; name: string; isDefault: boolean }[] = [];
  export let backends: { id: string; name: string; compiled: boolean; systemAvailable: boolean; canInstall: boolean; installHint: string; unavailableReason: string; gpuDetected: string; recommended: boolean; downloadSizeMB: number }[] = [];
  export let models: { name: string; fileName: string; size: string; sizeBytes: number; downloaded: boolean }[] = [];
  export let settings: { microphoneId: string; modelsDir: string; theme: string; uiLang: string; closeAction: string; autoStart: boolean; startMinimized: boolean; backend: string; onboardingDone: boolean };

  const dispatch = createEventDispatcher<{
    done: { uiLang: string; theme: string; backend: string; microphoneId: string };
    openModels: void;
  }>();

  const TOTAL_STEPS = 4;
  let step = 1;

  // Step 1
  let uiLang: Lang = (settings.uiLang as Lang) || 'en';
  let theme: 'dark' | 'light' = settings.theme === 'light' ? 'light' : 'dark';

  // Step 2
  let microphoneId = settings.microphoneId || '';

  // Step 3 — backend selection + install state
  let backend = settings.backend || 'auto';

  type InstallStatus = 'idle' | 'downloading' | 'done' | 'error';
  type InstallState = {
    status: InstallStatus;
    stageText: string;
    percent: number;
    error: string;
  };
  let installStates: Record<string, InstallState> = {};
  let infoOpenId: string | null = null; // id бэкенда, у которого открыта инфо-панель
  let nextWarning = false;

  // Step 4 — preset
  let presetName = '';
  let presetHotkey = '';
  let presetModel = '';
  let presetInputMode: 'hold' | 'toggle' = 'hold';
  let presetLanguage = 'auto';
  let presetCapturing = false;

  const TRANSCRIPTION_LANGS = [
    { code: 'auto', label: 'Auto' },
    { code: 'en',   label: 'English' },
    { code: 'ru',   label: 'Русский' },
    { code: 'de',   label: 'Deutsch' },
    { code: 'es',   label: 'Español' },
    { code: 'fr',   label: 'Français' },
    { code: 'zh',   label: '中文' },
    { code: 'ja',   label: '日本語' },
    { code: 'pt',   label: 'Português' },
    { code: 'ko',   label: '한국어' },
  ];

  const LANGS: { code: Lang; label: string }[] = [
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

  $: downloadedModels = models.filter(m => m.downloaded);
  $: if (downloadedModels.length > 0 && !presetModel) {
    presetModel = downloadedModels[0].name;
  }

  // GPU backends: hardware present or can install runtime (exclude CPU/auto)
  $: gpuBackends = backends.filter(b =>
    b.id !== 'cpu' && b.id !== 'auto' &&
    b.unavailableReason !== 'no_hardware'
  );
  $: cpuBackend = backends.find(b => b.id === 'cpu');

  // Is any install running right now?
  $: isDownloading = Object.values(installStates).some(s => s.status === 'downloading');

  // Subscribe to backend install progress events
  let unsubProgress: (() => void) | null = null;
  onMount(() => {
    unsubProgress = Events.On('backend:install:progress', (event: any) => {
      // Wails v3 wraps emit args as array in event.data; handle all formats defensively
      const raw = event?.data;
      const data = Array.isArray(raw) ? raw[0] : (raw ?? event);
      const id: string = data?.backendId;
      if (!id) return;

      if (data.done) {
        if (data.error) {
          installStates[id] = { status: 'error', stageText: '', percent: 0, error: data.error };
        } else {
          installStates[id] = { status: 'done', stageText: '', percent: 100, error: '' };
          // auto-select this backend
          backend = id;
        }
      } else {
        const stageText = data.stageText || (data.stage === 'downloading_runtime'
          ? t(uiLang, 'onboarding_installing_runtime')
          : t(uiLang, 'onboarding_downloading'));
        installStates[id] = {
          status: 'downloading',
          stageText,
          percent: data.percent || 0,
          error: '',
        };
      }
      installStates = installStates;
    });
  });

  onDestroy(() => {
    if (unsubProgress) unsubProgress();
  });

  function applyTheme(th: 'dark' | 'light') {
    theme = th;
    document.documentElement.setAttribute('data-theme', theme);
    try { localStorage.setItem('morgottalk-theme', theme); } catch {}
  }

  async function saveSettings(done = false) {
    try {
      await SaveGlobalSettings({
        microphoneId,
        modelsDir: settings.modelsDir,
        theme,
        uiLang,
        closeAction: settings.closeAction,
        autoStart: settings.autoStart,
        startMinimized: settings.startMinimized,
        backend,
        onboardingDone: done,
      });
    } catch {}
  }

  async function next() {
    if (step === 3 && isDownloading) {
      nextWarning = true;
      return;
    }
    nextWarning = false;
    await saveSettings(false);
    if (step < TOTAL_STEPS) step++;
  }

  function back() {
    nextWarning = false;
    if (step > 1) step--;
  }

  async function finish() {
    const name = presetName.trim() || t(uiLang, 'onboarding_step4_name_placeholder');
    try {
      await CreatePreset({
        id: '',
        name,
        modelName: presetModel,
        hotkey: presetHotkey,
        language: presetLanguage,
        inputMode: presetInputMode,
        keepModelLoaded: false,
        keepHistory: true,
        useKBLayout: false,
        enabled: presetHotkey !== '',
      });
    } catch {}
    await saveSettings(true);
    dispatch('done', { uiLang, theme, backend, microphoneId });
  }

  // --- Backend install flow ---

  function getState(id: string): InstallState {
    return installStates[id] || { status: 'idle', stageText: '', percent: 0, error: '' };
  }

  function openInfo(id: string) {
    infoOpenId = infoOpenId === id ? null : id;
  }

  async function startInstall(id: string) {
    infoOpenId = null;
    installStates[id] = { status: 'downloading', stageText: t(uiLang, 'onboarding_downloading'), percent: 0, error: '' };
    installStates = installStates;
    try {
      await InstallBackend(id);
      // InstallBackend returns "installing" immediately; actual progress comes via events.
      // Do NOT set done here — the event handler will set it when the download finishes.
    } catch (e: any) {
      installStates[id] = { status: 'error', stageText: '', percent: 0, error: String(e) };
      installStates = installStates;
    }
  }

  function gpuStatusText(b: typeof backends[0]): string {
    const state = getState(b.id);
    if (state.status === 'done') return t(uiLang, 'onboarding_gpu_ready');
    if (state.status === 'error') return t(uiLang, 'onboarding_install_error').replace('{msg}', state.error);
    if (state.status === 'downloading') return state.stageText || t(uiLang, 'onboarding_downloading');
    if (b.compiled && b.systemAvailable) return t(uiLang, 'onboarding_gpu_ready');
    if (b.systemAvailable && !b.compiled) return t(uiLang, 'onboarding_gpu_download').replace('{size}', String(b.downloadSizeMB || '?'));
    if (b.canInstall && !b.systemAvailable) return t(uiLang, 'onboarding_gpu_install_runtime');
    if (b.unavailableReason === 'no_driver') return t(uiLang, 'onboarding_gpu_bad_driver');
    return t(uiLang, 'onboarding_gpu_no_hw');
  }

  function speedLabel(id: string): string {
    switch (id) {
      case 'cpu':    return t(uiLang, 'onboarding_speed_slow');
      case 'vulkan': return t(uiLang, 'onboarding_speed_fast');
      case 'cuda':   return t(uiLang, 'onboarding_speed_vfast');
      case 'rocm':   return t(uiLang, 'onboarding_speed_fast');
      case 'metal':  return t(uiLang, 'onboarding_speed_vfast');
      default:       return t(uiLang, 'onboarding_speed_medium');
    }
  }

  function hwLabel(b: typeof backends[0]): string {
    if (b.id === 'cuda') return b.gpuDetected || t(uiLang, 'onboarding_hw_nvidia');
    if (b.id === 'vulkan') return b.gpuDetected || t(uiLang, 'onboarding_hw_anygpu');
    return b.gpuDetected || b.name;
  }

  // OS-aware platform id list to exclude
  // Go already filters by OS, but guard against unexpected items
  const osExclude: string[] = (() => {
    const ua = navigator.userAgent.toLowerCase();
    if (ua.includes('win')) return ['metal', 'rocm'];     // macOS/Linux only
    if (ua.includes('mac')) return ['rocm'];               // Linux only
    return ['metal'];                                       // Linux: hide Metal
  })();
  $: visibleGpuBackends = gpuBackends.filter(b => !osExclude.includes(b.id));

  function stepLabel(n: number): string {
    return t(uiLang, 'onboarding_step_of')
      .replace('{n}', String(n))
      .replace('{total}', String(TOTAL_STEPS));
  }

  // ── SVG icon set (20×20, consistent stroke style) ──
  const S = 'stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"';
  const iconSvg: Record<string, string> = {
    // CPU chip with pins
    cpu: `<svg viewBox="0 0 20 20" fill="none" ${S}><rect x="5" y="5" width="10" height="10" rx="1.5"/><rect x="7.5" y="7.5" width="5" height="5" rx="0.5"/><path d="M7.5 5V2.5M10 5V2.5M12.5 5V2.5M7.5 15v2.5M10 15v2.5M12.5 15v2.5M5 7.5H2.5M5 10H2.5M5 12.5H2.5M15 7.5h2.5M15 10h2.5M15 12.5h2.5"/></svg>`,
    // NVIDIA eye logo (simplified)
    cuda: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M2 10c2.5-5.5 13.5-5.5 16 0-2.5 5.5-13.5 5.5-16 0z"/><circle cx="10" cy="10" r="3"/><circle cx="10" cy="10" r="1.3" fill="currentColor" stroke="none"/></svg>`,
    // Vulkan angular V
    vulkan: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M3 4L10 17L17 4"/><path d="M7 4L10 9.5L13 4"/></svg>`,
    // Generic GPU card (ROCm, Metal, others)
    gpu: `<svg viewBox="0 0 20 20" fill="none" ${S}><rect x="1" y="6" width="14.5" height="8.5" rx="1.5"/><rect x="3" y="8" width="4" height="4" rx="0.5"/><circle cx="12" cy="10.25" r="2"/><path d="M15.5 8h3M15.5 10.25h3M15.5 12.5h3"/></svg>`,
    // Download arrow into tray
    download: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M10 3v9M7 9l3 3 3-3"/><path d="M3 14.5v1a1 1 0 001 1h12a1 1 0 001-1v-1"/></svg>`,
  };

  function backendIcon(id: string): string {
    if (id === 'cuda') return iconSvg.cuda;
    if (id === 'vulkan') return iconSvg.vulkan;
    return iconSvg.gpu;
  }
</script>

<div class="overlay">
  <div class="card">
    <!-- Header -->
    <div class="card-header">
      <div class="app-logo">Morgo<span class="tt">TT</span>alk</div>
      <div class="step-label">{stepLabel(step)}</div>
    </div>

    <!-- Progress bar -->
    <div class="progress-track">
      <div class="progress-fill" style="width: {(step / TOTAL_STEPS) * 100}%"></div>
    </div>

    <!-- Step content -->
    <div class="card-body">

      <!-- ── Step 1: Language & Theme ── -->
      {#if step === 1}
        <h2 class="step-title">{t(uiLang, 'onboarding_step1_title')}</h2>
        <p class="step-hint">{t(uiLang, 'onboarding_step1_hint')}</p>

        <div class="field">
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(uiLang, 'uiLanguage')}</label>
          <div class="lang-grid">
            {#each LANGS as l}
              <button class="lang-pill" class:active={uiLang === l.code} on:click={() => uiLang = l.code}>
                {l.label}
              </button>
            {/each}
          </div>
        </div>

        <div class="field">
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(uiLang, 'theme')}</label>
          <div class="pill-row">
            <button class="pill" class:active={theme === 'dark'} on:click={() => applyTheme('dark')}>
              {t(uiLang, 'dark')}
            </button>
            <button class="pill" class:active={theme === 'light'} on:click={() => applyTheme('light')}>
              {t(uiLang, 'light')}
            </button>
          </div>
        </div>

      <!-- ── Step 2: Microphone ── -->
      {:else if step === 2}
        <h2 class="step-title">{t(uiLang, 'onboarding_step2_title')}</h2>
        <p class="step-hint">{t(uiLang, 'onboarding_step2_hint')}</p>

        <div class="field">
          <label for="mic-select" class="field-label">{t(uiLang, 'microphone')}</label>
          <select id="mic-select" class="select" bind:value={microphoneId}>
            <option value="">{t(uiLang, 'default_mic')}</option>
            {#each microphones as mic}
              <option value={mic.id}>{mic.name}{mic.isDefault ? ' ★' : ''}</option>
            {/each}
          </select>
        </div>

      <!-- ── Step 3: GPU Acceleration ── -->
      {:else if step === 3}
        <h2 class="step-title">{t(uiLang, 'onboarding_step3_title')}</h2>
        <p class="step-hint">{t(uiLang, 'onboarding_step3_hint')}</p>

        <div class="acc-list">

          <!-- GPU backends (only visible if hardware exists, current OS only) -->
          {#each visibleGpuBackends as b}
            {@const state = getState(b.id)}
            {@const isReady = (b.compiled && b.systemAvailable) || state.status === 'done'}
            {@const needsDL = b.systemAvailable && !b.compiled && state.status === 'idle'}
            {@const needsInstall = b.canInstall && !b.systemAvailable && state.status === 'idle'}
            {@const badDriver = b.unavailableReason === 'no_driver'}
            {@const downloading = state.status === 'downloading'}
            {@const done = state.status === 'done'}
            {@const hasError = state.status === 'error'}
            {@const showInfo = infoOpenId === b.id && !downloading}

            <div class="acc-card" class:selected={backend === b.id} class:downloading>
              <!-- Card header row -->
              <div class="acc-card-header"
                on:click={() => {
                  if (isReady || done) { backend = b.id; infoOpenId = null; }
                  else if ((needsDL || needsInstall) && !downloading) { openInfo(b.id); }
                }}
                role="button" tabindex="0"
                on:keydown={(e) => { if (e.key === 'Enter') {
                  if (isReady || done) { backend = b.id; infoOpenId = null; }
                  else if (needsDL || needsInstall) { openInfo(b.id); }
                }}}>
                <div class="acc-icon">{@html backendIcon(b.id)}</div>
                <div class="acc-info">
                  <div class="acc-name">
                    {b.name}
                    <span class="badge-speed">{speedLabel(b.id)}</span>
                    {#if b.recommended && (isReady || needsDL || needsInstall)}
                      <span class="badge-rec">{t(uiLang, 'backendRecommended')}</span>
                    {/if}
                    {#if done || (b.compiled && b.systemAvailable)}
                      <span class="badge-ok">✓</span>
                    {/if}
                  </div>
                  <div class="acc-gpu">{hwLabel(b)}</div>
                  <div class="acc-status" class:status-ready={isReady || done} class:status-error={hasError}>
                    {gpuStatusText(b)}
                  </div>
                </div>
                <!-- Right: spinner | radio dot | download icon -->
                {#if downloading}
                  <div class="acc-right-icon">
                    <svg class="spin-anim" viewBox="0 0 20 20" fill="none">
                      <circle cx="10" cy="10" r="7.5" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-dasharray="14 33"/>
                    </svg>
                  </div>
                {:else if isReady || done}
                  <div class="acc-select-dot" class:dot-active={backend === b.id}></div>
                {:else if needsDL || needsInstall}
                  <!-- ⬇ icon: immediately starts download -->
                  <button class="acc-dl-icon"
                    title={needsDL ? t(uiLang, 'onboarding_download_btn') : t(uiLang, 'onboarding_install_btn')}
                    on:click|stopPropagation={() => startInstall(b.id)}>
                    {@html iconSvg.download}
                  </button>
                {/if}
              </div>

              <!-- Progress bar (persists across step navigation) -->
              {#if downloading}
                <div class="dl-progress">
                  <div class="dl-bar">
                    <div class="dl-fill" style="width: {state.percent}%"></div>
                  </div>
                  <div class="dl-meta">
                    <span>{state.stageText}</span>
                    <span>{Math.round(state.percent)}%</span>
                  </div>
                </div>
              {/if}

              <!-- Info panel: shown when user clicks the card body (not ⬇ icon) -->
              {#if showInfo}
                <div class="info-panel">
                  <p class="info-text">
                    {needsInstall
                      ? t(uiLang, 'onboarding_gpu_install_runtime')
                      : t(uiLang, 'onboarding_gpu_download').replace('{size}', String(b.downloadSizeMB || '?'))}
                  </p>
                  <div class="info-btns">
                    <button class="btn-cancel" on:click={() => infoOpenId = null}>
                      {t(uiLang, 'onboarding_cancel')}
                    </button>
                    <button class="btn-ok" on:click={() => startInstall(b.id)}>
                      {needsInstall ? t(uiLang, 'onboarding_install_btn') : t(uiLang, 'onboarding_download_btn')}
                    </button>
                  </div>
                </div>
              {/if}

              <!-- Error: show text + retry -->
              {#if hasError}
                <div class="error-row">
                  <span class="error-text">{state.error}</span>
                  <button class="btn-retry" on:click={() => startInstall(b.id)}>
                    {t(uiLang, 'onboarding_download_btn')}
                  </button>
                </div>
              {/if}
            </div>
          {/each}

          <!-- CPU — always available -->
          {#if cpuBackend}
            <div class="acc-card" class:selected={backend === 'cpu' || (gpuBackends.length === 0)}
              on:click={() => backend = 'cpu'}
              role="button" tabindex="0"
              on:keydown={(e) => e.key === 'Enter' && (backend = 'cpu')}>
              <div class="acc-card-header">
                <div class="acc-icon">{@html iconSvg.cpu}</div>
                <div class="acc-info">
                  <div class="acc-name">
                    {cpuBackend.name}
                    <span class="badge-speed speed-slow">{speedLabel('cpu')}</span>
                  </div>
                  <div class="acc-status status-ready">{t(uiLang, 'onboarding_cpu_desc')}</div>
                </div>
                <div class="acc-select-dot" class:dot-active={backend === 'cpu' || gpuBackends.length === 0}></div>
              </div>
            </div>
          {/if}
        </div>

        <!-- "Next during download" warning banner -->
        {#if nextWarning}
          <div class="next-warning">
            <strong>{t(uiLang, 'onboarding_next_warning_title')}</strong>
            <p>{t(uiLang, 'onboarding_next_warning_body')}</p>
            <button class="btn-ok" on:click={() => { nextWarning = false; saveSettings(false); step++; }}>
              {t(uiLang, 'onboarding_next_ok')}
            </button>
          </div>
        {/if}

      <!-- ── Step 4: First Preset ── -->
      {:else if step === 4}
        <h2 class="step-title">{t(uiLang, 'onboarding_step4_title')}</h2>
        <p class="step-hint">{t(uiLang, 'onboarding_step4_hint')}</p>

        <div class="field">
          <label for="preset-name" class="field-label">{t(uiLang, 'name')}</label>
          <input
            id="preset-name"
            class="input"
            type="text"
            placeholder={t(uiLang, 'onboarding_step4_name_placeholder')}
            bind:value={presetName}
          />
        </div>

        <div class="field">
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(uiLang, 'hotkey')}</label>
          <HotkeyCapture bind:value={presetHotkey} bind:capturing={presetCapturing} lang={uiLang} />
        </div>

        <div class="field">
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(uiLang, 'model')}</label>
          {#if downloadedModels.length === 0}
            <div class="model-empty">
              <span class="model-empty-text">{t(uiLang, 'onboarding_model_none')}</span>
              <button class="btn-link" on:click={() => dispatch('openModels')}>
                {t(uiLang, 'manageModels')} →
              </button>
            </div>
          {:else}
            <select class="select" bind:value={presetModel}>
              {#each downloadedModels as m}
                <option value={m.name}>{m.name} ({m.size})</option>
              {/each}
            </select>
          {/if}
        </div>

        <div class="field">
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(uiLang, 'inputMode')}</label>
          <div class="pill-row">
            <button class="pill" class:active={presetInputMode === 'hold'}
              on:click={() => presetInputMode = 'hold'}
              title={t(uiLang, 'tip_inputMode')}>
              {t(uiLang, 'hold')}
            </button>
            <button class="pill" class:active={presetInputMode === 'toggle'}
              on:click={() => presetInputMode = 'toggle'}
              title={t(uiLang, 'tip_inputMode')}>
              {t(uiLang, 'toggle')}
            </button>
          </div>
        </div>

        <div class="field">
          <label for="preset-lang" class="field-label">{t(uiLang, 'language')}</label>
          <select id="preset-lang" class="select" bind:value={presetLanguage}>
            {#each TRANSCRIPTION_LANGS as l}
              <option value={l.code}>{l.label}</option>
            {/each}
          </select>
        </div>
      {/if}
    </div>

    <!-- Footer -->
    <div class="card-footer">
      {#if step > 1}
        <button class="btn-back" on:click={back}>{t(uiLang, 'onboarding_back')}</button>
      {:else}
        <div></div>
      {/if}

      {#if step < TOTAL_STEPS}
        <button class="btn-next" on:click={next}>
          {t(uiLang, 'onboarding_next')} →
          {#if step === 3 && isDownloading}
            <span class="btn-dl-badge">⟳</span>
          {/if}
        </button>
      {:else}
        <button class="btn-finish" on:click={finish}>{t(uiLang, 'onboarding_finish')}</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 200;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0,0,0,0.72);
    backdrop-filter: blur(4px);
  }

  .card {
    width: 420px;
    max-height: 92vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-page);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    overflow: hidden;
    box-shadow: 0 24px 64px rgba(0,0,0,0.55);
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 18px 24px 10px;
    flex-shrink: 0;
  }
  .app-logo { font-size: 17px; font-weight: 700; color: var(--text-primary); letter-spacing: .02em; }
  .tt { color: var(--accent); }
  .step-label { font-size: 11px; color: var(--text-tertiary); font-family: ui-monospace, monospace; }

  .progress-track { height: 2px; background: var(--border-subtle, rgba(255,255,255,.07)); flex-shrink: 0; }
  .progress-fill { height: 100%; background: var(--accent); transition: width .3s ease; }

  .card-body {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    min-height: 0;
  }
  .card-body::-webkit-scrollbar { width: 3px; }
  .card-body::-webkit-scrollbar-track { background: transparent; }
  .card-body::-webkit-scrollbar-thumb { background: var(--border-subtle); border-radius: 3px; }

  .step-title { font-size: 17px; font-weight: 600; color: var(--text-primary); margin: 0; }
  .step-hint { font-size: 13px; color: var(--text-tertiary); margin: -10px 0 0; line-height: 1.5; }

  .field { display: flex; flex-direction: column; gap: 7px; }
  .field-label { font-size: 11px; font-weight: 500; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .06em; }

  /* Lang grid */
  .lang-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 5px; }
  .lang-pill {
    padding: 7px 4px; border-radius: 7px;
    border: 1px solid var(--toggle-border, var(--border-color));
    background: transparent; color: var(--text-muted, var(--text-tertiary));
    font-size: 13px; cursor: pointer; text-align: center; transition: all .14s;
  }
  .lang-pill:hover { color: var(--text-secondary); border-color: var(--border-hover); }
  .lang-pill.active { background: var(--accent-dim); border-color: var(--accent); color: var(--accent); font-weight: 500; }

  /* Pill row */
  .pill-row { display: flex; gap: 6px; }
  .pill {
    flex: 1; padding: 7px;
    border-radius: 7px; border: 1px solid var(--toggle-border, var(--border-color));
    background: transparent; color: var(--text-muted, var(--text-tertiary));
    font-size: 13px; cursor: pointer; transition: all .14s;
  }
  .pill:hover { color: var(--text-secondary); border-color: var(--border-hover); }
  .pill.active { background: var(--accent-dim); border-color: var(--accent); color: var(--accent); font-weight: 500; }

  /* Inputs */
  .select, .input {
    width: 100%; padding: 8px 10px; border-radius: 8px;
    border: 1px solid var(--toggle-border, var(--border-color));
    background: var(--bg-input, var(--bg-page));
    color: var(--text-secondary); font-size: 14px; outline: none;
    box-sizing: border-box; transition: border-color .14s;
  }
  .select:focus, .input:focus { border-color: var(--accent); }

  /* ── Acceleration cards ── */
  .acc-list { display: flex; flex-direction: column; gap: 8px; }

  .acc-card {
    border: 1px solid var(--border-color);
    border-radius: 10px;
    overflow: hidden;
    transition: border-color .15s;
    cursor: default;
  }
  .acc-card.selected { border-color: var(--accent); }
  .acc-card.downloading { border-color: var(--accent); }

  .acc-card-header {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 14px;
    cursor: pointer;
    transition: background .12s;
  }
  .acc-card-header:hover { background: var(--accent-dim); }

  .acc-icon {
    width: 20px; height: 20px;
    flex-shrink: 0; margin-top: 1px;
    color: var(--text-tertiary);
    display: flex; align-items: center; justify-content: center;
  }
  .acc-icon :global(svg) { width: 20px; height: 20px; display: block; }

  .acc-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
  .acc-name {
    font-size: 14px; font-weight: 600; color: var(--text-primary);
    display: flex; align-items: center; gap: 6px;
  }
  .acc-gpu { font-size: 12px; color: var(--text-tertiary); }
  .acc-status { font-size: 12px; color: var(--text-tertiary); margin-top: 2px; }
  .acc-status.status-ready { color: #34d399; }
  .acc-status.status-error { color: var(--accent-red, #ef4444); }

  .badge-rec {
    font-size: 10px; padding: 1px 5px; border-radius: 4px;
    background: var(--accent-dim); color: var(--accent);
    font-weight: 600; letter-spacing: .04em;
  }
  .badge-ok {
    font-size: 11px; color: #34d399;
  }
  .badge-speed {
    font-size: 10px; padding: 1px 5px; border-radius: 4px;
    background: rgba(255,255,255,.06);
    color: var(--text-tertiary);
    font-weight: 500; letter-spacing: .03em;
  }
  .speed-slow { background: rgba(255,255,255,.05); }

  .acc-select-dot {
    width: 16px; height: 16px; border-radius: 50%;
    border: 2px solid var(--border-color);
    flex-shrink: 0; margin-top: 2px;
    transition: all .15s;
  }
  .dot-active { border-color: var(--accent); background: var(--accent); }

  /* Download icon button */
  .acc-dl-icon {
    background: transparent; border: none;
    color: var(--accent);
    width: 28px; height: 28px;
    display: flex; align-items: center; justify-content: center;
    cursor: pointer; flex-shrink: 0;
    border-radius: 6px; padding: 0;
    transition: opacity .14s, background .12s;
  }
  .acc-dl-icon:hover { opacity: .7; background: var(--accent-dim); }
  .acc-dl-icon :global(svg) { width: 20px; height: 20px; display: block; }

  /* Spinner for downloading state */
  .acc-right-icon {
    width: 20px; height: 20px; flex-shrink: 0;
    margin-top: 2px; color: var(--accent);
    display: flex; align-items: center; justify-content: center;
  }
  .acc-right-icon :global(svg) { width: 20px; height: 20px; display: block; }
  .spin-anim { animation: spin 0.9s linear infinite; }

  /* Progress inside card */
  .dl-progress { padding: 0 14px 12px; }
  .dl-bar { height: 4px; background: var(--border-subtle, rgba(255,255,255,.07)); border-radius: 2px; overflow: hidden; }
  .dl-fill { height: 100%; background: var(--accent); border-radius: 2px; transition: width .3s ease; }
  .dl-meta { display: flex; justify-content: space-between; font-size: 11px; color: var(--text-tertiary); margin-top: 4px; }


  /* Info panel (opens when card body is clicked) */
  .info-panel {
    padding: 10px 14px 12px;
    border-top: 1px solid var(--border-subtle, rgba(255,255,255,.06));
    background: var(--bg-input, rgba(255,255,255,.03));
    display: flex; flex-direction: column; gap: 8px;
  }
  .info-text {
    font-size: 12px; color: var(--text-secondary); margin: 0; line-height: 1.5;
  }
  .info-btns { display: flex; gap: 8px; }
  .btn-cancel {
    flex: 1; padding: 7px; border-radius: 7px;
    border: 1px solid var(--border-color); background: transparent;
    color: var(--text-secondary); font-size: 13px; cursor: pointer; transition: all .14s;
  }
  .btn-cancel:hover { border-color: var(--border-hover); }
  .btn-ok {
    flex: 1; padding: 7px; border-radius: 7px;
    background: var(--accent); border: none;
    color: #fff; font-size: 13px; font-weight: 600;
    cursor: pointer; transition: opacity .14s;
  }
  .btn-ok:hover { opacity: .85; }

  /* Error row */
  .error-row {
    display: flex; align-items: center; gap: 8px;
    padding: 6px 14px 10px; flex-wrap: wrap;
  }
  .error-text { font-size: 12px; color: var(--accent-red, #ef4444); flex: 1; }
  .btn-retry {
    padding: 5px 12px; border-radius: 6px;
    background: var(--accent); border: none;
    color: #fff; font-size: 12px; font-weight: 600;
    cursor: pointer; transition: opacity .14s; flex-shrink: 0;
  }
  .btn-retry:hover { opacity: .85; }

  /* "Next during download" warning */
  .next-warning {
    padding: 12px 14px;
    background: var(--bg-input, rgba(255,255,255,.04));
    border: 1px solid var(--accent);
    border-radius: 10px;
    display: flex; flex-direction: column; gap: 6px;
  }
  .next-warning strong { font-size: 13px; color: var(--text-primary); }
  .next-warning p { font-size: 12px; color: var(--text-tertiary); margin: 0; line-height: 1.5; }
  .next-warning .btn-ok { align-self: flex-end; flex: none; padding: 6px 16px; }

  /* Link button (e.g. "Manage Models →") */
  .btn-link {
    background: transparent; border: none; padding: 0;
    color: var(--accent); font-size: 13px; cursor: pointer;
    text-decoration: underline; text-underline-offset: 2px; opacity: .85;
    transition: opacity .14s;
  }
  .btn-link:hover { opacity: 1; }

  /* Model empty */
  .model-empty {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 12px; border-radius: 8px;
    border: 1px dashed var(--border-hover); background: transparent;
  }
  .model-empty-text { font-size: 13px; color: var(--text-tertiary); }

  /* Footer */
  .card-footer {
    flex-shrink: 0;
    display: flex; justify-content: space-between; align-items: center;
    padding: 14px 24px;
    border-top: 1px solid var(--border-subtle, rgba(255,255,255,.06));
    gap: 12px;
  }
  .btn-back {
    background: transparent; border: 1px solid var(--border-color);
    border-radius: 8px; color: var(--text-secondary);
    font-size: 14px; padding: 8px 16px; cursor: pointer; transition: all .15s;
  }
  .btn-back:hover { border-color: var(--border-hover); color: var(--text-primary); }
  .btn-next, .btn-finish {
    background: var(--accent); border: none; border-radius: 8px;
    color: #fff; font-size: 14px; font-weight: 600;
    padding: 8px 20px; cursor: pointer; transition: opacity .15s;
    display: flex; align-items: center; gap: 6px;
  }
  .btn-next:hover, .btn-finish:hover { opacity: .85; }
  .btn-dl-badge { font-size: 12px; opacity: .85; animation: spin 1.2s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
