<script lang="ts">
  import { onMount, tick } from 'svelte';
  import Sortable from 'sortablejs';
  import { Events } from '@wailsio/runtime';
  import { GetPresets, CreatePreset, UpdatePreset, DeletePreset, SetPresetEnabled, StartRecording, StopRecording, GetRecordingStates, GetModelLanguages, ReorderPresets } from '../../bindings/github.com/UberMorgott/transcribation/services/presetservice.js';
  import { GetGlobalSettings, GetMicrophones, GetAllBackends, GetSystemInfo } from '../../bindings/github.com/UberMorgott/transcribation/services/settingsservice.js';
  import { GetAvailableModels, DownloadModel, DeleteModel, GetModelsDir, CancelDownload } from '../../bindings/github.com/UberMorgott/transcribation/services/modelservice.js';
  import { OpenHistoryWindow } from '../../bindings/github.com/UberMorgott/transcribation/services/historyservice.js';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';
  import PresetCard from '../components/PresetCard.svelte';
  import PresetEditor from '../components/PresetEditor.svelte';
  import SettingsModal from '../components/SettingsModal.svelte';
  import ModelModal from '../components/ModelModal.svelte';
  import Toast from '../components/Toast.svelte';

  type Preset = {
    id: string; name: string; modelName: string; keepModelLoaded: boolean;
    inputMode: string; hotkey: string; language: string; useKBLayout: boolean;
    keepHistory: boolean; enabled: boolean;
  };

  // State
  let presets: Preset[] = [];
  let states: Record<string, string> = {}; // id -> "idle"/"recording"/"processing"
  let microphoneId = '';
  let microphones: { id: string; name: string; isDefault: boolean }[] = [];
  let models: { name: string; fileName: string; size: string; sizeBytes: number; downloaded: boolean }[] = [];
  let downloading: Record<string, number> = {};
  let modelsDir = '';
  let languages: { code: string; name: string }[] = [];
  let backends: { id: string; name: string; compiled: boolean; systemAvailable: boolean; canInstall: boolean; installHint: string }[] = [];
  let backend = 'auto';

  // Theme & language state — read localStorage to match index.html inline script
  let theme: 'dark' | 'light' = (() => {
    try {
      const t = localStorage.getItem('morgottalk-theme');
      if (t === 'light') return 'light' as const;
    } catch {}
    return 'dark' as const;
  })();
  let uiLang: Lang = 'en';
  let closeAction = '';
  let autoStart = false;
  let startMinimized = false;

  // Modal state
  let showSettings = false;
  let showModels = false;
  let creatingPreset = false;

  // Expandable card state
  let expandedPresetId: string | null = null;

  // SortableJS instance
  let listEl: HTMLElement;
  let sortable: Sortable;

  // Polling for state updates
  let stateInterval: ReturnType<typeof setInterval>;

  // Diagnostic state
  let diagnosticMessage = '';
  let diagnosticType: 'error' | 'warning' | 'info' = 'info';

  function showDiagnostic(type: 'error' | 'warning' | 'info', message: string) {
    diagnosticType = type;
    diagnosticMessage = message;
  }

  onMount(async () => {
    await refreshAll();

    // First-run diagnostics
    try {
      const sysInfo = await GetSystemInfo();

      if (sysInfo.microphoneCount === 0) {
        showDiagnostic('warning', t(uiLang, 'diag_no_microphone'));
      } else if (sysInfo.modelsCount === 0 && presets.length > 0) {
        showDiagnostic('warning', t(uiLang, 'diag_no_models'));
      } else if (backend === 'cpu') {
        const gpuBackend = sysInfo.backends.find(b =>
          b.systemAvailable && b.id !== 'cpu' && b.id !== 'auto'
        );
        if (gpuBackend) {
          const gpuName = gpuBackend.gpuDetected || gpuBackend.name;
          showDiagnostic('info',
            t(uiLang, 'diag_gpu_available').replace('{gpu}', gpuName)
          );
        }
      }
    } catch (e) {
      console.error('Failed to get system info:', e);
    }

    // Poll recording states
    stateInterval = setInterval(async () => {
      try {
        const stateList = await GetRecordingStates() || [];
        const newStates: Record<string, string> = {};
        for (const s of stateList) {
          newStates[s.id] = s.state;
        }
        states = newStates;
      } catch {}
    }, 500);

    Events.On('model:download:progress', (event: any) => {
      const data = event.data?.[0] || event.data || event;
      if (data.modelName) {
        if (data.done) {
          delete downloading[data.modelName];
          downloading = downloading;
          refreshModels();
        } else {
          downloading[data.modelName] = data.percent || 0;
          downloading = downloading;
        }
      }
    });

    // Init SortableJS after DOM renders
    await tick();
    initSortable();

    return () => {
      clearInterval(stateInterval);
      if (sortable) sortable.destroy();
    };
  });

  async function refreshAll() {
    try {
      const [p, gs, mics, mdls, dir, bkends] = await Promise.all([
        GetPresets(), GetGlobalSettings(), GetMicrophones(), GetAvailableModels(), GetModelsDir(), GetAllBackends(),
      ]);
      backends = bkends || [];
      presets = p || [];
      if (gs) {
        microphoneId = gs.microphoneId || '';
        modelsDir = gs.modelsDir || '';
        if (gs.theme === 'dark' || gs.theme === 'light') {
          theme = gs.theme;
        }
        if (gs.uiLang) {
          uiLang = gs.uiLang as Lang;
        }
        closeAction = gs.closeAction || '';
        autoStart = gs.autoStart || false;
        startMinimized = gs.startMinimized || false;
        backend = gs.backend || 'auto';
        document.documentElement.setAttribute('data-theme', theme);
        try { localStorage.setItem('morgottalk-theme', theme); } catch {}
      }
      microphones = mics || [];
      models = mdls || [];
      if (!modelsDir) modelsDir = dir || '';
    } catch {}
  }

  async function refreshModels() {
    try { models = await GetAvailableModels() || []; } catch {}
  }

  async function refreshPresets() {
    try { presets = await GetPresets() || []; } catch {}
  }

  // --- Preset CRUD ---
  async function handleToggle(e: CustomEvent<{ id: string; enabled: boolean }>) {
    try {
      await SetPresetEnabled(e.detail.id, e.detail.enabled);
      await refreshPresets();
    } catch {}
  }

  // Expand/collapse card
  function handleExpand(e: CustomEvent<string>) {
    expandedPresetId = e.detail;
    const p = presets.find(p => p.id === e.detail);
    if (p) loadLanguagesForModel(p.modelName);
  }

  function handleCollapse() {
    expandedPresetId = null;
  }

  // Auto-collapse when recording starts
  $: if (activePreset) {
    expandedPresetId = null;
  }

  function handleNewPreset() {
    expandedPresetId = null;
    creatingPreset = true;
    // Default languages (native names, not localized — like in whisper model output)
    languages = [
      { code: 'auto', name: t(uiLang, 'autoDetect') },
      { code: 'en', name: 'English' },
      { code: 'ru', name: 'Russian' },
    ];
  }

  async function handleSavePreset(e: CustomEvent) {
    const data = e.detail;
    try {
      if (creatingPreset) {
        await CreatePreset(data);
        creatingPreset = false;
        await refreshPresets();
      } else {
        await UpdatePreset(data);
        // Update local array so collapsed view reflects changes
        const idx = presets.findIndex(p => p.id === data.id);
        if (idx >= 0) {
          presets[idx] = { ...data };
          presets = presets;
        }
      }
    } catch {}
  }

  async function handleDeletePreset(e: CustomEvent<string>) {
    try {
      await DeletePreset(e.detail);
      await refreshPresets();
    } catch {}
    creatingPreset = false;
    expandedPresetId = null;
  }

  async function loadLanguagesForModel(modelName: string) {
    try {
      languages = await GetModelLanguages(modelName) || [];
    } catch {
      languages = [{ code: 'auto', name: 'Auto-detect' }];
    }
  }

  // --- Settings (reactive, auto-saved by SettingsModal) ---
  function handleSettingsChange(e: CustomEvent<{ microphoneId: string; modelsDir: string; theme: string; uiLang: string; closeAction: string; autoStart: boolean; startMinimized: boolean; backend: string }>) {
    const d = e.detail;
    microphoneId = d.microphoneId;
    modelsDir = d.modelsDir;
    theme = d.theme as 'dark' | 'light';
    uiLang = d.uiLang as Lang;
    closeAction = d.closeAction;
    autoStart = d.autoStart;
    startMinimized = d.startMinimized;
    backend = d.backend;
  }

  // --- Models ---
  async function handleDownload(e: CustomEvent<string>) {
    downloading[e.detail] = 0;
    downloading = downloading;
    await DownloadModel(e.detail);
  }

  async function handleModelDelete(e: CustomEvent<string>) {
    await DeleteModel(e.detail);
    await refreshModels();
  }

  async function handleCancel(e: CustomEvent<string>) {
    await CancelDownload(e.detail);
    delete downloading[e.detail];
    downloading = downloading;
  }

  // Recording state helpers
  $: recordingPreset = presets.find(p => states[p.id] === 'recording');
  $: processingPreset = presets.find(p => states[p.id] === 'processing');
  $: activePreset = recordingPreset || processingPreset;
  // Global keyboard handler
  function handleGlobalKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && expandedPresetId && !showSettings && !showModels && !creatingPreset) {
      expandedPresetId = null;
    }
  }

  // Click outside cards to collapse
  function handlePresetsAreaClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.card') && expandedPresetId) {
      expandedPresetId = null;
    }
  }

  // SortableJS initialization
  function initSortable() {
    if (!listEl || sortable) return;
    sortable = Sortable.create(listEl, {
      handle: '.drag-grip',
      animation: 200,
      forceFallback: true,
      fallbackClass: 'sortable-fallback',
      ghostClass: 'sortable-ghost',
      chosenClass: 'sortable-chosen',
      onEnd: async (evt) => {
        if (evt.oldIndex === evt.newIndex) return;
        // Revert DOM so Svelte handles reorder via keyed each
        const { from, item, oldIndex, newIndex } = evt;
        from.removeChild(item);
        from.insertBefore(item, from.children[oldIndex!] || null);
        // Update Svelte array
        const updated = [...presets];
        const [moved] = updated.splice(oldIndex!, 1);
        updated.splice(newIndex!, 0, moved);
        presets = updated;
        await tick();
        ReorderPresets(updated.map(p => p.id)).catch(() => {});
      }
    });
  }
  // Disable sorting when a card is expanded
  $: if (sortable) sortable.option('disabled', !!expandedPresetId);
