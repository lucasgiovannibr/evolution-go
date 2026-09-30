import { cn } from '@/lib/cn';

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn('size-8 shrink-0', className)} aria-hidden>
      <rect width="32" height="32" rx="8" fill="var(--brand)" />
      <path d="M11.5 9v14M11.5 9.5h10M11.5 16h7M11.5 22.5h10" stroke="var(--brand-fg)" strokeWidth="2.6" strokeLinecap="round" fill="none" />
    </svg>
  );
}

export function Logo({ compact, className }: { compact?: boolean; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <LogoMark className="size-7" />
      {compact ? null : (
        <span className="text-[15px] leading-none font-semibold tracking-tight">
          Evolution <span className="text-brand-text">GO</span>
        </span>
      )}
    </span>
  );
}
