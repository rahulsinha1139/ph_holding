<script lang="ts">
    export let size = 112;
    export let compact = false;

    const piDigits = "31415926535897932384626433832795"
        .split("")
        .map((digit, index) => ({
            digit: Number(digit),
            index,
            rotation: index * (360 / 32),
        }));

    const ringRadii = [18, 32, 46, 60, 74];

    function buildLogSpiral(offset = 0) {
        const points: string[] = [];
        for (let i = 0; i <= 230; i += 1) {
            const t = i / 13;
            const radius = 2.2 * Math.exp(0.205 * t);
            const x = Math.cos(t + offset) * radius;
            const y = Math.sin(t + offset) * radius;
            points.push(`${i === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`);
        }
        return points.join(" ");
    }

    const spiralA = buildLogSpiral(0);
    const spiralB = buildLogSpiral(Math.PI);
    const loaderId = Math.random().toString(36).slice(2);
    const coreGradientId = `math-loader-core-${loaderId}`;
    const spiralGradientId = `math-loader-spiral-${loaderId}`;

    $: loaderStyle = `--loader-size: ${size}px;`;
</script>

<div class:compact class="math-loader" style={loaderStyle} aria-hidden="true">
    <svg viewBox="-96 -96 192 192" role="img">
        <defs>
            <radialGradient id={coreGradientId} cx="50%" cy="50%" r="50%">
                <stop offset="0%" stop-color="#ffffff" stop-opacity="0.95" />
                <stop offset="48%" stop-color="#eef5ee" stop-opacity="0.62" />
                <stop offset="100%" stop-color="#25312b" stop-opacity="0.08" />
            </radialGradient>
            <linearGradient id={spiralGradientId} x1="-70" y1="-70" x2="70" y2="70">
                <stop offset="0%" stop-color="#25312b" />
                <stop offset="52%" stop-color="#3e8f47" />
                <stop offset="100%" stop-color="#c98d2e" />
            </linearGradient>
        </defs>

        <circle class="field" r="86" fill={`url(#${coreGradientId})`} />

        <g class="wormhole">
            {#each ringRadii as radius, index}
                <circle
                    class="ring"
                    r={radius}
                    style={`--ring-index: ${index}; --ring-radius: ${radius};`}
                />
            {/each}
        </g>

        <g class="pi-bars">
            {#each piDigits as bar}
                <rect
                    class="pi-bar"
                    x="-1.4"
                    y={-82 - bar.digit * 0.9}
                    width="2.8"
                    height={7 + bar.digit * 2.1}
                    rx="1.4"
                    transform={`rotate(${bar.rotation})`}
                    style={`--bar-index: ${bar.index};`}
                />
            {/each}
        </g>

        <g class="spirals">
            <path class="spiral spiral-a" d={spiralA} stroke={`url(#${spiralGradientId})`} />
            <path class="spiral spiral-b" d={spiralB} stroke={`url(#${spiralGradientId})`} />
        </g>

        <g class="axes">
            <path d="M -72 0 H 72" />
            <path d="M 0 -72 V 72" />
        </g>
    </svg>
    <div class="core">π</div>
</div>

<style>
    .math-loader {
        width: var(--loader-size);
        height: var(--loader-size);
        position: relative;
        display: inline-grid;
        place-items: center;
        color: var(--onyx, #25312b);
        filter: drop-shadow(0 14px 30px rgba(37, 49, 43, 0.12));
    }

    svg {
        width: 100%;
        height: 100%;
        overflow: visible;
    }

    .field {
        stroke: rgba(37, 49, 43, 0.08);
        stroke-width: 1;
    }

    .wormhole {
        transform-origin: center;
        animation: loader-rotate 9s linear infinite;
    }

    .ring {
        fill: none;
        stroke: rgba(37, 49, 43, 0.24);
        stroke-width: 1.2;
        stroke-dasharray: calc(var(--ring-radius) * 0.55) calc(var(--ring-radius) * 0.34);
        animation: ring-breathe 2.8s ease-in-out infinite;
        animation-delay: calc(var(--ring-index) * -180ms);
    }

    .pi-bars {
        transform-origin: center;
        animation: loader-rotate-reverse 12s linear infinite;
    }

    .pi-bar {
        fill: #3e8f47;
        opacity: 0.28;
        transform-origin: center;
        animation: pi-pulse 1.8s ease-in-out infinite;
        animation-delay: calc(var(--bar-index) * -55ms);
    }

    .spirals {
        transform-origin: center;
        animation: loader-tilt 3.8s ease-in-out infinite;
    }

    .spiral {
        fill: none;
        stroke-width: 2.4;
        stroke-linecap: round;
        stroke-linejoin: round;
        stroke-dasharray: 380;
        stroke-dashoffset: 380;
        animation: spiral-trace 2.4s ease-in-out infinite;
    }

    .spiral-b {
        opacity: 0.58;
        animation-delay: -1.2s;
    }

    .axes path {
        fill: none;
        stroke: rgba(37, 49, 43, 0.12);
        stroke-width: 0.8;
        stroke-dasharray: 2 6;
    }

    .core {
        position: absolute;
        inset: 50% auto auto 50%;
        width: calc(var(--loader-size) * 0.3);
        height: calc(var(--loader-size) * 0.3);
        transform: translate(-50%, -50%);
        display: grid;
        place-items: center;
        border-radius: 50%;
        background: var(--onyx, #25312b);
        color: #ffffff;
        font-family: var(--font-display, system-ui);
        font-size: calc(var(--loader-size) * 0.15);
        font-weight: 700;
        box-shadow:
            inset 0 1px 0 rgba(255, 255, 255, 0.2),
            0 10px 22px rgba(37, 49, 43, 0.18);
    }

    .compact {
        filter: drop-shadow(0 8px 18px rgba(37, 49, 43, 0.1));
    }

    .compact .spiral {
        stroke-width: 2.8;
    }

    @keyframes loader-rotate {
        to {
            transform: rotate(360deg);
        }
    }

    @keyframes loader-rotate-reverse {
        to {
            transform: rotate(-360deg);
        }
    }

    @keyframes ring-breathe {
        0%,
        100% {
            opacity: 0.22;
            transform: scale(0.97);
        }
        50% {
            opacity: 0.5;
            transform: scale(1.03);
        }
    }

    @keyframes pi-pulse {
        0%,
        100% {
            opacity: 0.18;
        }
        45% {
            opacity: 0.72;
        }
    }

    @keyframes spiral-trace {
        0% {
            stroke-dashoffset: 380;
            opacity: 0.2;
        }
        48% {
            stroke-dashoffset: 0;
            opacity: 1;
        }
        100% {
            stroke-dashoffset: -380;
            opacity: 0.26;
        }
    }

    @keyframes loader-tilt {
        0%,
        100% {
            transform: rotate(-8deg) scale(0.96);
        }
        50% {
            transform: rotate(8deg) scale(1.02);
        }
    }

    @media (prefers-reduced-motion: reduce) {
        .wormhole,
        .pi-bars,
        .spirals,
        .ring,
        .pi-bar,
        .spiral {
            animation: none;
        }

        .spiral {
            stroke-dashoffset: 0;
        }
    }
</style>
