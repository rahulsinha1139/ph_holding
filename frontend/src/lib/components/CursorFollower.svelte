<script>
    import { onMount, onDestroy } from "svelte";

    let cursorX = 0;
    let cursorY = 0;
    let targetX = 0;
    let targetY = 0;
    let visible = false;
    let mode = "default"; // "default" | "text" | "magnetic"
    let magneticScale = 1;
    let magneticWidth = 6;
    let magneticHeight = 6;
    let magneticRadius = 50; // percent

    let animFrame;
    let magnetTarget = null; // The element we're magnetically attached to

    const LERP_FACTOR = 0.15; // Smooth follow (lower = more lag)
    const MAGNETIC_RANGE = 80; // px distance to start magnetic pull

    function lerp(a, b, t) {
        return a + (b - a) * t;
    }

    function handleMouseMove(e) {
        targetX = e.clientX;
        targetY = e.clientY;
        visible = true;

        const target = e.target;

        // Detect text elements for I-beam mode
        const isText = target.closest('p, span, h1, h2, h3, h4, h5, h6, label, .text-content, td');
        const isInput = target.closest('input, textarea, [contenteditable]');

        // Detect interactive elements for magnetic mode
        const isInteractive = target.closest(
            'button, a, [role="button"], .nav-link, .tab, .card, .btn, .quick-link'
        );

        if (isInput) {
            mode = "text";
            magnetTarget = null;
            magneticWidth = 2;
            magneticHeight = 20;
            magneticRadius = 2;
            magneticScale = 1;
        } else if (isText && !isInteractive) {
            mode = "text";
            magnetTarget = null;
            magneticWidth = 2;
            magneticHeight = 18;
            magneticRadius = 2;
            magneticScale = 1;
        } else if (isInteractive) {
            mode = "magnetic";
            magnetTarget = isInteractive;
            const rect = isInteractive.getBoundingClientRect();
            magneticWidth = rect.width + 12;
            magneticHeight = rect.height + 8;
            magneticRadius = Math.min(rect.height / 2 + 4, 20);
            magneticScale = 1;
        } else {
            mode = "default";
            magnetTarget = null;
            magneticWidth = 6;
            magneticHeight = 6;
            magneticRadius = 50;
            magneticScale = 1;
        }
    }

    function handleMouseLeave() {
        visible = false;
    }

    function animate() {
        if (mode === "magnetic" && magnetTarget) {
            // Snap cursor to center of the magnetic target
            const rect = magnetTarget.getBoundingClientRect();
            const centerX = rect.left + rect.width / 2;
            const centerY = rect.top + rect.height / 2;

            cursorX = lerp(cursorX, centerX, 0.2);
            cursorY = lerp(cursorY, centerY, 0.2);
        } else {
            // Smooth follow for default/text mode
            cursorX = lerp(cursorX, targetX, LERP_FACTOR);
            cursorY = lerp(cursorY, targetY, LERP_FACTOR);
        }

        animFrame = requestAnimationFrame(animate);
    }

    function handleMouseDown() {
        magneticScale = 0.9;
        setTimeout(() => { magneticScale = 1; }, 150);
    }

    onMount(() => {
        document.addEventListener("mousemove", handleMouseMove);
        document.addEventListener("mouseleave", handleMouseLeave);
        document.addEventListener("mousedown", handleMouseDown);
        animFrame = requestAnimationFrame(animate);
    });

    onDestroy(() => {
        document.removeEventListener("mousemove", handleMouseMove);
        document.removeEventListener("mouseleave", handleMouseLeave);
        document.removeEventListener("mousedown", handleMouseDown);
        if (animFrame) cancelAnimationFrame(animFrame);
    });
</script>

<div
    class="cursor-follower"
    class:visible
    class:mode-text={mode === "text"}
    class:mode-magnetic={mode === "magnetic"}
    style="
        left: {cursorX}px;
        top: {cursorY}px;
        width: {magneticWidth}px;
        height: {magneticHeight}px;
        border-radius: {magneticRadius}px;
        transform: translate(-50%, -50%) scale({magneticScale});
    "
></div>

<style>
    .cursor-follower {
        position: fixed;
        pointer-events: none;
        z-index: 9999;
        background: #000;
        opacity: 0;
        transition:
            width 0.25s cubic-bezier(0.34, 1.56, 0.64, 1),
            height 0.25s cubic-bezier(0.34, 1.56, 0.64, 1),
            border-radius 0.2s ease,
            opacity 0.15s ease,
            background 0.2s ease;
        will-change: left, top, width, height;
    }

    .cursor-follower.visible {
        opacity: 1;
    }

    /* Default: small black dot */
    .cursor-follower:not(.mode-text):not(.mode-magnetic) {
        background: #000;
    }

    /* Text mode: thin I-beam line */
    .cursor-follower.mode-text {
        background: #1D1D1F;
        opacity: 0.7;
    }

    /* Magnetic mode: translucent gray encompassing the element */
    .cursor-follower.mode-magnetic {
        background: rgba(0, 0, 0, 0.06);
        border: 1px solid rgba(0, 0, 0, 0.08);
        mix-blend-mode: normal;
    }
</style>
