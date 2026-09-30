import type { ReactNode } from 'react';
import { AlertCircle, AlertTriangle, CheckCircle2, Info, Loader2 } from 'lucide-react';
import { cn } from '@/lib/cn';
import type { Tone } from './badge';

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn('size-4 animate-spin text-muted', className)} aria-label="Carregando" />;
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn('animate-pulse rounded-md bg-surface-3/80', className)} aria-hidden />;
}

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn('flex flex-col items-center px-6 py-14 text-center', className)}>
      <span className="mb-4 flex size-12 items-center justify-center rounded-xl bg-surface-2 text-muted ring-1 ring-line [&_svg]:size-6">{icon}</span>
      <h3 className="text-[15px] font-semibold">{title}</h3>
      {description ? <p className="mt-1 max-w-sm text-[13px] text-muted">{description}</p> : null}
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}

const alertTones: Record<Exclude<Tone, 'neutral' | 'brand'>, { box: string; icon: ReactNode }> = {
  info: { box: 'border-info/30 bg-info-soft text-fg', icon: <Info className="text-info" /> },
  ok: { box: 'border-ok/30 bg-ok-soft text-fg', icon: <CheckCircle2 className="text-ok" /> },
  warn: { box: 'border-warn/30 bg-warn-soft text-fg', icon: <AlertTriangle className="text-warn" /> },
  danger: { box: 'border-danger/30 bg-danger-soft text-fg', icon: <AlertCircle className="text-danger" /> },
};

interface AlertProps {
  tone?: keyof typeof alertTones;
  title?: string;
  children?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function Alert({ tone = 'info', title, children, action, className }: AlertProps) {
  const t = alertTones[tone];
  return (
    <div role={tone === 'danger' ? 'alert' : 'status'} className={cn('flex gap-2.5 rounded-card border px-3 py-2.5 text-[13px]', t.box, className)}>
      <span className="mt-0.5 shrink-0 [&_svg]:size-4">{t.icon}</span>
      <div className="min-w-0 flex-1">
        {title ? <p className="font-medium">{title}</p> : null}
        {children ? <div className={cn('text-muted', title && 'mt-0.5')}>{children}</div> : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}
