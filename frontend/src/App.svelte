<script lang="ts">
  import MainPage from './pages/MainPage.svelte';
  import HistoryPage from './pages/HistoryPage.svelte';
  import OverlayPage from './pages/OverlayPage.svelte';

  const params = window.location.search;
  const isHistoryWindow = params.includes('window=history');
  const isOverlayWindow = params.includes('window=overlay');
</script>

{#if isOverlayWindow}
  <OverlayPage />
{:else}
  <main class="w-full h-full overflow-hidden" style="background: var(--bg-page); color: var(--text-secondary);">
    {#if isHistoryWindow}
      <HistoryPage />
    {:else}
      <MainPage />
    {/if}
  </main>
{/if}
