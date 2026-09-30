import { useMemo, type ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { CopyButton } from './copy-button';

const TOKEN = /("(?:\\.|[^"\\])*")(\s*:)?|\b(true|false|null)\b|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;

/** Tiny JSON highlighter: enough to scan a payload without pulling in a syntax library. */
export function highlightJson(text: string): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let n = 0;
  for (const m of text.matchAll(TOKEN)) {
    const idx = m.index ?? 0;
    if (idx > last) out.push(text.slice(last, idx));
    const [full, str, colon, lit, num] = m;
    let cls = '';
    if (str) cls = colon ? 'text-info' : 'text-brand-text';
    else if (lit) cls = 'text-warn';
    else if (num) cls = 'text-info';
    if (str && colon) {
      out.push(
        <span key={n++} className={cls}>
          {str}
        </span>,
        colon,
      );
    } else {
      out.push(
        <span key={n++} className={cls}>
          {full}
        </span>,
      );
    }
    last = idx + full.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

interface CodeBlockProps {
  code: string;
  language?: 'json' | 'text' | 'bash';
  className?: string;
  maxHeight?: string;
  copy?: boolean;
  /** Text placed on the clipboard when it differs from what is displayed (e.g. a masked secret). */
  copyValue?: string;
}

export function CodeBlock({ code, language = 'text', className, maxHeight = '20rem', copy = true, copyValue }: CodeBlockProps) {
  const content = useMemo(() => (language === 'json' ? highlightJson(code) : code), [code, language]);
  return (
    <div className={cn('group relative rounded-control border border-line bg-surface-2/60', className)}>
      <pre
        className="overflow-auto p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all text-fg"
        style={{ maxHeight }}
      >
        <code>{content}</code>
      </pre>
      {copy ? (
        <CopyButton value={copyValue ?? code} className="absolute top-1.5 right-1.5 bg-surface/80 opacity-0 backdrop-blur transition-opacity group-hover:opacity-100 focus-visible:opacity-100 pointer-coarse:opacity-100" />
      ) : null}
    </div>
  );
}
