import { writable } from "svelte/store";

export type TextScalePreset = "standard" | "comfortable" | "large";

const STORAGE_KEY = "asymmflow.textScale";
const MIN_SCALE = 1;
const MAX_SCALE = 1.25;

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

function applyTextScale(scale: number) {
    if (typeof document === "undefined") return;
    const clamped = clampScale(scale);
    const preset = presetForScale(clamped);
    const layoutScale = Math.min(clamped, 1.2);
    const modalScale = Math.min(clamped, 1.25);
    const navScale = Math.min(clamped, 1.18);
    document.documentElement.style.setProperty("--ui-font-scale", clamped.toFixed(2));
    document.documentElement.style.setProperty("--body-text-size-scaled", `${(14 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--control-text-size-scaled", `${(13 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--tab-text-size-scaled", `${(12.5 * navScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--meta-text-size-scaled", `${(12 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--card-title-size-scaled", `${(15 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--kpi-value-size-scaled", `${(24 * modalScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--table-text-size", `${(13 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--table-row-height", `${(44 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--table-row-min-height", `${(44 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--modal-label-size", `${(12 * modalScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--modal-body-size", `${(14 * modalScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--section-title-size-scaled", `${(16 * layoutScale).toFixed(1)}px`);
    document.documentElement.style.setProperty("--page-title-size-scaled", `${(24 * layoutScale).toFixed(1)}px`);
    document.documentElement.dataset.textScale = preset;
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
