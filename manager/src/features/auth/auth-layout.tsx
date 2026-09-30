import type { ReactNode } from 'react';
import { Activity, KeyRound, Layers, Moon, Sun } from 'lucide-react';
import { Logo } from '@/components/ui/logo';
import { Button } from '@/components/ui/button';
import { useUi } from '@/stores/ui';

const FEATURES = [
  { icon: <Layers />, title: 'Várias instâncias, um só painel', text: 'Crie, conecte e monitore todos os seus números WhatsApp em um lugar.' },
  { icon: <Activity />, title: 'Tempo real', text: 'Status de conexão, QR Code e pareamento acompanhados ao vivo.' },
  { icon: <KeyRound />, title: 'Acesso controlado', text: 'Autenticação por API Key, sem depender de serviços externos para operar.' },
];

/** Split layout: brand panel on large screens, a single centered card below that. */
export function AuthLayout({ children }: { children: ReactNode }) {
  const { theme, toggleTheme } = useUi();
  return (
    <div className="grid min-h-dvh lg:grid-cols-[minmax(0,1fr)_minmax(0,1.05fr)]">
      <aside className="relative hidden overflow-hidden border-r border-line bg-surface-2 lg:flex lg:flex-col lg:justify-between lg:p-10">
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.5]"
          style={{
            backgroundImage: 'radial-gradient(circle at 1px 1px, var(--line-strong) 1px, transparent 0)',
            backgroundSize: '22px 22px',
            maskImage: 'radial-gradient(ellipse 80% 70% at 30% 20%, black, transparent)',
          }}
          aria-hidden
        />
        <div
          className="pointer-events-none absolute -top-32 -left-24 size-[28rem] rounded-full opacity-40 blur-3xl"
          style={{ background: 'radial-gradient(closest-side, var(--brand-soft), transparent)' }}
          aria-hidden
        />
        <Logo className="relative" />
        <div className="relative max-w-md space-y-8">
          <div>
            <h2 className="text-2xl leading-tight font-semibold tracking-tight text-balance">Gerencie suas instâncias WhatsApp com clareza.</h2>
            <p className="mt-2 text-[13px] text-muted">Interface enxuta para conectar, configurar e testar a Evolution GO.</p>
          </div>
          <ul className="space-y-5">
            {FEATURES.map((f) => (
              <li key={f.title} className="flex gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-control bg-surface text-brand-text shadow-card ring-1 ring-line [&_svg]:size-[18px]">{f.icon}</span>
                <div>
                  <p className="text-[13px] font-medium">{f.title}</p>
                  <p className="text-xs text-muted">{f.text}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-xs text-subtle">Evolution GO Manager</p>
      </aside>

      <main className="relative flex flex-col items-center justify-center px-4 py-10 sm:px-8">
        <Button variant="ghost" size="icon" onClick={toggleTheme} className="absolute top-4 right-4" aria-label={theme === 'dark' ? 'Usar tema claro' : 'Usar tema escuro'}>
          {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
        </Button>
        <div className="w-full max-w-sm">
          <Logo className="mb-8 lg:hidden" />
          {children}
        </div>
      </main>
    </div>
  );
}
