<script lang="ts">
  import { onMount } from 'svelte';
  import { GetHistory, ClearHistory, DeleteEntry } from '../../bindings/github.com/UberMorgott/transcribation/services/historyservice.js';
  import { GetGlobalSettings } from '../../bindings/github.com/UberMorgott/transcribation/services/settingsservice.js';
  import { t } from '../lib/i18n';
  import type { Lang } from '../lib/i18n';

  let entries: { text: string; timestamp: number; language: string }[] = [];
  let confirmClear = false;
  let lang: Lang = 'en';

  onMount(async () => {
    // Apply theme and language
    try {
      const gs = await GetGlobalSettings();
      if (gs?.theme) {
        document.documentElement.setAttribute('data-theme', gs.theme);
      }
      if (gs?.uiLang) {
        lang = gs.uiLang as Lang;
      }
    } catch {}
    await loadHistory();
  });

  async function loadHistory() {
    try {
      entries = await GetHistory() || [];
    } catch {}
  }

  async function handleClear() {
    if (!confirmClear) {
      confirmClear = true;
      setTimeout(() => confirmClear = false, 3000);
      return;
    }
    await ClearHistory();
    entries = [];
    confirmClear = false;
  }

  async function handleDelete(ts: number) {
    await DeleteEntry(ts);
    entries = entries.filter(e => e.timestamp !== ts);
  }

  async function copyText(text: string) {
    await navigator.clipboard.writeText(text);
  }

  function formatTime(ts: number): string {
    const d = new Date(ts);
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${pad(d.getDate())}.${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
</script>

<div class="root">
  <!-- Header -->
  <div class="header">
    <div class="header-left">
      <div class="header-accent"></div>
      <h2 class="header-title">{t(lang, 'history')}</h2>
      <span class="entry-count">{entries.length}</span>
    </div>
    {#if entries.length > 0}
      <button
        class="clear-btn"
        class:clear-confirm={confirmClear}
        on:click={handleClear}
      >
        {confirmClear ? t(lang, 'confirm') + '?' : t(lang, 'clearAll')}
      </button>
    {/if}
  </div>

  <!-- List -->
  <div class="list">
    {#if entries.length === 0}
      <div class="empty-state">
        <svg class="empty-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <p class="empty-text">{t(lang, 'noHistory')}</p>
      </div>
    {:else}
      {#each entries as entry (entry.timestamp)}
        <div class="entry-card">
          <div class="entry-header">
            <span class="entry-time">{formatTime(entry.timestamp)}</span>
            <div class="entry-actions">
              <button class="action-btn" on:click={() => copyText(entry.text)} title={t(lang, 'copy')}>
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
                </svg>
              </button>
              <button class="action-btn action-btn-del" on:click={() => handleDelete(entry.timestamp)} title={t(lang, 'delete')}>
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>
          <p class="entry-text selectable">{entry.text}</p>
          {#if entry.language && entry.language !== 'auto'}
            <span class="entry-lang">{entry.language}</span>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .root {
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  /* -- Header -- */
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px 12px;
    flex-shrink: 0;
  }
  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .header-accent {
    width: 3px;
    height: 16px;
    border-radius: 2px;
    background: linear-gradient(to bottom, var(--accent), var(--accent-red));
  }
  .header-title {
    font-size: 15px;
    font-family: ui-monospace, monospace;
    letter-spacing: 0.15em;
    text-transform: uppercase;
    color: var(--text-tertiary);
  }
  .entry-count {
    font-size: 12px;
    font-family: ui-monospace, monospace;
    color: var(--text-muted);
  }

  .clear-btn {
    font-size: 12px;
    font-family: ui-monospace, monospace;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 5px 12px;
    border-radius: 6px;
    border: 1px solid transparent;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.2s;
  }
  .clear-btn:hover {
    color: var(--text-secondary);
    background: var(--accent-dim);
  }
  .clear-confirm {
    color: var(--accent-red) !important;
    background: rgba(220, 38, 38, 0.08) !important;
    border-color: rgba(220, 38, 38, 0.2) !important;
  }

  /* -- List -- */
  .list {
    flex: 1;
    overflow-y: auto;
    padding: 0 16px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 0;
  }

  .list::-webkit-scrollbar {
    width: 4px;
  }
  .list::-webkit-scrollbar-track {
    background: transparent;
  }
  .list::-webkit-scrollbar-thumb {
    background: var(--border-subtle);
    border-radius: 4px;
  }

  /* -- Empty state -- */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-muted);
  }
  .empty-icon {
    width: 48px;
    height: 48px;
    margin-bottom: 12px;
    opacity: 0.3;
  }
  .empty-text {
    font-size: 13px;
    font-family: ui-monospace, monospace;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  /* -- Entry card -- */
  .entry-card {
    padding: 12px 14px;
    border-radius: 8px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    transition: border-color 0.2s;
  }
  .entry-card:hover {
    border-color: var(--border-color);
  }

  .entry-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 6px;
  }
  .entry-time {
    font-size: 12px;
    font-family: ui-monospace, monospace;
    color: var(--text-muted);
  }
  .entry-actions {
    display: flex;
    gap: 4px;
    opacity: 0;
    transition: opacity 0.2s;
  }
  .entry-card:hover .entry-actions {
    opacity: 1;
  }

  .action-btn {
    padding: 4px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s;
    display: flex;
    align-items: center;
  }
  .action-btn:hover {
    color: var(--accent);
    background: var(--accent-dim);
  }
  .action-btn-del:hover {
    color: var(--accent-red);
    background: rgba(220, 38, 38, 0.08);
  }

  .entry-text {
    font-size: 14px;
    line-height: 1.5;
    color: var(--text-secondary);
  }

  .entry-lang {
    display: inline-block;
    margin-top: 6px;
    font-size: 10px;
    font-family: ui-monospace, monospace;
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--text-muted);
    background: var(--accent-dim);
  }
</style>
