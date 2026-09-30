import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState, type ReactNode, type Ref } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '@/lib/cn';

interface TriggerProps {
  ref: Ref<HTMLButtonElement>;
  onClick: () => void;
  'aria-haspopup': 'menu';
  'aria-expanded': boolean;
}

const CloseCtx = createContext<() => void>(() => {});

interface MenuProps {
  trigger: (props: TriggerProps) => ReactNode;
  children: ReactNode;
  align?: 'start' | 'end';
  width?: number;
}

/** Popover menu rendered in a portal, so cards with overflow-hidden never clip it. Flips up near the viewport bottom. */
export function Menu({ trigger, children, align = 'end', width = 224 }: MenuProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top?: number; bottom?: number; left: number } | null>(null);
  const btn = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const close = useCallback(() => setOpen(false), []);

  useLayoutEffect(() => {
    if (!open || !btn.current) return;
    const r = btn.current.getBoundingClientRect();
    const left = Math.min(Math.max(8, align === 'end' ? r.right - width : r.left), window.innerWidth - width - 8);
    const spaceBelow = window.innerHeight - r.bottom;
    setPos(spaceBelow < 260 && r.top > spaceBelow ? { bottom: window.innerHeight - r.top + 4, left } : { top: r.bottom + 4, left });
  }, [open, align, width]);

  useEffect(() => {
    if (!open) return;
    panel.current?.focus();
    const onDown = (e: MouseEvent) => {
      if (!panel.current?.contains(e.target as Node) && !btn.current?.contains(e.target as Node)) close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        close();
        btn.current?.focus();
      }
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const items = [...(panel.current?.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])') ?? [])];
        if (!items.length) return;
        const i = items.indexOf(document.activeElement as HTMLElement);
        const next = e.key === 'ArrowDown' ? (i + 1) % items.length : (i - 1 + items.length) % items.length;
        items[next]?.focus();
      }
    };
    const onScroll = () => close();
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    window.addEventListener('resize', onScroll);
    window.addEventListener('scroll', onScroll, true);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
      window.removeEventListener('resize', onScroll);
      window.removeEventListener('scroll', onScroll, true);
    };
  }, [open, close]);

  return (
    <>
      {trigger({ ref: btn, onClick: () => setOpen((o) => !o), 'aria-haspopup': 'menu', 'aria-expanded': open })}
      {open
        ? createPortal(
            <div
              ref={panel}
              role="menu"
              tabIndex={-1}
              style={{ width, top: pos?.top, bottom: pos?.bottom, left: pos?.left ?? -9999 }}
              className="fixed z-60 animate-pop-in rounded-card border border-line bg-surface p-1 shadow-pop outline-none"
            >
              <CloseCtx.Provider value={close}>{children}</CloseCtx.Provider>
            </div>,
            document.body,
          )
        : null}
    </>
  );
}

interface MenuItemProps {
  icon?: ReactNode;
  children: ReactNode;
  onSelect: () => void;
  danger?: boolean;
  disabled?: boolean;
}

export function MenuItem({ icon, children, onSelect, danger, disabled }: MenuItemProps) {
  const close = useContext(CloseCtx);
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={() => {
        close();
        onSelect();
      }}
      className={cn(
        'flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-[13px] transition-colors [&_svg]:size-4 [&_svg]:shrink-0',
        'focus:outline-none disabled:opacity-50 pointer-coarse:py-2.5',
        danger ? 'text-danger hover:bg-danger-soft focus:bg-danger-soft' : 'text-fg hover:bg-surface-2 focus:bg-surface-2 [&_svg]:text-muted',
      )}
    >
      {icon}
      {children}
    </button>
  );
}

export function MenuSeparator() {
  return <div role="separator" className="my-1 h-px bg-line" />;
}

export function MenuLabel({ children }: { children: ReactNode }) {
  return <p className="px-2.5 pt-1.5 pb-1 text-[11px] font-medium tracking-wide text-subtle uppercase">{children}</p>;
}
