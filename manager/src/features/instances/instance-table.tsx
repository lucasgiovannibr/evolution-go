import { Link } from 'react-router-dom';
import { PlugZap } from 'lucide-react';
import type { Instance } from '@/api/types';
import { formatPhone, timeAgo } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { useInstanceActions } from './actions';
import { InstanceMenu } from './instance-menu';
import { InstanceAvatar, IntegrationChips, StatusBadge } from './instance-parts';

const th = 'px-4 py-2.5 text-left text-[11px] font-medium tracking-wide text-subtle uppercase';

export function InstanceTable({ instances }: { instances: Instance[] }) {
  const actions = useInstanceActions();
  return (
    <div className="overflow-hidden rounded-card border border-line bg-surface shadow-card">
      <table className="w-full border-collapse">
        <thead className="border-b border-line bg-surface-2/60">
          <tr>
            <th className={th}>Instância</th>
            <th className={`${th} hidden sm:table-cell`}>Status</th>
            <th className={`${th} hidden md:table-cell`}>Número</th>
            <th className={`${th} hidden lg:table-cell`}>Integrações</th>
            <th className={`${th} hidden xl:table-cell`}>Criada</th>
            <th className={`${th} w-px`}>
              <span className="sr-only">Ações</span>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {instances.map((i) => (
            <tr key={i.id} className="group relative transition-colors hover:bg-surface-2/60">
              <td className="px-4 py-2.5">
                <div className="flex items-center gap-3">
                  <InstanceAvatar instance={i} size="sm" showStatus={false} />
                  <div className="min-w-0">
                    <Link
                      to={`/manager/instances/${i.id}`}
                      className="block truncate text-[13px] font-semibold after:absolute after:inset-0"
                    >
                      {i.name}
                    </Link>
                    <p className="truncate font-mono text-xs text-muted md:hidden">{i.number ? formatPhone(i.number) : 'Sem número'}</p>
                    <StatusBadge connected={i.connected} className="mt-1 sm:hidden" />
                  </div>
                </div>
              </td>
              <td className="hidden px-4 py-2.5 sm:table-cell">
                <StatusBadge connected={i.connected} />
              </td>
              <td className="hidden px-4 py-2.5 font-mono text-xs text-muted md:table-cell">{i.number ? formatPhone(i.number) : '—'}</td>
              <td className="hidden px-4 py-2.5 lg:table-cell">
                <IntegrationChips instance={i} />
              </td>
              <td className="hidden px-4 py-2.5 text-xs whitespace-nowrap text-muted xl:table-cell">{timeAgo(i.createdAt)}</td>
              <td className="px-3 py-2.5">
                <div className="relative z-10 flex items-center justify-end gap-1">
                  {!i.connected ? (
                    <Button variant="soft" size="sm" onClick={() => actions.connect(i)} aria-label={`Conectar ${i.name}`}>
                      <PlugZap className="size-3.5" />
                      <span className="max-sm:hidden">Conectar</span>
                    </Button>
                  ) : null}
                  <InstanceMenu instance={i} />
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
