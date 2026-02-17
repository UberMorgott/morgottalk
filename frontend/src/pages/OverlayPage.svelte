<script lang="ts">
  import { onMount } from 'svelte';
  import { Events } from '@wailsio/runtime';

  let state: 'recording' | 'processing' | 'idle' = 'idle';
  let progress = { current: 0, total: 0 };

  // Read initial state from URL param (set by Go on first window creation)
  const urlParams = new URLSearchParams(window.location.search);
  const initialState = urlParams.get('state');
  if (initialState === 'recording' || initialState === 'processing') {
    state = initialState;
  }

  onMount(() => {
    Events.On('overlay:state', (event: any) => {
      const data = event.data?.[0] || event.data || event;
      if (data.state) {
        state = data.state;
        if (data.state === 'recording') {
          progress = { current: 0, total: 0 };
        }
      }
    });

    Events.On('transcription:progress', (event: any) => {
      const data = event.data?.[0] || event.data || event;
      if (data.current && data.total) {
        progress = { current: data.current, total: data.total };
      }
    });
  });
</script>

<div class="overlay">
  {#if state === 'recording'}
    <!-- Vintage vacuum tube with audio frequency bars -->
    <div class="tube">
      <div class="tube-glass">
        <div class="tube-glow"></div>
        <div class="bars">
          <div class="bar bar1"></div>
          <div class="bar bar2"></div>
          <div class="bar bar3"></div>
          <div class="bar bar4"></div>
          <div class="bar bar5"></div>
          <div class="bar bar6"></div>
          <div class="bar bar7"></div>
          <div class="bar bar8"></div>
          <div class="bar bar9"></div>
        </div>
        <div class="tube-reflection"></div>
      </div>
      <div class="tube-base">
        <div class="tube-pin"></div>
        <div class="tube-pin"></div>
        <div class="tube-pin"></div>
      </div>
      <div class="rec-label">REC</div>
    </div>

  {:else if state === 'processing'}
    <!-- Steampunk gears with sparks -->
    <div class="gears-container">
      <svg class="gears-svg" viewBox="0 0 200 200">
        <!-- Big gear -->
        <g class="gear-big" transform-origin="80 100">
          <circle cx="80" cy="100" r="38" fill="none" stroke="#b8860b" stroke-width="3"/>
          <circle cx="80" cy="100" r="28" fill="none" stroke="#8B6914" stroke-width="2"/>
          <circle cx="80" cy="100" r="8" fill="#b8860b"/>
          {#each Array(12) as _, i}
            <rect
              x="76" y="58"
              width="8" height="14"
              rx="2"
              fill="#b8860b"
              transform="rotate({i * 30}, 80, 100)"
            />
          {/each}
        </g>
        <!-- Small gear -->
        <g class="gear-small" transform-origin="138 72">
          <circle cx="138" cy="72" r="24" fill="none" stroke="#cd7f32" stroke-width="3"/>
          <circle cx="138" cy="72" r="16" fill="none" stroke="#a0622b" stroke-width="2"/>
          <circle cx="138" cy="72" r="6" fill="#cd7f32"/>
          {#each Array(8) as _, i}
            <rect
              x="134.5" y="44"
              width="7" height="12"
              rx="2"
              fill="#cd7f32"
              transform="rotate({i * 45}, 138, 72)"
            />
          {/each}
        </g>
        <!-- Tiny gear -->
        <g class="gear-tiny" transform-origin="145 130">
          <circle cx="145" cy="130" r="18" fill="none" stroke="#8B6914" stroke-width="2"/>
          <circle cx="145" cy="130" r="10" fill="none" stroke="#6B4F12" stroke-width="1.5"/>
          <circle cx="145" cy="130" r="4" fill="#8B6914"/>
          {#each Array(8) as _, i}
            <rect
              x="142" y="109"
              width="6" height="9"
              rx="1.5"
              fill="#8B6914"
              transform="rotate({i * 45}, 145, 130)"
            />
          {/each}
        </g>
        <!-- Sparks -->
        <g class="sparks">
          <circle class="spark s1" cx="110" cy="82" r="2"/>
          <circle class="spark s2" cx="115" cy="88" r="1.5"/>
          <circle class="spark s3" cx="105" cy="78" r="1"/>
          <circle class="spark s4" cx="118" cy="95" r="1.5"/>
          <circle class="spark s5" cx="108" cy="90" r="1"/>
          <circle class="spark s6" cx="130" cy="105" r="1.5"/>
          <circle class="spark s7" cx="125" cy="110" r="1"/>
        </g>
      </svg>
      {#if progress.total > 1}
        <div class="progress-label">{progress.current}/{progress.total}</div>
      {:else}
        <div class="progress-label dots">...</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  :global(html),
  :global(body),
  :global(#app) {
    margin: 0;
    padding: 0;
    background: transparent !important;
    overflow: hidden;
  }

  .overlay {
    width: 220px;
    height: 220px;
    display: flex;
    align-items: center;
    justify-content: center;
    user-select: none;
    pointer-events: none;
  }

  /* ============================================
     RECORDING — Vintage Vacuum Tube
     ============================================ */
  .tube {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .tube-glass {
    position: relative;
    width: 120px;
    height: 150px;
    border-radius: 50% 50% 10% 10% / 40% 40% 5% 5%;
    background: radial-gradient(ellipse at 50% 60%, rgba(30, 15, 0, 0.7) 0%, rgba(10, 5, 0, 0.85) 100%);
    border: 2px solid rgba(255, 143, 12, 0.25);
    overflow: hidden;
    box-shadow:
      0 0 30px rgba(255, 100, 0, 0.3),
      0 0 60px rgba(255, 60, 0, 0.15),
      inset 0 0 20px rgba(255, 100, 0, 0.1);
  }

  .tube-glow {
    position: absolute;
    bottom: 0;
    left: 10%;
    width: 80%;
    height: 40%;
    background: radial-gradient(ellipse at 50% 100%, rgba(255, 120, 0, 0.4) 0%, transparent 70%);
    animation: glow-pulse 2s ease-in-out infinite;
  }

  @keyframes glow-pulse {
    0%, 100% { opacity: 0.6; }
    50% { opacity: 1; }
  }

  .tube-reflection {
    position: absolute;
    top: 8px;
    left: 15%;
    width: 30%;
    height: 40%;
    background: linear-gradient(160deg, rgba(255, 255, 255, 0.12) 0%, transparent 60%);
    border-radius: 50%;
    pointer-events: none;
  }

  /* Audio frequency bars inside tube */
  .bars {
    position: absolute;
    bottom: 15px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    gap: 3px;
    align-items: flex-end;
    height: 80px;
  }

  .bar {
    width: 8px;
    border-radius: 2px 2px 0 0;
    animation: audio-wave 1s ease-in-out infinite;
    background: linear-gradient(0deg, #ff6f00 0%, #ff8f0c 50%, #ffb74d 100%);
    box-shadow: 0 0 6px rgba(255, 143, 12, 0.6);
    opacity: 0.9;
  }

  .bar1 { animation-duration: 0.8s; height: 30px; }
  .bar2 { animation-duration: 1.1s; animation-delay: 0.1s; height: 50px; }
  .bar3 { animation-duration: 0.7s; animation-delay: 0.05s; height: 40px; }
  .bar4 { animation-duration: 1.3s; animation-delay: 0.15s; height: 65px; }
  .bar5 { animation-duration: 0.9s; animation-delay: 0.08s; height: 55px; }
  .bar6 { animation-duration: 1.2s; animation-delay: 0.12s; height: 45px; }
  .bar7 { animation-duration: 0.75s; animation-delay: 0.18s; height: 60px; }
  .bar8 { animation-duration: 1.0s; animation-delay: 0.03s; height: 35px; }
  .bar9 { animation-duration: 0.85s; animation-delay: 0.2s; height: 25px; }

  @keyframes audio-wave {
    0%, 100% { transform: scaleY(0.3); }
    25% { transform: scaleY(1); }
    50% { transform: scaleY(0.5); }
    75% { transform: scaleY(0.85); }
  }

  /* Tube base (socket) */
  .tube-base {
    width: 70px;
    height: 18px;
    background: linear-gradient(180deg, #2a2a2a 0%, #1a1a1a 100%);
    border-radius: 0 0 8px 8px;
    border: 1px solid rgba(255, 143, 12, 0.15);
    border-top: none;
    display: flex;
    justify-content: center;
    gap: 10px;
    padding-top: 4px;
  }

  .tube-pin {
    width: 4px;
    height: 8px;
    background: #b8860b;
    border-radius: 0 0 2px 2px;
  }

  .rec-label {
    margin-top: 8px;
    font-family: monospace;
    font-size: 14px;
    font-weight: bold;
    color: #ff4444;
    text-shadow: 0 0 8px rgba(255, 68, 68, 0.8), 0 0 16px rgba(255, 0, 0, 0.4);
    animation: rec-blink 1s steps(1) infinite;
    letter-spacing: 3px;
  }

  @keyframes rec-blink {
    0%, 70% { opacity: 1; }
    71%, 100% { opacity: 0; }
  }

  /* ============================================
     PROCESSING — Steampunk Gears (Factorio)
     ============================================ */
  .gears-container {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .gears-svg {
    width: 180px;
    height: 180px;
    filter: drop-shadow(0 0 8px rgba(184, 134, 11, 0.4));
  }

  .gear-big {
    animation: spin-cw 4s linear infinite;
  }

  .gear-small {
    animation: spin-ccw 2.7s linear infinite;
  }

  .gear-tiny {
    animation: spin-cw 3.2s linear infinite;
  }

  @keyframes spin-cw {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  @keyframes spin-ccw {
    from { transform: rotate(0deg); }
    to { transform: rotate(-360deg); }
  }

  /* Sparks at gear contact points */
  .spark {
    fill: #ffb74d;
    filter: drop-shadow(0 0 3px #ff6f00);
  }

  .s1 { animation: spark-flash 0.3s ease-out infinite; }
  .s2 { animation: spark-flash 0.4s ease-out 0.15s infinite; }
  .s3 { animation: spark-flash 0.25s ease-out 0.08s infinite; }
  .s4 { animation: spark-flash 0.35s ease-out 0.22s infinite; }
  .s5 { animation: spark-flash 0.28s ease-out 0.12s infinite; }
  .s6 { animation: spark-flash 0.32s ease-out 0.18s infinite; }
  .s7 { animation: spark-flash 0.22s ease-out 0.05s infinite; }

  @keyframes spark-flash {
    0% {
      opacity: 1;
      r: 2.5;
      fill: #fff;
      filter: drop-shadow(0 0 6px #ff6f00) drop-shadow(0 0 12px #ff4500);
    }
    30% {
      opacity: 0.8;
      fill: #ffb74d;
    }
    100% {
      opacity: 0;
      r: 0.5;
      fill: #ff6f00;
      filter: none;
    }
  }

  .progress-label {
    font-family: monospace;
    font-size: 16px;
    font-weight: bold;
    color: #b8860b;
    text-shadow: 0 0 6px rgba(184, 134, 11, 0.6);
    margin-top: 2px;
    letter-spacing: 2px;
    text-align: center;
  }

  .progress-label.dots {
    animation: dots-pulse 1.5s ease-in-out infinite;
  }

  @keyframes dots-pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 1; }
  }
</style>
