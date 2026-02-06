<script lang="ts">
  let status = 'Ready';
  let lastText = '';
</script>

<main class="min-h-screen bg-[#12121a] text-white flex flex-col items-center justify-center p-8">
  <h1 class="text-3xl font-bold mb-2">Transcribation</h1>
  <p class="text-gray-400 mb-8">Push-to-talk voice transcription</p>

  <div class="w-20 h-20 rounded-full border-4 border-gray-600 flex items-center justify-center mb-6
    {status === 'Recording' ? 'border-red-500 animate-pulse' : ''}">
    <span class="text-2xl">{status === 'Recording' ? '🔴' : '🎤'}</span>
  </div>

  <p class="text-sm text-gray-500 mb-4">{status}</p>

  {#if lastText}
    <div class="bg-gray-800 rounded-lg p-4 max-w-md w-full">
      <p class="text-gray-200">{lastText}</p>
    </div>
  {/if}
</main>
