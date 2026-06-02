import { writable } from "svelte/store";

export type TextScalePreset = "standard" | "comfortable" | "large";

const STORAGE_KEY = "asymmflow.textScale";
const MIN_SCALE = 1;
const MAX_SCALE = 1.25;
const TEXT_SIZE_BASES = [8, 9, 10, 11, 12, 12.5, 13, 14, 15, 16, 18, 20, 22, 24, 28, 32, 48] as const;

export const textScale = writable(1);
export const textScalePreset = writable<TextScalePreset>("standard");

function clampScale(value: number) {
    if (!Number.isFinite(value)) return 1;
    return Math.min(MAX_SCALE, Math.max(MIN_SCALE, value));
}

function presetForScale(scale: number): TextScalePreset {
    if (scale >= 1.2) return "large";
    if (scale >= 1.08) return "comfortable";
    return "standard";
}

export function scaleForPreset(preset: TextScalePreset) {
    if (preset === "large") return 1.25;
    if (preset === "comfortable") return 1.12;
    return 1;
}

function textSizeToken(size: number) {
    return `--text-${String(size).replace(".", "-")}`;
}

function setScaledPixelVar(root: HTMLElement, name: string, basePixels: number, scale: number) {
    root.style.setProperty(name, `${(basePixels * scale).toFixed(1)}px`);
}

function applyTextScale(scale: number) {
    if (typeof document === "undefined") return;
    const clamped = clampScale(scale);
    const preset = presetForScale(clamped);
    const root = document.documentElement;
    const navScale = Math.min(clamped, 1.18);

    root.style.setProperty("--ui-font-scale", clamped.toFixed(2));
    for (const size of TEXT_SIZE_BASES) {
        setScaledPixelVar(root, textSizeToken(size), size, clamped);
    }

    setScaledPixelVar(root, "--body-text-size-scaled", 14, clamped);
    setScaledPixelVar(root, "--control-text-size-scaled", 13, clamped);
    setScaledPixelVar(root, "--tab-text-size-scaled", 12.5, navScale);
    setScaledPixelVar(root, "--meta-text-size-scaled", 11, clamped);
    setScaledPixelVar(root, "--card-title-size-scaled", 15, clamped);
    setScaledPixelVar(root, "--section-title-size-scaled", 16, clamped);
    setScaledPixelVar(root, "--page-title-size-scaled", 24, clamped);
    setScaledPixelVar(root, "--kpi-value-size-scaled", 24, clamped);

    setScaledPixelVar(root, "--label-size", 11, clamped);
    setScaledPixelVar(root, "--meta-size", 11, clamped);
    setScaledPixelVar(root, "--section-title-size", 16, clamped);
    setScaledPixelVar(root, "--page-title-size", 24, clamped);
    setScaledPixelVar(root, "--table-text-size", 13, clamped);
    setScaledPixelVar(root, "--table-row-height", 44, clamped);
    setScaledPixelVar(root, "--table-row-min-height", 44, clamped);
    setScaledPixelVar(root, "--modal-title-size", 22, clamped);
    setScaledPixelVar(root, "--modal-label-size", 12, clamped);
    setScaledPixelVar(root, "--modal-body-size", 14, clamped);
    setScaledPixelVar(root, "--kpi-value-size", 32, clamped);
    setScaledPixelVar(root, "--north-star-kpi-value-size", 48, clamped);

    root.dataset.textScale = preset;
}

export function setTextScale(value: number) {
    const scale = clampScale(value);
    const preset = presetForScale(scale);
    textScale.set(scale);
    textScalePreset.set(preset);
    applyTextScale(scale);
    if (typeof localStorage !== "undefined") {
        localStorage.setItem(STORAGE_KEY, scale.toFixed(2));
    }
}

export function setTextScalePreset(preset: TextScalePreset) {
    setTextScale(scaleForPreset(preset));
}

export function initTextScale() {
    let scale = 1;
    if (typeof localStorage !== "undefined") {
        const stored = Number(localStorage.getItem(STORAGE_KEY));
        if (Number.isFinite(stored)) scale = stored;
    }
    setTextScale(scale);
}
