interface LogoMarkProps {
  className?: string;
}

export function LogoMark({ className = '' }: LogoMarkProps) {
  return <span className={`logo-mark ${className}`.trim()} aria-hidden="true" />;
}
