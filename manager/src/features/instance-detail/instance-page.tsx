import { Link, Outlet, useOutletContext, useParams } from 'react-router-dom';
import { Bell, FlaskConical, Info, Phone, PlugZap, SearchX, SlidersHorizontal, Unplug } from 'lucide-react';
import type { Instance } from '@/api/types';
import { useInstance } from '@/hooks/use-instances';
import { useDocumentTitle } from '@/hooks/use-document-title';
import { formatPhone } from '@/lib/format';
import { Button, buttonStyles } from '@/components/ui/button';
import { EmptyState, Skeleton } from '@/components/ui/feedback';
import { TabLinks } from '@/components/ui/tabs';
import { PageContainer, PageHeader } from '@/components/layout/page';
import { useInstanceActions } from '@/features/instances/actions';
import { InstanceMenu } from '@/features/instances/instance-menu';
import { InstanceAvatar, StatusBadge } from '@/features/instances/instance-parts';

export function useInstanceContext() {
  return useOutletContext<{ instance: Instance }>();
}

export function InstancePage() {
  const { instanceId } = useParams();
  const { data: instance, isPending, isError } = useInstance(instanceId);
  const actions = useInstanceActions();
  useDocumentTitle(instance?.name);

  if (isPending && !instance) {
    return (
      <PageContainer>
        <Skeleton className="mb-6 h-14 w-80" />
        <Skeleton className="mb-4 h-10 w-full" />
        <div className="grid gap-4 lg:grid-cols-3">
          <Skeleton className="h-64 rounded-card lg:col-span-2" />
          <Skeleton className="h-64 rounded-card" />
        </div>
      </PageContainer>
    );
  }

  if (isError || !instance) {
    return (
      <PageContainer>
        <div className="rounded-card border border-dashed border-line-strong bg-surface">
          <EmptyState
            icon={<SearchX />}
            title="Instância não encontrada"
            description="Ela pode ter sido excluída ou o endereço está incorreto."
            action={
              <Link to="/manager/instances" className={buttonStyles('primary')}>
                Voltar às instâncias
              </Link>
            }
          />
        </div>
      </PageContainer>
    );
  }

  const base = `/manager/instances/${instance.id}`;

  return (
    <PageContainer>
      <PageHeader
        breadcrumbs={[{ label: 'Instâncias', to: '/manager/instances' }, { label: instance.name }]}
        leading={<InstanceAvatar instance={instance} size="lg" />}
        title={
          <span className="flex items-center gap-2.5">
            <span className="truncate">{instance.name}</span>
            <StatusBadge connected={instance.connected} pulse />
          </span>
        }
        description={<span className="font-mono text-xs">{instance.number ? formatPhone(instance.number) : 'Nenhum número vinculado'}</span>}
        actions={
          <>
            {instance.connected ? (
              <Button onClick={() => actions.disconnect(instance)}>
                <Unplug className="size-4" />
                Desconectar
              </Button>
            ) : (
              <Button variant="primary" onClick={() => actions.connect(instance)}>
                <PlugZap className="size-4" />
                Conectar
              </Button>
            )}
            <InstanceMenu instance={instance} variant="secondary" />
          </>
        }
        footer={
          <TabLinks
            label="Seções da instância"
            items={[
              { to: base, label: 'Geral', icon: <Info />, end: true },
              { to: `${base}/webhook`, label: 'Webhook e eventos', icon: <Bell /> },
              { to: `${base}/behavior`, label: 'Comportamento', icon: <SlidersHorizontal /> },
              { to: `${base}/calls`, label: 'Chamadas', icon: <Phone /> },
              { to: `${base}/test`, label: 'Testar envio', icon: <FlaskConical /> },
            ]}
          />
        }
      />
      <Outlet context={{ instance }} />
    </PageContainer>
  );
}
