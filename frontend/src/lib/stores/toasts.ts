// ============================================================
// WABI-SABI TOAST STORE - Imperfect Notifications with Grace
// ============================================================

import { writable } from 'svelte/store';

export type ToastType = 'success' | 'warning' | 'danger' | 'info';

export interface Toast {
  id: string;
  message: string;
  type: ToastType;
  duration: number;
  showBrush: boolean;
}

// Generate unique ID with wabi-sabi imperfection
function generateId(): string {
  return `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

// Create the writable store
const { subscribe, update } = writable<Toast[]>([]);

// Add a toast with full options
function add(options: Partial<Toast> & { message: string }): string {
  const newToast: Toast = {
    id: generateId(),
    message: options.message,
    type: options.type || 'info',
    duration: options.duration ?? 4000,
    showBrush: options.showBrush ?? true,
  };

  console.log('Toast added:', newToast.message, newToast.type, newToast);
  console.trace('Toast add called from:');
  update(toasts => {
    const newArray = [...toasts, newToast];
    console.log('Toast array updated, new length:', newArray.length);
    return newArray;
  });
  return newToast.id;
}

// Remove a toast by ID
function dismiss(id: string): void {
  update(toasts => toasts.filter(t => t.id !== id));
}

// Clear all toasts
function clear(): void {
  update(() => []);
}

// Singleton store with methods
export const toasts = {
  subscribe,
  add,
  dismiss,
  clear,
  success: (message: string, duration?: number) => add({ message, type: 'success', duration }),
  warning: (message: string, duration?: number) => add({ message, type: 'warning', duration }),
  danger: (message: string, duration?: number) => add({ message, type: 'danger', duration }),
  info: (message: string, duration?: number) => add({ message, type: 'info', duration }),
};

// Re-export convenience functions for direct import
export const toast = {
  success: (message: string, duration?: number) => add({ message, type: 'success', duration }),
  warning: (message: string, duration?: number) => add({ message, type: 'warning', duration }),
  danger: (message: string, duration?: number) => add({ message, type: 'danger', duration }),
  info: (message: string, duration?: number) => add({ message, type: 'info', duration }),
  dismiss,
  clear,
};
