import { cn } from '@/lib/cn';

interface SwitchProps {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
  label: string;
  id?: string;
}

export function Switch({ checked, onChange, disabled, label, id }: SwitchProps) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors disabled:opacity-50 pointer-coarse:h-6 pointer-coarse:w-11',
        checked ? 'bg-brand' : 'bg-line-strong',
      )}
    >
      <span
        className={cn(
          'block size-4 rounded-full bg-white shadow transition-transform pointer-coarse:size-5',
          checked ? 'translate-x-[18px] pointer-coarse:translate-x-[22px]' : 'translate-x-0.5',
        )}
      />
    </button>
  );
}

interface SwitchRowProps extends Omit<SwitchProps, 'label' | 'id'> {
  title: string;
  description: string;
  icon?: React.ReactNode;
}

/** A settings row: icon, title + description on the left, switch on the right. Whole row toggles. */
export function SwitchRow({ title, description, icon, checked, onChange, disabled }: SwitchRowProps) {
  return (
    <div className="flex items-center gap-3 py-3">
      {icon ? (
        <span className="flex size-8 shrink-0 items-center justify-center rounded-control bg-surface-2 text-muted [&_svg]:size-4">
          {icon}
        </span>
      ) : null}
      <div className="min-w-0 flex-1">
        <p className="text-[13px] font-medium">{title}</p>
        <p className="text-xs text-muted">{description}</p>
      </div>
      <Switch checked={checked} onChange={onChange} disabled={disabled} label={title} />
    </div>
  );
}
