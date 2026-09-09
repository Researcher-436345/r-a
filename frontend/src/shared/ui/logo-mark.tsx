import logoUrl from '../assets/logo-ra.png';

interface LogoMarkProps {
  className?: string;
  alt?: string;
  compact?: boolean;
}

export function LogoMark({ className = '', alt = '', compact = false }: LogoMarkProps) {
  return (
    <span className={`brand ${className}`.trim()}>
      <img className="logo-mark" src={logoUrl} alt={alt} />
      {!compact && <span className="brand__name">Odyssey</span>}
    </span>
  );
}