</script>

<svelte:window on:keydown={handleGlobalKeydown} />

<div class="page">
  <!-- Top bar -- full width for drag area -->
  <div class="topbar" style="--wails-draggable: drag">
    <div class="topbar-inner">
      <button class="topbar-btn" on:click={() => showSettings = true} style="--wails-draggable: no-drag" title={t(uiLang, 'tip_settings')}>
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 010 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 010-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28z" />
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
      <div class="topbar-spacer"></div>
      <div class="app-title" style="--wails-draggable: no-drag">
        <span class="app-name">Morgo<span class="app-tt">TT</span>alk</span>
        <span class="app-version">v0.3.0</span>
      </div>
      <div class="topbar-spacer"></div>
      <button class="topbar-btn" on:click={() => OpenHistoryWindow()} style="--wails-draggable: no-drag" title={t(uiLang, 'tip_history')}>
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </button>
    </div>
  </div>

  <!-- Status bar with diagnostics -->
  {#if diagnosticMessage}
    <div class="status-bar">
      <Toast
        type={diagnosticType}
        message={diagnosticMessage}
        dismissible={true}
        on:dismiss={() => diagnosticMessage = ''}
      />
    </div>
  {/if}

  <!-- Centered content column -->
  <div class="content-col">
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="presets-area" style="--wails-draggable: no-drag" on:click={handlePresetsAreaClick}>
      {#if presets.length === 0}
        <div class="empty-state">
          <p class="empty-text">{t(uiLang, 'noPresets')}</p>
          <p class="empty-sub">{t(uiLang, 'noPresetsHint')}</p>
        </div>
      {:else}
        <div class="presets-list" bind:this={listEl}>
          {#each presets as preset (preset.id)}
            <div class="preset-wrapper">
            <PresetCard
              {preset}
              state={states[preset.id] || 'idle'}
              lang={uiLang}
              {models}
              {languages}
              expanded={expandedPresetId === preset.id}
              on:toggle={handleToggle}
              on:expand={handleExpand}
              on:collapse={handleCollapse}
              on:save={handleSavePreset}
              on:delete={handleDeletePreset}
              on:openModels={() => showModels = true}
              on:modelChanged={(e) => loadLanguagesForModel(e.detail)}
            />
            </div>
          {/each}
        </div>
      {/if}

      <button class="add-btn" on:click={handleNewPreset} title={t(uiLang, 'tip_newPreset')}>
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
        </svg>
        {t(uiLang, 'newPreset')}
      </button>
    </div>
  </div>
</div>

<!-- Modals -->
{#if creatingPreset}
  <PresetEditor
    preset={null}
    isNew={true}
    {models}
    {languages}
    lang={uiLang}
    on:save={handleSavePreset}
    on:delete={handleDeletePreset}
    on:close={() => { creatingPreset = false; }}
    on:openModels={() => showModels = true}
    on:modelChanged={(e) => loadLanguagesForModel(e.detail)}
  />
{/if}

{#if showSettings}
  <SettingsModal
    {microphoneId}
    {microphones}
    {theme}
    uiLang={uiLang}
    {modelsDir}
    {closeAction}
    {autoStart}
    {startMinimized}
    {backend}
    {backends}
    on:change={handleSettingsChange}
    on:close={() => showSettings = false}
    on:openModels={() => { showSettings = false; showModels = true; }}
  />
{/if}

{#if showModels}
  <ModelModal
    {models}
    {downloading}
    {modelsDir}
    lang={uiLang}
    on:close={() => showModels = false}
    on:download={handleDownload}
    on:delete={handleModelDelete}
    on:cancel={handleCancel}
  />
{/if}

<style>
  .page {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-page);
  }

  /* -- Top bar -- spans full width for drag -- */
  .topbar {
    flex-shrink: 0;
    padding: 8px clamp(16px, 4vw, 48px);
  }
  .topbar-inner {
    display: flex;
    align-items: center;
    max-width: 680px;
    margin: 0 auto;
    width: 100%;
    gap: 6px;
  }
  .topbar-spacer { flex: 1; }

  /* -- Status bar with diagnostics -- */
  .status-bar {
    flex-shrink: 0;
    padding: 0 clamp(16px, 4vw, 48px) 8px;
    max-width: 680px;
    margin: 0 auto;
    width: 100%;
  }

  .app-title {
    display: flex;
    align-items: baseline;
    gap: 6px;
    user-select: none;
  }
  .app-name {
    font-size: 15px;
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: 0.02em;
  }
  .app-tt {
    color: var(--accent);
  }
  .app-version {
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: ui-monospace, monospace;
  }

  .topbar-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: 6px;
    border: 2px solid var(--border-color);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.2s;
  }
  .topbar-btn:hover {
    color: var(--accent);
    border-color: var(--border-hover);
  }

  /* -- Centered content column -- */
  .content-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-height: 0;
    max-width: 680px;
    width: 100%;
    margin: 0 auto;
    padding: 0 clamp(16px, 4vw, 48px);
  }

  /* -- Presets area -- */
  .presets-area {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 0;
    padding-bottom: 12px;
  }
  .presets-area::-webkit-scrollbar { width: 3px; }
  .presets-area::-webkit-scrollbar-track { background: transparent; }
  .presets-area::-webkit-scrollbar-thumb { background: var(--border-subtle); border-radius: 3px; }

  .presets-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  /* SortableJS drag styles (global — library adds these classes outside Svelte scope) */
  :global(.sortable-ghost) {
    opacity: 0.3;
  }
  :global(.sortable-chosen) {
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3), 0 0 12px var(--accent-dim);
  }
  :global(.sortable-fallback) {
    opacity: 0.9 !important;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.4), 0 0 16px var(--accent-dim);
    border-radius: 10px;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px 0;
    gap: 4px;
  }
  .empty-text {
    font-size: 16px;
    color: var(--text-primary);
  }
  .empty-sub {
    font-size: 13px;
    color: var(--text-tertiary);
  }

  .add-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px;
    border-radius: 10px;
    border: 2px dashed var(--border-hover);
    background: transparent;
    color: var(--text-secondary);
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s;
    flex-shrink: 0;
    margin-top: 4px;
  }
  .add-btn:hover {
    color: var(--accent);
    border-color: var(--border-active);
    background: var(--accent-dim);
  }
</style>
