import type { CSSProperties } from 'react';
import { Cable, Globe, Network, Radio, ShieldCheck, Webhook } from 'lucide-react';
import type { Instance } from '@/api/types';
import { hueFromString, initials } from '@/lib/format';
import { cn } from '@/lib/cn';
import { Badge } from '@/components/ui/badge';

export function InstanceAvatar({ instance, size = 'md', showStatus = true }: { instance: Pick<Instance, 'name' | 'connected'>; size?: 'sm' | 'md' | 'lg'; showStatus?: boolean }) {
  const dims = { sm: 'size-8 text-[11px]', md: 'size-10 text-[13px]', lg: 'size-12 text-base' }[size];
  return (
    <span className="relative inline-flex shrink-0">
      <span
        className={cn('avatar-tint inline-flex items-center justify-center rounded-xl font-semibold tracking-wide select-none', dims)}
        style={{ '--h': hueFromString(instance.name) } as CSSProperties}
        aria-hidden
      >
        {initials(instance.name)}
      </span>
      {showStatus ? (
        <span
          className={cn(
            'absolute -right-0.5 -bottom-0.5 size-3 rounded-full ring-2 ring-surface',
            instance.connected ? 'bg-ok' : 'bg-subtle',
          )}
          aria-hidden
        />
      ) : null}
    </span>
  );
}

export function StatusBadge({ connected, pulse, className }: { connected: boolean; pulse?: boolean; className?: string }) {
  return connected ? (
    <Badge tone="ok" dot pulse={pulse} className={className}>
      Conectada
    </Badge>
  ) : (
    <Badge tone="neutral" dot className={className}>
      Desconectada
    </Badge>
  );
}

interface Integration {
  key: string;
  label: string;
  icon: React.ReactNode;
}

/** Which delivery channels an instance pushes events to. */
export function integrationsOf(i: Instance): Integration[] {
  const out: Integration[] = [];
  if (i.webhook) out.push({ key: 'webhook', label: 'Webhook', icon: <Webhook /> });
  if (i.producers.rabbitmq === 'enabled') out.push({ key: 'rabbitmq', label: 'RabbitMQ', icon: <Network /> });
  if (i.producers.websocket === 'enabled') out.push({ key: 'websocket', label: 'WebSocket', icon: <Radio /> });
  if (i.producers.nats === 'enabled') out.push({ key: 'nats', label: 'NATS', icon: <Cable /> });
  return out;
}

export function IntegrationChips({ instance, className }: { instance: Instance; className?: string }) {
  const items = integrationsOf(instance);
  return (
    <div className={cn('flex flex-wrap items-center gap-1.5', className)}>
      {items.length === 0 ? <span className="text-xs text-subtle">Sem integrações</span> : null}
      {items.map((it) => (
        <span
          key={it.key}
          title={it.label}
          className="inline-flex h-5 items-center gap-1 rounded-md bg-surface-2 px-1.5 text-[11px] font-medium text-muted ring-1 ring-line ring-inset [&_svg]:size-3"
        >
          {it.icon}
          {it.label}
        </span>
      ))}
      {instance.proxy ? (
        <span
          title={`Proxy ${instance.proxy.host}:${instance.proxy.port}`}
          className="inline-flex h-5 items-center gap-1 rounded-md bg-surface-2 px-1.5 text-[11px] font-medium text-muted ring-1 ring-line ring-inset [&_svg]:size-3"
        >
          <Globe />
          Proxy
        </span>
      ) : null}
    </div>
  );
}

export function SecureNote({ children }: { children: React.ReactNode }) {
  return (
    <p className="flex items-center gap-1.5 text-xs text-muted [&_svg]:size-3.5 [&_svg]:text-subtle">
      <ShieldCheck />
      {children}
    </p>
  );
}
