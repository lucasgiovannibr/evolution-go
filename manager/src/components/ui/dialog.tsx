import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { AlertTriangle, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Button } from './button';
import { Input } from './form';

const FOCUSABLE =
  'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

let scrollLocks = 0;
function lockScroll() {
  if (scrollLocks++ === 0) document.documentElement.style.overflow = 'hidden';
  return () => {
    if (--scrollLocks === 0) document.documentElement.style.overflow = '';
  };
}

const sizes = {
  sm: 'sm:max-w-sm',
  md: 'sm:max-w-md',
  lg: 'sm:max-w-lg',
  xl: 'sm:max-w-2xl',
} as const;

interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: ReactNode;
  icon?: ReactNode;
  size?: keyof typeof sizes;
  /** Rendered in a sticky footer. */
  footer?: ReactNode;
  children?: ReactNode;
  /** Set false to keep the dialog open on Esc/backdrop (e.g. while saving). */
  dismissible?: boolean;
}

/**
 * Modal built on a portal (not <dialog>) so toasts, which live in the body, stay above it.
 * Bottom sheet on phones, centered card from `sm` up. Traps focus and restores it on close.
 */
export function Dialog({ open, onClose, title, description, icon, size = 'md', footer, children, dismissible = true }: DialogProps) {
  const panel = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const descId = useId();
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const unlock = lockScroll();
    const node = panel.current;
    // Focus the requested field, else the dialog itself: landing on the close button reads as an error state.
    (node?.querySelector<HTMLElement>('[data-autofocus]') ?? node)?.focus({ preventScroll: true });

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && dismissible) {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== 'Tab' || !node) return;
      const items = [...node.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.offsetParent !== null);
      if (items.length === 0) {
        e.preventDefault();
        return;
      }
      const a = items[0] as HTMLElement;
      const z = items[items.length - 1] as HTMLElement;
      if (e.shiftKey && document.activeElement === a) {
        e.preventDefault();
        z.focus();
      } else if (!e.shiftKey && document.activeElement === z) {
        e.preventDefault();
        a.focus();
      }
    };
    document.addEventListener('keydown', onKey, true);
    return () => {
      document.removeEventListener('keydown', onKey, true);
      unlock();
      previous?.focus?.({ preventScroll: true });
    };
  }, [open, dismissible]);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:p-4">
      <div className="absolute inset-0 animate-fade-in bg-(--overlay) backdrop-blur-[2px]" onClick={dismissible ? onClose : undefined} aria-hidden />
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
        tabIndex={-1}
        className={cn(
          'relative flex max-h-[92dvh] w-full animate-pop-in flex-col overflow-hidden rounded-t-2xl border border-line bg-surface shadow-pop outline-none sm:rounded-2xl',
          sizes[size],
        )}
      >
        <header className="flex items-start gap-3 px-5 pt-5 pb-3">
          {icon ? (
            <span className="flex size-9 shrink-0 items-center justify-center rounded-control bg-brand-soft text-brand-text [&_svg]:size-[18px]">
              {icon}
            </span>
          ) : null}
          <div className="min-w-0 flex-1">
            <h2 id={titleId} className="text-[15px] leading-6 font-semibold">
              {title}
            </h2>
            {description ? (
              <p id={descId} className="mt-0.5 text-[13px] text-muted">
                {description}
              </p>
            ) : null}
          </div>
          {dismissible ? (
            <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Fechar" className="-mt-1 -mr-2">
              <X className="size-4" />
            </Button>
          ) : null}
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-2">{children}</div>
        {footer ? (
          <footer className="flex flex-col-reverse gap-2 border-t border-line bg-surface-2/60 px-5 py-3 sm:flex-row sm:justify-end [&>button]:max-sm:w-full">
            {footer}
          </footer>
        ) : (
          <div className="h-3" />
        )}
      </div>
    </div>,
    document.body,
  );
}

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: ReactNode;
  confirmLabel: string;
  loading?: boolean;
  /** When set, the user must type this exact text to enable the button. */
  requireText?: string;
  tone?: 'danger' | 'primary';
}

export function ConfirmDialog({ open, onClose, onConfirm, title, description, confirmLabel, loading, requireText, tone = 'danger' }: ConfirmDialogProps) {
  const [typed, setTyped] = useState('');
  useEffect(() => {
    if (!open) setTyped('');
  }, [open]);
  const blocked = requireText !== undefined && typed !== requireText;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      dismissible={!loading}
      size="sm"
      title={title}
      icon={tone === 'danger' ? <AlertTriangle className="text-danger" /> : undefined}
      description={description}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={loading}>
            Cancelar
          </Button>
          <Button variant={tone === 'danger' ? 'danger' : 'primary'} onClick={onConfirm} loading={loading} disabled={blocked}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      {requireText !== undefined ? (
        <form
          className="space-y-2 pb-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (!blocked && !loading) onConfirm();
          }}
        >
          <label className="text-xs text-muted">
            Digite <code className="rounded bg-surface-2 px-1 py-0.5 font-mono text-fg">{requireText}</code> para confirmar
          </label>
          <Input data-autofocus value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" spellCheck={false} />
        </form>
      ) : null}
    </Dialog>
  );
}
