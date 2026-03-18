import '@testing-library/jest-dom'

// Polyfill ResizeObserver for jsdom (required by Radix UI components like Checkbox)
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};
