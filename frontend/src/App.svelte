<script lang="ts">
  import MainPage from './pages/MainPage.svelte';
  import HistoryPage from './pages/HistoryPage.svelte';

  const isHistoryWindow = window.location.search.includes('window=history');
</script>

<main class="w-full h-full overflow-hidden" style="background: var(--bg-page); color: var(--text-secondary);">
  {#if isHistoryWindow}
    <HistoryPage />
  {:else}
    <MainPage />
  {/if}
</main>
