'use client';

/**
 * SearchBar component
 * Search input with icon, keyboard shortcut support (Cmd/Ctrl+K)
 * Per SPEC.md Part 4.2: Global Elements - Search Bar
 */

import { useState, useRef, useEffect, useCallback } from 'react';

interface SearchBarProps {
  /** Controlled value */
  value?: string;
  /** Change handler for controlled mode */
  onChange?: (value: string) => void;
  /** Search handler called on form submit */
  onSearch?: (query: string) => void;
  /** Custom placeholder text */
  placeholder?: string;
  /** Disable the input */
  disabled?: boolean;
  /** Custom CSS classes */
  className?: string;
}

/**
 * SearchBar provides a search input with keyboard shortcut support
 */
export default function SearchBar({
  value: controlledValue,
  onChange,
  onSearch,
  placeholder = 'Search Solvr...',
  disabled = false,
  className = '',
}: SearchBarProps) {
  // Internal state for uncontrolled mode
  const [internalValue, setInternalValue] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  // Determine if controlled
  const isControlled = controlledValue !== undefined;
  const value = isControlled ? controlledValue : internalValue;

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;
    if (isControlled) {
      onChange?.(newValue);
    } else {
      setInternalValue(newValue);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedValue = value.trim();
    if (trimmedValue && onSearch) {
      onSearch(trimmedValue);
    }
  };

  const handleClear = () => {
    if (isControlled) {
      onChange?.('');
    } else {
      setInternalValue('');
    }
    inputRef.current?.focus();
  };

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    // Cmd+K (Mac) or Ctrl+K (Windows/Linux) to focus search
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      inputRef.current?.focus();
    }
  }, []);

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    // Escape to blur
    if (e.key === 'Escape') {
      inputRef.current?.blur();
    }
  };

  // Register global keyboard shortcut
  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [handleKeyDown]);

  return (
    <form onSubmit={handleSubmit} className={`relative ${className}`}>
      <div className="relative border border-[var(--border)] rounded-md bg-[var(--background-secondary)] focus-within:border-[var(--color-primary)] focus-within:ring-1 focus-within:ring-[var(--color-primary)] transition-colors">
        {/* Search Icon */}
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
          <svg
            className="h-4 w-4 text-[var(--foreground-muted)]"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>

        {/* Input */}
        <label htmlFor="search-input" className="sr-only">
          Search
        </label>
        <input
          ref={inputRef}
          id="search-input"
          type="search"
          role="searchbox"
          value={value}
          onChange={handleChange}
          onKeyDown={handleInputKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          className="block w-full rounded-md border-0 bg-transparent py-2 pl-10 pr-20 text-sm text-[var(--foreground)] placeholder-[var(--foreground-muted)] focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        />

        {/* Right side: Clear button and shortcut hint */}
        <div className="absolute inset-y-0 right-0 flex items-center gap-2 pr-3">
          {/* Clear button - only show when there's text */}
          {value && (
            <button
              type="button"
              onClick={handleClear}
              aria-label="Clear search"
              className="text-[var(--foreground-muted)] hover:text-[var(--foreground)] transition-colors"
            >
              <svg
                className="h-4 w-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          )}

          {/* Keyboard shortcut hint */}
          <kbd className="hidden sm:inline-flex items-center gap-1 rounded border border-[var(--border)] bg-[var(--background)] px-1.5 py-0.5 text-xs text-[var(--foreground-muted)]">
            <span className="text-xs">⌘K</span>
          </kbd>
        </div>
      </div>
    </form>
  );
}
