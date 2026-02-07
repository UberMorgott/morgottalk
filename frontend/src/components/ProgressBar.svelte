<script lang="ts">
  export let percent: number = 0;
</script>

<div class="progress-track">
  <div class="progress-fill"
       style="width: {Math.min(100, Math.max(0, percent))}%;">
  </div>
</div>

<style>
  .progress-track {
    width: 100%;
    height: 6px;
    border-radius: 9999px;
    background: var(--bg-input);
    overflow: hidden;
    border: 1px solid var(--border-subtle);
  }
  .progress-fill {
    height: 100%;
    border-radius: 9999px;
    transition: width 0.2s;
    background: linear-gradient(90deg, var(--accent), var(--accent));
    box-shadow: 0 0 8px var(--accent-dim);
  }
</style>
