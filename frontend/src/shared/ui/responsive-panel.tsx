import { useEffect, useRef, type ReactNode } from 'react';

import { useMobileLayout } from '../lib/use-mobile-layout';

interface ResponsivePanelProps {
  className: string;
  label: string;
  onClose: () => void;
  children: ReactNode;
}

export function ResponsivePanel({ className, label, onClose, children }: ResponsivePanelProps) {
  const isMobile = useMobileLayout();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!isMobile || !dialog) return;
    if (!dialog.open) {
      previousFocusRef.current = document.activeElement as HTMLElement | null;
      dialog.showModal();
    }
    return () => {
      if (previousFocusRef.current?.isConnected) previousFocusRef.current.focus({ preventScroll: true });
    };
  }, [isMobile]);

  if (!isMobile) return <aside className={className} aria-label={label}>{children}</aside>;

  return (
    <dialog
      ref={dialogRef}
      className={`mobile-dialog ${className}`}
      aria-label={label}
      aria-modal="true"
      onClose={onClose}
      onCancel={(event) => { event.preventDefault(); onClose(); }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') { event.preventDefault(); onClose(); }
      }}
      onClick={(event) => {
        if (event.target !== event.currentTarget) return;
        const box = event.currentTarget.getBoundingClientRect();
        if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) {
          onClose();
        }
      }}
    >
      {children}
    </dialog>
  );
}
