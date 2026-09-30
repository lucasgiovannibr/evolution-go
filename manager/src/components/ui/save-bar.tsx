import { Save, Undo2 } from 'lucide-react';
import { Button } from './button';

interface SaveBarProps {
  dirty: boolean;
  saving: boolean;
  onSave: () => void;
  onReset: () => void;
  disabled?: boolean;
}

/** Sticky "unsaved changes" bar. Only exists while there is something to save. */
export function SaveBar({ dirty, saving, onSave, onReset, disabled }: SaveBarProps) {
  if (!dirty) return null;
  return (
    <div className="sticky bottom-4 z-20 mt-5 flex animate-pop-in items-center gap-3 rounded-card border border-line-strong bg-surface/95 px-4 py-2.5 shadow-pop backdrop-blur">
      <span className="size-2 shrink-0 rounded-full bg-warn" aria-hidden />
      <p className="min-w-0 flex-1 truncate text-[13px] font-medium">Alterações não salvas</p>
      <Button variant="ghost" onClick={onReset} disabled={saving}>
        <Undo2 className="size-3.5" />
        <span className="max-sm:hidden">Descartar</span>
      </Button>
      <Button variant="primary" onClick={onSave} loading={saving} disabled={disabled}>
        {!saving ? <Save className="size-3.5" /> : null}
        Salvar
      </Button>
    </div>
  );
}
