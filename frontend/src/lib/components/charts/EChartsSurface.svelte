<script lang="ts">
    import { createEventDispatcher, onDestroy, onMount, tick } from "svelte";
    import * as echarts from "echarts";
    import type { ECharts, EChartsOption } from "echarts";

    export let option: EChartsOption;
    export let height = 300;
    export let loading = false;
    export let ariaLabel = "Interactive chart";

    const dispatch = createEventDispatcher<{
        hover: any;
        leave: any;
        click: any;
    }>();

    let element: HTMLDivElement;
    let chart: ECharts | null = null;
    let resizeObserver: ResizeObserver | null = null;

    function resizeChart() {
        chart?.resize();
    }

    async function applyOption() {
        if (!chart || !option) return;
        await tick();
        chart.setOption(option, true);
        resizeChart();
        if (loading) {
            chart.showLoading("default", {
                text: "",
                color: "#3E8F47",
                maskColor: "rgba(247, 250, 247, 0.6)",
            });
        } else {
            chart.hideLoading();
        }
    }

    onMount(() => {
        chart = echarts.init(element, undefined, { renderer: "canvas" });
        chart.on("mouseover", (params) => dispatch("hover", params));
        chart.on("mouseout", (params) => dispatch("leave", params));
        chart.on("click", (params) => dispatch("click", params));

        resizeObserver = new ResizeObserver(resizeChart);
        resizeObserver.observe(element);
        window.addEventListener("resize", resizeChart);
        applyOption();
    });

    onDestroy(() => {
        window.removeEventListener("resize", resizeChart);
        resizeObserver?.disconnect();
        chart?.dispose();
        chart = null;
    });

    $: if (chart && option) applyOption();
</script>

<div
    bind:this={element}
    class="echarts-surface"
    style={`height: ${height}px;`}
    role="img"
    aria-label={ariaLabel}
></div>

<style>
    .echarts-surface {
        width: 100%;
        min-width: 0;
    }
</style>
