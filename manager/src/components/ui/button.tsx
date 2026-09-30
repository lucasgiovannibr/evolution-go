import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { Loader2 } from 'lucide-react';
import { cn } from '@/lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'soft' | 'danger' | 'danger-soft';
export type ButtonSize = 'sm' | 'md' | 'lg' | 'icon-sm' | 'icon' | 'icon-lg';

const variants: Record<ButtonVariant, string> = {
  primary: 'bg-brand text-brand-fg hover:bg-brand-hover shadow-sm',
  secondary: 'border border-line-strong bg-surface text-fg shadow-sm hover:bg-surface-2',
  ghost: 'text-muted hover:bg-surface-2 hover:text-fg',
  soft: 'bg-brand-soft text-brand-text hover:bg-brand/20',
  danger: 'bg-danger text-white hover:bg-danger-hover shadow-sm',
  'danger-soft': 'bg-danger-soft text-danger hover:bg-danger/20',
};

const sizes: Record<ButtonSize, string> = {
  sm: 'h-7 gap-1.5 px-2.5 text-xs pointer-coarse:h-9',
  md: 'h-8 gap-1.5 px-3 text-[13px] pointer-coarse:h-10',
  lg: 'h-10 gap-2 px-4 text-sm',
  'icon-sm': 'size-7 pointer-coarse:size-9',
  icon: 'size-8 pointer-coarse:size-10',
  'icon-lg': 'size-10',
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
}

export function buttonStyles(variant: ButtonVariant = 'secondary', size: ButtonSize = 'md', className?: string) {
  return cn(
    'inline-flex shrink-0 items-center justify-center rounded-control font-medium whitespace-nowrap transition-colors',
    'select-none disabled:pointer-events-none disabled:opacity-50',
    variants[variant],
    sizes[size],
    className,
  );
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'secondary', size = 'md', loading, disabled, className, children, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      className={buttonStyles(variant, size, className)}
      {...rest}
    >
      {loading ? <Loader2 className="size-3.5 animate-spin" aria-hidden /> : null}
      {children}
    </button>
  );
});
