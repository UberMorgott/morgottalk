<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let type: 'error' | 'warning' | 'info' = 'info';
  export let message: string = '';
  export let dismissible: boolean = true;

  const dispatch = createEventDispatcher();

  let expanded = false;

  function handleDismiss() {
    dispatch('dismiss');
  }

  function toggleDetails() {
    expanded = !expanded;
  }
</script>

<div class="toast toast-{type}">
  <div class="toast-message">{message}</div>

  {#if $$slots.default}
    <button class="toast-toggle-details" on:click={toggleDetails}>
      {expanded ? 'Hide details' : 'Show details'}
    </button>
    {#if expanded}
      <div class="toast-details">
        <slot />
      </div>
    {/if}
  {/if}

  {#if dismissible}
    <button class="toast-dismiss" on:click={handleDismiss} title="Dismiss">×</button>
  {/if}
</div>

<style>
  .toast {
    font-family: ui-monospace, monospace;
    font-size: 12px;
    padding: 8px 12px;
    border: 1.5px solid var(--border-color);
    border-radius: 6px;
    background: var(--bg-input);
    display: flex;
    flex-direction: column;
    gap: 6px;
    position: relative;
  }

  .toast-error {
    border-color: var(--accent-red, #ff4444);
    color: var(--accent-red, #ff4444);
  }

  .toast-warning {
    border-color: var(--accent);
    color: var(--accent);
  }

  .toast-info {
    border-color: var(--border-hover);
    color: var(--text-secondary);
  }

  .toast-message {
    margin: 0;
  }

  .toast-toggle-details {
    all: unset;
    cursor: pointer;
    font-size: 11px;
    color: var(--text-secondary);
    text-decoration: underline;
    padding: 2px 0;
    transition: color 0.2s;
  }

  .toast-toggle-details:hover {
    color: var(--text-primary);
  }

  .toast-details {
    font-size: 11px;
    color: var(--text-tertiary, var(--text-secondary));
    padding-left: 12px;
    border-left: 2px solid var(--border-subtle, var(--border-color));
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 2px;
  }

  .toast-dismiss {
    all: unset;
    cursor: pointer;
    position: absolute;
    top: 6px;
    right: 8px;
    font-size: 18px;
    line-height: 1;
    color: var(--text-secondary);
    padding: 2px 4px;
    transition: color 0.2s;
  }

  .toast-dismiss:hover {
    color: var(--text-primary);
  }
</style>
