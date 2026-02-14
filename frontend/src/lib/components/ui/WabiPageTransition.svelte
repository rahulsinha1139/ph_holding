<script lang="ts">
  /**
   * Wabi-Sabi Page Transition
   * Smooth, zen-like transitions between screens
   * Like turning a page in a beautiful book
   */
  import { fade, fly } from 'svelte/transition';
  import { cubicOut, quintOut } from 'svelte/easing';
  
  export let key: string | number;
  export let direction: 'up' | 'down' | 'left' | 'right' | 'fade' = 'fade';
  export let duration: number = 300;
  
  const transitions = {
    up: { y: 20, x: 0 },
    down: { y: -20, x: 0 },
    left: { y: 0, x: 20 },
    right: { y: 0, x: -20 },
    fade: { y: 0, x: 0 },
  };
  
  $: trans = transitions[direction];
</script>

{#key key}
  <div
    class="page-transition"
    in:fly={{ y: trans.y, x: trans.x, duration, easing: quintOut, delay: 50 }}
    out:fade={{ duration: duration / 2 }}
  >
    <slot />
  </div>
{/key}

<style>
  .page-transition {
    width: 100%;
    height: 100%;
  }
</style>
