<script>
  import { onMount } from "svelte";

  export let text = "";
  export let duration = 600;
  export let delay = 0;
  export let className = "";

  let displayChars = [];
  let mounted = false;

  $: if (text && mounted) {
    animateText();
  }

  onMount(() => {
    mounted = true;
    animateText();
  });

  function animateText() {
    const chars = text.toString().split('');
    displayChars = chars.map((char, i) => ({
      char,
      delay: delay + (i * duration) / chars.length
    }));
  }
</script>

<span class="text-bloom {className}">
  {#each displayChars as { char, delay: charDelay }, i (i + text)}
    <span class="bloom-char inline-block" style="animation-delay: {charDelay}ms">
      {char}
    </span>
  {/each}
</span>

<style>
  .text-bloom {
    display: inline-flex;
  }

  .bloom-char {
    opacity: 0;
    transform: scale(0.5) translateY(10px);
    animation: bloom 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94) forwards;
  }

  @keyframes bloom {
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }
</style>
