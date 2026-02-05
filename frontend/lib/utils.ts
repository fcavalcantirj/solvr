import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Format large numbers in a readable way (1.2K, 48.2K, 1.5M format)
 * @param n - The number to format
 * @returns Formatted string
 */
export function formatCount(n: number): string {
  if (n < 0) return '-' + formatCount(-n);
  if (n < 1000) return n.toString();
  if (n < 10000) {
    const k = n / 1000;
    return k.toFixed(1).replace(/\.0$/, '') + 'K';
  }
  if (n < 1000000) {
    const k = n / 1000;
    return k >= 100 ? Math.round(k) + 'K' : k.toFixed(1).replace(/\.0$/, '') + 'K';
  }
  if (n < 10000000) {
    const m = n / 1000000;
    return m.toFixed(1).replace(/\.0$/, '') + 'M';
  }
  if (n < 1000000000) {
    const m = n / 1000000;
    return m >= 100 ? Math.round(m) + 'M' : m.toFixed(1).replace(/\.0$/, '') + 'M';
  }
  const b = n / 1000000000;
  return b.toFixed(1).replace(/\.0$/, '') + 'B';
}
