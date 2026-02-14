<script lang="ts">
  import QuaternionScenePlayer from '../../components/QuaternionScenePlayer.svelte';
  export let icon = 'success_burst'; // default
  export let size = 24;

  import successData from '../presets/success_burst.json';
  import errorData from '../presets/error_shake.json';

  let data;
  $: {
    if (icon === 'success_burst') data = successData;
    else if (icon === 'error_shake') data = errorData;
    else data = successData;
  }
</script>

<div class="qgif-icon" style="width: {size}px; height: {size}px;">
  <QuaternionScenePlayer qgifData={data} width={size} height={size} />
</div>
