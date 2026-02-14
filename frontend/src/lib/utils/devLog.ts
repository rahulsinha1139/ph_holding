/**
 * Development-only logging utility
 * Automatically strips logs in production builds
 */

export const devLog = {
  log: (...args: any[]) => {
    if (import.meta.env.DEV) {
      console.log(...args);
    }
  },

  info: (...args: any[]) => {
    if (import.meta.env.DEV) {
      console.info(...args);
    }
  },

  warn: (...args: any[]) => {
    if (import.meta.env.DEV) {
      console.warn(...args);
    }
  },

  error: (...args: any[]) => {
    // Always log errors, even in production
    console.error(...args);
  },

  debug: (...args: any[]) => {
    if (import.meta.env.DEV) {
      console.debug(...args);
    }
  }
};
