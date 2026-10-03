import { cn } from '@/lib/cn';

const UPSTREAM = 'https://github.com/evolution-foundation/evolution-go';

/**
 * Credit and copyright line required by the upstream LICENSE (conditions 1a and 1b): this panel
 * must keep the copyright of the original project and tell the administrator that Evolution Go
 * is in use. Do not remove it.
 */
export function LegalNotice({ compact, className }: { compact?: boolean; className?: string }) {
  if (compact) {
    return (
      <p className={cn('text-center text-[11px] text-subtle', className)} title="Baseado no Evolution Go · © 2026 Evolution Foundation · Apache 2.0">
        <span aria-hidden>©</span>
        <span className="sr-only">Baseado no Evolution Go, © 2026 Evolution Foundation, licença Apache 2.0</span>
      </p>
    );
  }
  return (
    <p className={cn('text-[11px] leading-snug text-subtle', className)}>
      Baseado no{' '}
      <a href={UPSTREAM} target="_blank" rel="noreferrer noopener" className="underline underline-offset-2 hover:text-fg">
        Evolution Go
      </a>
      <br />© 2026 Evolution Foundation · Apache 2.0
    </p>
  );
}
