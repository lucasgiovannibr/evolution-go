import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import { cn } from '@/lib/cn';

export function PageContainer({ children, className, wide }: { children: ReactNode; className?: string; wide?: boolean }) {
  return <div className={cn('mx-auto w-full px-4 py-5 sm:px-6 sm:py-6', wide ? 'max-w-[1400px]' : 'max-w-6xl', className)}>{children}</div>;
}

export interface Crumb {
  label: string;
  to?: string;
}

interface PageHeaderProps {
  title: ReactNode;
  description?: ReactNode;
  breadcrumbs?: Crumb[];
  actions?: ReactNode;
  leading?: ReactNode;
  /** Slot under the title row, e.g. tabs. */
  footer?: ReactNode;
  className?: string;
}

export function PageHeader({ title, description, breadcrumbs, actions, leading, footer, className }: PageHeaderProps) {
  return (
    <header className={cn('mb-5', className)}>
      {breadcrumbs?.length ? (
        <nav aria-label="Trilha" className="mb-2 flex items-center gap-1 text-xs text-muted">
          {breadcrumbs.map((c, i) => (
            <span key={c.label} className="flex items-center gap-1">
              {i > 0 ? <ChevronRight className="size-3 text-subtle" aria-hidden /> : null}
              {c.to ? (
                <Link to={c.to} className="rounded transition-colors hover:text-fg">
                  {c.label}
                </Link>
              ) : (
                <span className="text-fg">{c.label}</span>
              )}
            </span>
          ))}
        </nav>
      ) : null}
      <div className="flex flex-wrap items-start gap-x-4 gap-y-3">
        {leading}
        <div className="min-w-0 flex-1 basis-56">
          <h1 className="truncate text-xl leading-7 font-semibold tracking-tight">{title}</h1>
          {description ? <div className="mt-0.5 text-[13px] text-muted">{description}</div> : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
      {footer ? <div className="mt-4 border-b border-line">{footer}</div> : null}
    </header>
  );
}
