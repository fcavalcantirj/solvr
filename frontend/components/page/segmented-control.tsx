import { cn } from '@/lib/utils';

// One square segmented control for every window selector: the options and their
// labels are the caller's (the API's), the pressed one is filled in ink. The group's
// label is the API's too; a page that has none passes none rather than inventing one.

export type SegmentedOption = { value: string; label: string };

export function SegmentedControl({
  label,
  labelledBy,
  options,
  value,
  onSelect,
  disabled = false,
  className,
}: {
  label?: string;
  /** The id of a visible heading that names the group, when the API sent no label. */
  labelledBy?: string;
  options: SegmentedOption[];
  value: string;
  onSelect: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <div
      role="group"
      aria-label={label}
      aria-labelledby={label ? undefined : labelledBy}
      className={cn('inline-flex border border-border divide-x divide-border bg-background', className)}
    >
      {options.map((option) => {
        const pressed = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={pressed}
            disabled={disabled}
            onClick={() => onSelect(option.value)}
            className={cn(
              'px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] transition-colors disabled:opacity-50 focus-visible:relative focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground',
              pressed
                ? 'bg-foreground text-background'
                : 'text-muted-foreground hover:bg-secondary hover:text-foreground',
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
