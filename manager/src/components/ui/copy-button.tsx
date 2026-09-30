import { useEffect, useRef, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { toast } from 'sonner';
import { copyText } from '@/lib/clipboard';
import { cn } from '@/lib/cn';
import { Button, type ButtonProps } from './button';

interface CopyButtonProps extends Omit<ButtonProps, 'onClick' | 'children'> {
  value: string;
  label?: string;
  /** When set, renders text next to the icon. */
  text?: string;
}

export function CopyButton({ value, label = 'Copiar', text, size = 'icon-sm', variant = 'ghost', className, ...rest }: CopyButtonProps) {
  const [done, setDone] = useState(false);
  const timer = useRef<number>(0);
  useEffect(() => () => window.clearTimeout(timer.current), []);

  const copy = async () => {
    const ok = await copyText(value);
    if (!ok) {
      toast.error('Não foi possível copiar');
      return;
    }
    setDone(true);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setDone(false), 1600);
  };

  return (
    <Button
      variant={variant}
      size={text ? 'sm' : size}
      onClick={copy}
      aria-label={done ? 'Copiado' : label}
      title={done ? 'Copiado!' : label}
      className={cn(done && 'text-ok hover:text-ok', className)}
      {...rest}
    >
      {done ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      {text ? (done ? 'Copiado' : text) : null}
    </Button>
  );
}
