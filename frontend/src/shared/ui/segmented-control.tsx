import type { ComponentType } from 'react';
import type { LucideProps } from 'lucide-react';

interface SegmentedOption<TValue extends string> {
  value: TValue;
  label: string;
  mobileLabel?: string;
  icon?: ComponentType<LucideProps>;
}

interface SegmentedControlProps<TValue extends string> {
  ariaLabel: string;
  options: readonly SegmentedOption<TValue>[];
  value: TValue;
  onChange: (value: TValue) => void;
  className?: string;
}

export function SegmentedControl<TValue extends string>({
  ariaLabel,
  options,
  value,
  onChange,
  className = '',
}: SegmentedControlProps<TValue>) {
  return (
    <div className={`segmented-control ${className}`.trim()} role="group" aria-label={ariaLabel}>
      {options.map((option) => {
        const Icon = option.icon;
        const isActive = option.value === value;

        return (
          <button
            key={option.value}
            className={
              isActive
                ? 'segmented-control__item segmented-control__item--active'
                : 'segmented-control__item'
            }
            type="button"
            aria-label={option.label}
            title={option.mobileLabel ? option.label : undefined}
            aria-pressed={isActive}
            onClick={() => onChange(option.value)}
          >
            {Icon ? <Icon aria-hidden="true" size={15} strokeWidth={2} /> : null}
            <span className={option.mobileLabel ? 'segmented-control__label--full' : undefined}>{option.label}</span>
            {option.mobileLabel ? <span className="segmented-control__label--mobile" aria-hidden="true">{option.mobileLabel}</span> : null}
          </button>
        );
      })}
    </div>
  );
}
