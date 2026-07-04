import type { ButtonHTMLAttributes, ComponentType } from 'react';
import type { LucideProps } from 'lucide-react';

type IconButtonVariant = 'ghost' | 'send' | 'modal';

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: ComponentType<LucideProps>;
  label: string;
  variant?: IconButtonVariant;
}

export function IconButton({
  icon: Icon,
  label,
  variant = 'ghost',
  className = '',
  type = 'button',
  ...buttonProps
}: IconButtonProps) {
  return (
    <button
      {...buttonProps}
      className={`icon-button icon-button--${variant} ${className}`.trim()}
      type={type}
      aria-label={label}
      title={label}
    >
      <Icon aria-hidden="true" size={variant === 'send' ? 18 : 17} strokeWidth={2} />
    </button>
  );
}
