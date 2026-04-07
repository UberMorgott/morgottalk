<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { SaveGlobalSettings, InstallBackend } from '../../bindings/github.com/UberMorgott/transcribation/services/settingsservice.js';
  import { DownloadModel, GetAvailableModels } from '../../bindings/github.com/UberMorgott/transcribation/services/modelservice.js';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';

  export let microphones: { id: string; name: string; isDefault: boolean }[] = [];
  export let backends: { id: string; name: string; compiled: boolean; systemAvailable: boolean; canInstall: boolean; installHint: string; unavailableReason: string; gpuDetected: string; recommended: boolean; downloadSizeMB: number; runtimeInstalled: boolean }[] = [];
  export let models: { name: string; fileName: string; size: string; sizeBytes: number; downloaded: boolean; description: string; languages: number; speed: number; quality: number; englishOnly: boolean; translation: boolean; category: string }[] = [];
  export let settings: { microphoneId: string; modelsDir: string; theme: string; uiLang: string; closeAction: string; autoStart: boolean; startMinimized: boolean; backend: string; onboardingDone: boolean };

  const dispatch = createEventDispatcher<{
    done: { uiLang: string; theme: string; backend: string; microphoneId: string };
    openModels: void;
  }>();

  const TOTAL_STEPS = 2;
  let step = 1;

  // Step 1: Basic Settings
  let uiLang: Lang = (settings.uiLang as Lang) || 'en';
  let theme: 'dark' | 'light' = settings.theme === 'light' ? 'light' : 'dark';
  let microphoneId = settings.microphoneId || '';

  // Step 2: Downloads
  let backend = settings.backend || 'auto';

  // Model download state
  type ModelDLState = { status: 'idle' | 'downloading' | 'done' | 'error'; percent: number; error: string };
  let modelDownloadStates: Record<string, ModelDLState> = {};
  let selectedModel = '';

  // Backend install state
  type InstallStatus = 'idle' | 'downloading' | 'done' | 'error';
  type InstallStage = '' | 'downloading_runtime' | 'installing_runtime' | 'downloading' | 'installing';
  type InstallState = {
    status: InstallStatus;
    stage: InstallStage;
    stageText: string;
    percent: number;
    error: string;
  };
  let installStates: Record<string, InstallState> = {};
  let infoOpenId: string | null = null;

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
  $: recommendedModels = models.filter(m => m.category && !m.downloaded);

  // Auto-select first downloaded model
  $: if (downloadedModels.length > 0 && !selectedModel) {
    selectedModel = downloadedModels[0].name;
  }

  // GPU backends: hardware present or can install runtime (exclude CPU/auto)
  $: gpuBackends = backends.filter(b =>
    b.id !== 'cpu' && b.id !== 'auto' &&
    b.unavailableReason !== 'no_hardware'
  );
  $: cpuBackend = backends.find(b => b.id === 'cpu');

  // Is any download running right now?
  $: isAnyDownloading =
    Object.values(installStates).some(s => s.status === 'downloading') ||
    Object.values(modelDownloadStates).some(s => s.status === 'downloading');

  // Currently active download for unified progress area
  $: activeModelDL = Object.entries(modelDownloadStates).find(([, s]) => s.status === 'downloading');
  $: activeBackendDL = Object.entries(installStates).find(([, s]) => s.status === 'downloading');

  // Subscribe to events
  let unsubBackendProgress: (() => void) | null = null;
  let unsubModelProgress: (() => void) | null = null;

  onMount(() => {
    unsubBackendProgress = Events.On('backend:install:progress', (event: any) => {
      const raw = event?.data;
      const data = Array.isArray(raw) ? raw[0] : (raw ?? event);
      const id: string = data?.backendId;
      if (!id) return;

      if (data.done) {
        if (data.error) {
          installStates[id] = { status: 'error', stage: '', stageText: '', percent: 0, error: data.error };
        } else {
          installStates[id] = { status: 'done', stage: '', stageText: '', percent: 100, error: '' };
          backend = id;
        }
      } else {
        const rawStage: InstallStage = data.stage || 'downloading';
        const isRuntimeStage = rawStage === 'downloading_runtime' || rawStage === 'installing_runtime';
        const stageText = data.stageText || (isRuntimeStage
          ? t(uiLang, 'onboarding_installing_runtime')
          : t(uiLang, 'onboarding_downloading'));
        installStates[id] = {
          status: 'downloading',
          stage: rawStage,
          stageText,
          percent: data.percent || 0,
          error: '',
        };
      }
      installStates = installStates;
    });

    unsubModelProgress = Events.On('model:download:progress', (event: any) => {
      const data = event.data?.[0] || event.data || event;
      if (!data.modelName) return;

      if (data.done) {
        if (data.error) {
          modelDownloadStates[data.modelName] = { status: 'error', percent: 0, error: data.error };
        } else {
          modelDownloadStates[data.modelName] = { status: 'done', percent: 100, error: '' };
          selectedModel = data.modelName;
          // Refresh models list
          refreshModels();
        }
      } else {
        modelDownloadStates[data.modelName] = {
          status: 'downloading',
          percent: data.percent || 0,
          error: '',
        };
      }
      modelDownloadStates = modelDownloadStates;
    });
  });

  onDestroy(() => {
    if (unsubBackendProgress) unsubBackendProgress();
    if (unsubModelProgress) unsubModelProgress();
  });

  async function refreshModels() {
    try {
      models = await GetAvailableModels();
    } catch {}
  }

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
    await saveSettings(false);
    if (step < TOTAL_STEPS) step++;
  }

  function back() {
    if (step > 1) step--;
  }

  async function skip() {
    await saveSettings(true);
    dispatch('done', { uiLang, theme, backend, microphoneId });
  }

  async function finish() {
    await saveSettings(true);
    dispatch('done', { uiLang, theme, backend, microphoneId });
  }

  // --- Model download ---
  async function startModelDownload(name: string) {
    modelDownloadStates[name] = { status: 'downloading', percent: 0, error: '' };
    modelDownloadStates = modelDownloadStates;
    try {
      await DownloadModel(name);
    } catch (e: any) {
      modelDownloadStates[name] = { status: 'error', percent: 0, error: String(e) };
      modelDownloadStates = modelDownloadStates;
    }
  }

  function getModelState(name: string): ModelDLState {
    return modelDownloadStates[name] || { status: 'idle', percent: 0, error: '' };
  }

  // --- Backend install flow ---
  function getState(id: string): InstallState {
    return installStates[id] || { status: 'idle', stage: '', stageText: '', percent: 0, error: '' };
  }

  function openInfo(id: string) {
    infoOpenId = infoOpenId === id ? null : id;
  }

  async function startInstall(id: string) {
    infoOpenId = null;
    installStates[id] = { status: 'downloading', stage: 'downloading', stageText: t(uiLang, 'onboarding_downloading'), percent: 0, error: '' };
    installStates = installStates;
    try {
      await InstallBackend(id);
    } catch (e: any) {
      installStates[id] = { status: 'error', stage: '', stageText: '', percent: 0, error: String(e) };
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

  function backendExplanation(b: typeof backends[0]): string {
    if (b.id === 'cuda') return t(uiLang, 'onboarding_cuda_explain');
    if (b.id === 'vulkan') return t(uiLang, 'onboarding_vulkan_explain');
    return '';
  }

  // OS-aware platform filter
  const osExclude: string[] = (() => {
    const ua = navigator.userAgent.toLowerCase();
    if (ua.includes('win')) return ['metal', 'rocm'];
    if (ua.includes('mac')) return ['rocm'];
    return ['metal'];
  })();
  $: visibleGpuBackends = gpuBackends.filter(b => !osExclude.includes(b.id));

  function stepLabel(n: number): string {
    return t(uiLang, 'onboarding_step_of')
      .replace('{n}', String(n))
      .replace('{total}', String(TOTAL_STEPS));
  }

  function formatSize(bytes: number): string {
    if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
    if (bytes >= 1024 * 1024) return Math.round(bytes / (1024 * 1024)) + ' MB';
    return Math.round(bytes / 1024) + ' KB';
  }

  // SVG icons
  const S = 'stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"';
  const iconSvg: Record<string, string> = {
    cpu: `<svg viewBox="0 0 20 20" fill="none" ${S}><rect x="5" y="5" width="10" height="10" rx="1.5"/><rect x="7.5" y="7.5" width="5" height="5" rx="0.5"/><path d="M7.5 5V2.5M10 5V2.5M12.5 5V2.5M7.5 15v2.5M10 15v2.5M12.5 15v2.5M5 7.5H2.5M5 10H2.5M5 12.5H2.5M15 7.5h2.5M15 10h2.5M15 12.5h2.5"/></svg>`,
    cuda: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M2 10c2.5-5.5 13.5-5.5 16 0-2.5 5.5-13.5 5.5-16 0z"/><circle cx="10" cy="10" r="3"/><circle cx="10" cy="10" r="1.3" fill="currentColor" stroke="none"/></svg>`,
    vulkan: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M3 4L10 17L17 4"/><path d="M7 4L10 9.5L13 4"/></svg>`,
    gpu: `<svg viewBox="0 0 20 20" fill="none" ${S}><rect x="1" y="6" width="14.5" height="8.5" rx="1.5"/><rect x="3" y="8" width="4" height="4" rx="0.5"/><circle cx="12" cy="10.25" r="2"/><path d="M15.5 8h3M15.5 10.25h3M15.5 12.5h3"/></svg>`,
    download: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M10 3v9M7 9l3 3 3-3"/><path d="M3 14.5v1a1 1 0 001 1h12a1 1 0 001-1v-1"/></svg>`,
    check: `<svg viewBox="0 0 20 20" fill="none" ${S}><path d="M4 10l4 4 8-8"/></svg>`,
    mic: `<svg viewBox="0 0 20 20" fill="none" ${S}><rect x="7" y="2" width="6" height="10" rx="3"/><path d="M4 10a6 6 0 0012 0"/><path d="M10 16v2M7 18h6"/></svg>`,
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

      <!-- ==================== STEP 1: Basic Settings ==================== -->
      {#if step === 1}
        <h2 class="step-title">{t(uiLang, 'onboarding_step1_title_new')}</h2>
        <p class="step-hint">{t(uiLang, 'onboarding_step1_hint_new')}</p>

        <!-- Language -->
        <div class="field">
          <!-- svelte-ignore a11y-label-has-associated-control -->
          <label class="field-label">{t(uiLang, 'uiLanguage')}</label>
          <div class="lang-grid">
            {#each LANGS as l (l.code)}
              <button class="lang-pill" class:active={uiLang === l.code} on:click={() => uiLang = l.code}>
                {l.label}
              </button>
            {/each}
          </div>
        </div>

        <!-- Theme -->
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

        <!-- Microphone -->
        <div class="field">
          <label for="mic-select" class="field-label">{t(uiLang, 'microphone')}</label>
          {#if microphones.length === 0}
            <div class="mic-warning">
              <div class="mic-warning-icon">{@html iconSvg.mic}</div>
              <div class="mic-warning-text">
                <span class="mic-warning-title">{t(uiLang, 'diag_no_microphone')}</span>
                <span class="mic-warning-hint">{t(uiLang, 'onboarding_no_mic_hint')}</span>
              </div>
            </div>
          {:else}
            <select id="mic-select" class="select" bind:value={microphoneId}>
              <option value="">{t(uiLang, 'default_mic')}</option>
              {#each microphones as mic (mic.id)}
                <option value={mic.id}>{mic.name}{mic.isDefault ? ' ★' : ''}</option>
              {/each}
            </select>
          {/if}
        </div>

      <!-- ==================== STEP 2: Downloads (Optional) ==================== -->
      {:else if step === 2}
        <h2 class="step-title">{t(uiLang, 'onboarding_step2_title_new')}</h2>
        <p class="step-hint">{t(uiLang, 'onboarding_step2_hint_new')}</p>

        <!-- MODEL SECTION -->
        <div class="section">
          <div class="section-header">
            <span class="section-label">{t(uiLang, 'onboarding_model_section')}</span>
            <button class="btn-link-sm" on:click={() => dispatch('openModels')}>
              {t(uiLang, 'manageModels')} →
            </button>
          </div>

          {#if downloadedModels.length > 0}
            <div class="model-ready">
              <span class="model-ready-icon">{@html iconSvg.check}</span>
              <span class="model-ready-text">
                {downloadedModels.length === 1
                  ? downloadedModels[0].name
                  : `${downloadedModels.length} ${t(uiLang, 'onboarding_models_ready')}`}
              </span>
            </div>
          {/if}
          {#if recommendedModels.length === 0 && downloadedModels.length > 0}
            <div class="model-ready">
              <span class="model-ready-icon">{@html iconSvg.check}</span>
              <span class="model-ready-text">{t(uiLang, 'onboarding_models_all_ready')}</span>
            </div>
          {:else if recommendedModels.length > 0}
            <p class="section-note">{t(uiLang, 'onboarding_model_explain')}</p>
            <div class="model-list">
              {#each recommendedModels as m (m.name)}
                {@const mState = getModelState(m.name)}
                <div class="model-item">
                  <div class="model-item-info">
                    <div class="model-item-top">
                      <span class="model-item-name">{m.name}</span>
                      {#if m.category === 'fast'}
                        <span class="model-cat-badge cat-fast">&#9889; Fast</span>
                      {:else if m.category === 'balanced'}
                        <span class="model-cat-badge cat-balanced">&#9878; Balanced</span>
                      {:else if m.category === 'quality'}
                        <span class="model-cat-badge cat-quality">&#128081; Best</span>
                      {/if}
                      <span class="model-item-size">{m.size}</span>
                    </div>
                    {#if m.description}
                      <span class="model-item-desc">{m.description}</span>
                    {/if}
                  </div>
                  {#if mState.status === 'downloading'}
                    <div class="model-item-progress">
                      <div class="mini-bar">
                        <div class="mini-fill" style="width: {mState.percent}%"></div>
                      </div>
                      <span class="mini-pct">{Math.round(mState.percent)}%</span>
                    </div>
                  {:else if mState.status === 'done'}
                    <span class="model-item-done">{@html iconSvg.check}</span>
                  {:else if mState.status === 'error'}
                    <button class="btn-sm" on:click={() => startModelDownload(m.name)}>
                      {t(uiLang, 'onboarding_retry')}
                    </button>
                  {:else}
                    <button class="btn-sm" on:click={() => startModelDownload(m.name)}>
                      {@html iconSvg.download}
                      <span>{t(uiLang, 'modelGet')}</span>
                    </button>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- GPU BACKEND SECTION -->
        <div class="section">
          <div class="section-header">
            <span class="section-label">{t(uiLang, 'onboarding_gpu_section')}</span>
          </div>
          <p class="section-note">{t(uiLang, 'onboarding_gpu_explain')}</p>

          <div class="acc-list">
            {#each visibleGpuBackends as b (b.id)}
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
                <div class="acc-card-header"
                  on:click={() => {
                    if (isReady || done) { backend = b.id; }
                  }}
                  role="button" tabindex="0"
                  on:keydown={(e) => { if (e.key === 'Enter') {
                    if (isReady || done) { backend = b.id; }
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
                    <div class="acc-explain">{backendExplanation(b)}</div>
                    <div class="acc-status" class:status-ready={isReady || done} class:status-error={hasError}>
                      {gpuStatusText(b)}
                    </div>
                  </div>
                  {#if isReady || done}
                    <div class="acc-select-dot" class:dot-active={backend === b.id}></div>
                  {/if}
                </div>

                <!-- 2-stage component status for CUDA -->
                {#if b.id === 'cuda' && !isReady && !done && !downloading}
                  <div class="cuda-stages">
                    <div class="cuda-stage-item">
                      <span class="cuda-stage-icon" class:stage-ok={b.runtimeInstalled}>{b.runtimeInstalled ? '✓' : '✗'}</span>
                      <span>{t(uiLang, b.runtimeInstalled ? 'cuda_runtime_installed' : 'cuda_runtime_missing')}</span>
                    </div>
                    <div class="cuda-stage-item">
                      <span class="cuda-stage-icon" class:stage-ok={b.compiled}>{b.compiled ? '✓' : '✗'}</span>
                      <span>{t(uiLang, b.compiled ? 'cuda_dll_ready' : 'cuda_dll_missing')}</span>
                      {#if !b.compiled && b.downloadSizeMB > 0}
                        <span class="cuda-stage-size">~{b.downloadSizeMB}{t(uiLang, 'mb')}</span>
                      {/if}
                    </div>
                  </div>
                {/if}

                <!-- Direct download/install button -->
                {#if (needsDL || needsInstall) && !downloading && !done && !hasError}
                  <div class="acc-action-row">
                    <button class="btn-install" on:click|stopPropagation={() => startInstall(b.id)}>
                      {@html iconSvg.download}
                      <span>{needsInstall ? t(uiLang, 'onboarding_install_btn') : t(uiLang, 'onboarding_download_btn')}</span>
                      {#if b.downloadSizeMB > 0}
                        <span class="btn-size">~{b.downloadSizeMB} {t(uiLang, 'mb')}</span>
                      {/if}
                    </button>
                  </div>
                {/if}

                <!-- Progress bar -->
                {#if downloading}
                  <div class="dl-progress">
                    {#if b.id === 'cuda'}
                      <div class="dl-step-label">
                        {t(uiLang, (state.stage === 'downloading_runtime' || state.stage === 'installing_runtime') ? 'cuda_step_1' : 'cuda_step_2')}
                      </div>
                    {/if}
                    <div class="dl-bar">
                      <div class="dl-fill" style="width: {(state.stage === 'installing_runtime') ? 100 : state.percent}%"
                        class:dl-fill-pulse={state.stage === 'installing_runtime'}></div>
                    </div>
                    <div class="dl-meta">
                      <span>{state.stageText}</span>
                      {#if state.stage !== 'installing_runtime'}
                        <span>{Math.round(state.percent)}%</span>
                      {/if}
                    </div>
                    {#if state.stage === 'downloading_runtime' || state.stage === 'installing_runtime'}
                      <div class="dl-uac-hint">{t(uiLang, 'cuda_uac_warning')}</div>
                    {/if}
                  </div>
                {/if}


                <!-- Error -->
                {#if hasError}
                  <div class="error-row">
                    <span class="error-text">{state.error}</span>
                    <button class="btn-retry" on:click={() => startInstall(b.id)}>
                      {t(uiLang, 'onboarding_retry')}
                    </button>
                  </div>
                {/if}
              </div>
            {/each}

            <!-- CPU -->
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
        </div>

        <!-- UNIFIED PROGRESS AREA -->
        {#if activeModelDL || activeBackendDL}
          <div class="unified-progress">
            <div class="unified-progress-label">{t(uiLang, 'onboarding_download_active')}</div>
            {#if activeModelDL}
              <div class="unified-progress-item">
                <span class="unified-item-name">{t(uiLang, 'model')}: {activeModelDL[0]}</span>
                <div class="unified-bar">
                  <div class="unified-fill" style="width: {activeModelDL[1].percent}%"></div>
                </div>
                <span class="unified-pct">{Math.round(activeModelDL[1].percent)}%</span>
              </div>
            {/if}
            {#if activeBackendDL}
              <div class="unified-progress-item">
                <span class="unified-item-name">{t(uiLang, 'backend')}: {activeBackendDL[0]}</span>
                <div class="unified-bar">
                  <div class="unified-fill" style="width: {activeBackendDL[1].percent}%"></div>
                </div>
                <span class="unified-pct">{Math.round(activeBackendDL[1].percent)}%</span>
              </div>
            {/if}
            <p class="unified-note">{t(uiLang, 'onboarding_download_bg_note')}</p>
          </div>
        {/if}

      {/if}
    </div>

    <!-- Footer -->
    <div class="card-footer">
      <div class="footer-left">
        {#if step > 1}
          <button class="btn-back" on:click={back}>{t(uiLang, 'onboarding_back')}</button>
        {:else}
          <div></div>
        {/if}
      </div>
      <div class="footer-right">
        <button class="btn-skip" on:click={skip}>
          {step === 2 ? t(uiLang, 'onboarding_skip_downloads') : t(uiLang, 'onboarding_skip')}
        </button>
        {#if step < TOTAL_STEPS}
          <button class="btn-next" on:click={next}>
            {t(uiLang, 'onboarding_next')} →
          </button>
        {:else}
          <button class="btn-finish" on:click={finish}>
            {t(uiLang, 'onboarding_finish')}
            {#if isAnyDownloading}
              <span class="btn-dl-badge">⟳</span>
            {/if}
          </button>
        {/if}
      </div>
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
    width: 460px;
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
  .select {
    width: 100%; padding: 8px 10px; border-radius: 8px;
    border: 1px solid var(--toggle-border, var(--border-color));
    background: var(--bg-input, var(--bg-page));
    color: var(--text-secondary); font-size: 14px; outline: none;
    box-sizing: border-box; transition: border-color .14s;
  }
  .select:focus { border-color: var(--accent); }

  /* Microphone warning */
  .mic-warning {
    display: flex; align-items: flex-start; gap: 10px;
    padding: 12px 14px; border-radius: 10px;
    border: 1px solid var(--accent-red, #ef4444);
    background: rgba(239, 68, 68, 0.06);
  }
  .mic-warning-icon {
    width: 20px; height: 20px; flex-shrink: 0;
    color: var(--accent-red, #ef4444);
  }
  .mic-warning-icon :global(svg) { width: 20px; height: 20px; display: block; }
  .mic-warning-text { display: flex; flex-direction: column; gap: 2px; }
  .mic-warning-title { font-size: 13px; font-weight: 600; color: var(--accent-red, #ef4444); }
  .mic-warning-hint { font-size: 12px; color: var(--text-tertiary); line-height: 1.4; }

  /* Sections (Step 2) */
  .section {
    display: flex; flex-direction: column; gap: 8px;
  }
  .section-header {
    display: flex; align-items: center; justify-content: space-between;
  }
  .section-label {
    font-size: 11px; font-weight: 600; color: var(--text-tertiary);
    text-transform: uppercase; letter-spacing: .06em;
  }
  .section-note {
    font-size: 12px; color: var(--text-tertiary); margin: 0; line-height: 1.5;
  }

  .btn-link-sm {
    background: transparent; border: none; padding: 0;
    color: var(--accent); font-size: 11px; cursor: pointer;
    opacity: .85; transition: opacity .14s;
  }
  .btn-link-sm:hover { opacity: 1; }

  /* Model section */
  .model-ready {
    display: flex; align-items: center; gap: 8px;
    padding: 10px 14px; border-radius: 8px;
    border: 1px solid #34d399;
    background: rgba(52, 211, 153, 0.06);
  }
  .model-ready-icon {
    width: 18px; height: 18px; color: #34d399; flex-shrink: 0;
  }
  .model-ready-icon :global(svg) { width: 18px; height: 18px; display: block; }
  .model-ready-text { font-size: 13px; color: var(--text-secondary); }

  .model-list { display: flex; flex-direction: column; gap: 6px; }
  .model-item {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 8px;
    border: 1px solid var(--border-color);
    gap: 8px;
  }
  .model-item-info { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0; }
  .model-item-top { display: flex; align-items: center; gap: 6px; }
  .model-item-name { font-size: 13px; color: var(--text-primary); font-weight: 500; }
  .model-item-size { font-size: 11px; color: var(--text-tertiary); }
  .model-item-desc {
    font-size: 10px; color: var(--text-muted); line-height: 1.3;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .model-cat-badge {
    font-size: 9px; padding: 1px 6px; border-radius: 4px;
    font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em;
    flex-shrink: 0; white-space: nowrap;
  }
  .cat-fast { color: #34d399; background: rgba(52, 211, 153, 0.12); }
  .cat-balanced { color: #60a5fa; background: rgba(96, 165, 250, 0.12); }
  .cat-quality { color: #fbbf24; background: rgba(251, 191, 36, 0.12); }

  .model-item-progress { display: flex; align-items: center; gap: 8px; flex-shrink: 0; width: 120px; }
  .mini-bar { flex: 1; height: 4px; background: var(--border-subtle, rgba(255,255,255,.07)); border-radius: 2px; overflow: hidden; }
  .mini-fill { height: 100%; background: var(--accent); border-radius: 2px; transition: width .3s ease; }
  .mini-pct { font-size: 11px; color: var(--text-tertiary); width: 30px; text-align: right; }

  .model-item-done {
    width: 18px; height: 18px; color: #34d399; flex-shrink: 0;
  }
  .model-item-done :global(svg) { width: 18px; height: 18px; display: block; }

  .btn-sm {
    display: flex; align-items: center; gap: 4px;
    padding: 5px 10px; border-radius: 6px;
    background: var(--accent-dim); border: 1px solid var(--accent);
    color: var(--accent); font-size: 12px; font-weight: 500;
    cursor: pointer; transition: all .14s; flex-shrink: 0;
  }
  .btn-sm:hover { background: var(--accent); color: #fff; }
  .btn-sm :global(svg) { width: 14px; height: 14px; display: block; }

  /* Acceleration cards */
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
  .acc-explain { font-size: 11px; color: var(--text-tertiary); line-height: 1.4; opacity: 0.8; }
  .acc-status { font-size: 12px; color: var(--text-tertiary); margin-top: 2px; }
  .acc-status.status-ready { color: #34d399; }
  .acc-status.status-error { color: var(--accent-red, #ef4444); }

  .badge-rec {
    font-size: 10px; padding: 1px 5px; border-radius: 4px;
    background: var(--accent-dim); color: var(--accent);
    font-weight: 600; letter-spacing: .04em;
  }
  .badge-ok { font-size: 11px; color: #34d399; }
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


  .dl-progress { padding: 0 14px 12px; }
  .dl-step-label { font-size: 10px; font-weight: 600; color: var(--accent); margin-bottom: 4px; font-family: ui-monospace, monospace; }
  .dl-bar { height: 4px; background: var(--border-subtle, rgba(255,255,255,.07)); border-radius: 2px; overflow: hidden; }
  .dl-fill { height: 100%; background: var(--accent); border-radius: 2px; transition: width .3s ease; }
  .dl-fill-pulse { animation: dl-pulse 1.2s ease-in-out infinite; }
  @keyframes dl-pulse { 0%, 100% { opacity: 0.5; } 50% { opacity: 1; } }
  .dl-meta { display: flex; justify-content: space-between; font-size: 11px; color: var(--text-tertiary); margin-top: 4px; }
  .dl-uac-hint { font-size: 10px; color: #f59e0b; margin-top: 3px; font-family: ui-monospace, monospace; }

  /* CUDA 2-stage component status */
  .cuda-stages { padding: 6px 14px 8px; display: flex; flex-direction: column; gap: 3px; }
  .cuda-stage-item { display: flex; align-items: center; gap: 6px; font-size: 10px; color: var(--text-muted); font-family: ui-monospace, monospace; }
  .cuda-stage-icon { width: 14px; text-align: center; flex-shrink: 0; font-size: 11px; }
  .cuda-stage-icon.stage-ok { color: #22c55e; }
  .cuda-stage-size { opacity: 0.6; font-size: 9px; }

  .acc-action-row {
    padding: 6px 14px 10px;
    display: flex; align-items: center;
  }
  .btn-install {
    display: flex; align-items: center; gap: 6px;
    padding: 7px 16px; border-radius: 7px;
    border: 1px solid var(--accent); background: var(--accent);
    color: #fff; font-size: 13px; font-weight: 500;
    cursor: pointer; transition: all .14s;
  }
  .btn-install:hover { opacity: .85; }
  .btn-install :global(svg) { width: 14px; height: 14px; }
  .btn-size { font-size: 11px; opacity: .7; }

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

  /* Unified progress area */
  .unified-progress {
    padding: 14px;
    background: var(--bg-input, rgba(255,255,255,.04));
    border: 1px solid var(--accent);
    border-radius: 10px;
    display: flex; flex-direction: column; gap: 10px;
  }
  .unified-progress-label {
    font-size: 12px; font-weight: 600; color: var(--accent);
  }
  .unified-progress-item {
    display: flex; align-items: center; gap: 10px;
  }
  .unified-item-name {
    font-size: 12px; color: var(--text-secondary); width: 110px; flex-shrink: 0;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .unified-bar {
    flex: 1; height: 6px; background: var(--border-subtle, rgba(255,255,255,.07));
    border-radius: 3px; overflow: hidden;
  }
  .unified-fill {
    height: 100%; background: var(--accent); border-radius: 3px;
    transition: width .3s ease;
  }
  .unified-pct {
    font-size: 12px; color: var(--text-tertiary); width: 36px; text-align: right;
    font-family: ui-monospace, monospace;
  }
  .unified-note {
    font-size: 11px; color: var(--text-tertiary); margin: 0; line-height: 1.4;
  }

  /* Footer */
  .card-footer {
    flex-shrink: 0;
    display: flex; justify-content: space-between; align-items: center;
    padding: 14px 24px;
    border-top: 1px solid var(--border-subtle, rgba(255,255,255,.06));
    gap: 12px;
  }
  .footer-left { display: flex; gap: 8px; }
  .footer-right { display: flex; gap: 8px; align-items: center; }

  .btn-back {
    background: transparent; border: 1px solid var(--border-color);
    border-radius: 8px; color: var(--text-secondary);
    font-size: 14px; padding: 8px 16px; cursor: pointer; transition: all .15s;
  }
  .btn-back:hover { border-color: var(--border-hover); color: var(--text-primary); }

  .btn-skip {
    background: transparent; border: none;
    color: var(--text-tertiary); font-size: 13px;
    cursor: pointer; transition: color .14s;
    padding: 8px 12px;
  }
  .btn-skip:hover { color: var(--text-secondary); }

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
