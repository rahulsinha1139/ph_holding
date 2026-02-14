<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { fade, fly } from 'svelte/transition';

  export let message: string = '';
  export let type: 'success' | 'warning' | 'danger' | 'info' = 'info';
  export let duration: number = 4000; // Auto-dismiss after 4s
  export let showBrush: boolean = true; // Show sumi-e brush stroke

  const dispatch = createEventDispatcher();
  
  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null;
  let breathT = 0;
  let animationId: number;
  let dismissTimeout: ReturnType<typeof setTimeout>;

  // Wabi-sabi noise function (imperfect by design)
  function noise(x: number, y: number): number {
    return Math.sin(x * 12.9898 + y * 78.233) * 43758.5453 % 1;
  }

  // Sumi-e brush stroke - the heart of wabi-sabi
  function sumiEStroke(
    x1: number, y1: number,
    x2: number, y2: number,
    options: { variability?: number; fade?: number; width?: number; segments?: number; color?: string } = {}
  ) {
    if (!ctx) return;
    
    const {
      variability = 0.3,
      fade: fadeFactor = 0.97,
      width = 4,
      segments = 60,
      color = 'rgba(255,255,255,0.6)'
    } = options;

    const points: Array<{x: number, y: number}> = [];

    for (let i = 0; i <= segments; i++) {
      const t = i / segments;
      let x = x1 + (x2 - x1) * t;
      let y = y1 + (y2 - y1) * t;

      // Imperfection jitter - the beauty of impermanence
      const jx = (noise(x * 0.01, y * 0.01) * 2 - 1) * variability * 10;
      const jy = (noise(y * 0.02, x * 0.02) * 2 - 1) * variability * 10;

      points.push({ x: x + jx, y: y + jy });
    }

    ctx.lineCap = 'round';
    for (let i = 0; i < points.length - 1; i++) {
      const p = points[i];
      const q = points[i + 1];

      // Parse color and apply fade
      ctx.strokeStyle = color.replace(/[\d.]+\)$/, `${fadeFactor ** i * 0.6})`);
      ctx.lineWidth = width * (1 + (Math.random() * variability - variability / 2));

      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(q.x, q.y);
      ctx.stroke();
    }
  }

  // Imperfect breathing circle - wabi-sabi mandala
  function wabiCircle(cx: number, cy: number, radius: number, wobble: number = 0.02) {
    if (!ctx) return;
    
    ctx.beginPath();
    const strokes = 120;

    for (let i = 0; i <= strokes; i++) {
      const theta = (i / strokes) * Math.PI * 2;
      const r = radius + Math.sin(theta * 13.7 + breathT) * wobble * radius;

      const x = cx + r * Math.cos(theta);
      const y = cy + r * Math.sin(theta);

      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }

    ctx.strokeStyle = getTypeColor(0.3);
    ctx.lineWidth = 1;
    ctx.stroke();
  }

  function getTypeColor(alpha: number = 1): string {
    const colors = {
      success: `rgba(74, 222, 128, ${alpha})`,  // Green
      warning: `rgba(251, 191, 36, ${alpha})`,  // Gold
      danger: `rgba(248, 113, 113, ${alpha})`,  // Red
      info: `rgba(147, 197, 253, ${alpha})`     // Blue
    };
    return colors[type];
  }

  function animate() {
    if (!ctx || !canvas) return;
    
    breathT += 0.015;
    
    // Clear with slight trail effect
    ctx.fillStyle = 'rgba(11, 11, 11, 0.1)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    // Draw breathing circle on the left
    wabiCircle(30, canvas.height / 2, 15, 0.04);

    animationId = requestAnimationFrame(animate);
  }

  function drawInitialStroke() {
    if (!ctx || !canvas) return;
    
    // Clear canvas
    ctx.fillStyle = '#0b0b0b';
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    if (showBrush) {
      // Single elegant sumi-e stroke across the toast
      sumiEStroke(
        10, canvas.height * 0.7,
        canvas.width - 10, canvas.height * 0.3,
        { variability: 0.25, width: 3, color: getTypeColor(0.4) }
      );
    }
  }

  function dismiss() {
    dispatch('dismiss');
  }

  onMount(() => {
    console.log('WabiSabiToast mounted:', message, type);
    try {
      if (canvas) {
        ctx = canvas.getContext('2d');
        canvas.width = 320;
        canvas.height = 80;

        drawInitialStroke();
        animate();
      } else {
        console.warn('Canvas not available for toast');
      }
    } catch (err) {
      console.error('Toast canvas initialization failed:', err);
    }

    // Auto-dismiss
    if (duration > 0) {
      dismissTimeout = setTimeout(dismiss, duration);
    }
  });

  onDestroy(() => {
    if (animationId) cancelAnimationFrame(animationId);
    if (dismissTimeout) clearTimeout(dismissTimeout);
  });
</script>

<button
  class="wabi-toast {type}"
  in:fly={{ y: -20, duration: 300 }}
  out:fade={{ duration: 200 }}
  on:click={dismiss}
  aria-label="Dismiss notification: {message}"
  data-testid="{type}-toast"
>
  <canvas bind:this={canvas} class="toast-canvas"></canvas>
  <div class="toast-content">
    <span class="toast-icon">
      {#if type === 'success'}OK{/if}
      {#if type === 'warning'}!{/if}
      {#if type === 'danger'}X{/if}
      {#if type === 'info'}i{/if}
    </span>
    <span class="toast-message">{message}</span>
  </div>
</button>

<style>
  .wabi-toast {
    position: relative;
    width: 320px;
    height: 80px;
    border-radius: 8px;
    overflow: hidden;
    cursor: pointer;
    border: none;
    padding: 0;
    text-align: left;
    background: #0b0b0b;
    box-shadow: 
      0 4px 20px rgba(0, 0, 0, 0.4),
      0 0 40px rgba(197, 160, 89, 0.03);
    transition: transform 0.2s ease, box-shadow 0.2s ease;
  }

  .wabi-toast:hover {
    transform: translateY(-2px);
    box-shadow: 
      0 6px 24px rgba(0, 0, 0, 0.5),
      0 0 50px rgba(197, 160, 89, 0.06);
  }

  .toast-canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .toast-content {
    position: relative;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 20px;
    height: 100%;
    box-sizing: border-box;
  }

  .toast-icon {
    font-size: 20px;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .toast-message {
    font-family: 'Georgia', serif;
    font-size: 14px;
    color: #e6e6e6;
    line-height: 1.4;
    letter-spacing: 0.3px;
  }

  /* Type-specific styles */
  .wabi-toast.success .toast-icon {
    background: rgba(74, 222, 128, 0.2);
    color: #4ade80;
  }

  .wabi-toast.warning .toast-icon {
    background: rgba(251, 191, 36, 0.2);
    color: #fbbf24;
  }

  .wabi-toast.danger .toast-icon {
    background: rgba(248, 113, 113, 0.2);
    color: #f87171;
  }

  .wabi-toast.info .toast-icon {
    background: rgba(147, 197, 253, 0.2);
    color: #93c5fd;
  }

  /* Kintsugi gold border accent */
  .wabi-toast::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 3px;
    background: linear-gradient(
      to bottom,
      transparent,
      var(--accent-color, #c5a059) 20%,
      var(--accent-color, #c5a059) 80%,
      transparent
    );
    opacity: 0.6;
  }

  .wabi-toast.success { --accent-color: #4ade80; }
  .wabi-toast.warning { --accent-color: #fbbf24; }
  .wabi-toast.danger { --accent-color: #f87171; }
  .wabi-toast.info { --accent-color: #93c5fd; }
</style>
