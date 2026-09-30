import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';

export type Tone = 'neutral' | 'ok' | 'warn' | 'danger' | 'info' | 'brand';

const tones: Record<Tone, string> = {
  neutral: 'bg-surface-2 text-muted ring-line',
  ok: 'bg-ok-soft text-ok ring-ok/25',
  brand: 'bg-brand-soft text-brand-text ring-brand/25',
  warn: 'bg-warn-soft text-warn ring-warn/25',
  danger: 'bg-danger-soft text-danger ring-danger/25',
  info: 'bg-info-soft text-info ring-info/25',
};

const dots: Record<Tone, string> = {
  neutral: 'bg-subtle',
  ok: 'bg-ok',
  brand: 'bg-brand',
  warn: 'bg-warn',
  danger: 'bg-danger',
  info: 'bg-info',
};

interface BadgeProps {
  tone?: Tone;
  dot?: boolean;
  pulse?: boolean;
  icon?: ReactNode;
  className?: string;
  children: ReactNode;
}

export function Badge({ tone = 'neutral', dot, pulse, icon, className, children }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex h-5 items-center gap-1.5 rounded-full px-2 text-[11px] leading-none font-medium whitespace-nowrap ring-1 ring-inset [&_svg]:size-3',
        tones[tone],
        className,
      )}
    >
      {dot ? <span className={cn('size-1.5 rounded-full', dots[tone], pulse && 'animate-pulse-ring')} aria-hidden /> : null}
      {icon}
      {children}
    </span>
  );
}
