import { Link } from 'react-router-dom';
import { PlugZap } from 'lucide-react';
import type { Instance } from '@/api/types';
import { formatPhone, timeAgo } from '@/lib/format';
import { cn } from '@/lib/cn';
import { Button } from '@/components/ui/button';
import { useInstanceActions } from './actions';
import { InstanceMenu } from './instance-menu';
import { InstanceAvatar, IntegrationChips, StatusBadge } from './instance-parts';

export function InstanceCard({ instance }: { instance: Instance }) {
  const actions = useInstanceActions();
  return (
    <article
      className={cn(
        'group relative flex flex-col rounded-card border border-line bg-surface shadow-card transition-all',
        'hover:border-line-strong hover:shadow-pop',
      )}
    >
      <div className="flex items-start gap-3 p-4">
        <InstanceAvatar instance={instance} size="md" />
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-[14px] leading-5 font-semibold">
            {/* Stretched link: the whole card opens the detail page, while buttons sit above it. */}
            <Link to={`/manager/instances/${instance.id}`} className="rounded outline-offset-4 after:absolute after:inset-0 after:rounded-card">
              {instance.name}
            </Link>
          </h3>
          <p className="truncate font-mono text-xs text-muted">{instance.number ? formatPhone(instance.number) : 'Sem número vinculado'}</p>
        </div>
        <StatusBadge connected={instance.connected} />
      </div>

      <div className="px-4 pb-3">
        <IntegrationChips instance={instance} />
        {!instance.connected && instance.disconnectReason ? (
          <p className="mt-2 truncate text-xs text-warn" title={instance.disconnectReason}>
            {instance.disconnectReason}
          </p>
        ) : null}
      </div>

      <div className="relative z-10 mt-auto flex items-center gap-2 border-t border-line px-4 py-2.5">
        <span className="min-w-0 flex-1 truncate text-xs text-subtle">Criada {timeAgo(instance.createdAt)}</span>
        {!instance.connected ? (
          <Button variant="soft" size="sm" onClick={() => actions.connect(instance)}>
            <PlugZap className="size-3.5" />
            Conectar
          </Button>
        ) : null}
        <InstanceMenu instance={instance} />
      </div>
    </article>
  );
}
