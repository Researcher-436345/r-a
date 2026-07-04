import logoUrl from '../assets/logo-ra.png';

interface LogoMarkProps {
  className?: string;
  alt?: string;
}

export function LogoMark({ className = '', alt = '' }: LogoMarkProps) {
  return <img className={`logo-mark ${className}`.trim()} src={logoUrl} alt={alt} />;
}
