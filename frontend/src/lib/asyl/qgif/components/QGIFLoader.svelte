<script lang="ts">
  import { onMount } from 'svelte';
  // Corrected path to QuaternionScenePlayer in .../src/lib/asyl/components/
  import QuaternionScenePlayer from '../../components/QuaternionScenePlayer.svelte';
  // Corrected path to presets in .../src/lib/asyl/qgif/presets/
  import loadingData from '../presets/loading_spinner.json';

  export let size = 100;
  export let color = "#c5a059"; // Kintsugi Gold

  let qgifData = { ...loadingData };
  qgifData.geometry.color = color;
</script>

<div class="qgif-loader" style="width: {size}px; height: {size}px;">
  <QuaternionScenePlayer {qgifData} width={size} height={size} loop={true} />
</div>

<style>
  .qgif-loader {
    display: flex;
    justify-content: center;
    align-items: center;
  }
</style>
