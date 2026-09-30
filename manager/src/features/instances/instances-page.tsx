import { useMemo, useState } from 'react';
import { LayoutGrid, List, Plus, RefreshCw, Search, Smartphone, SearchX, X } from 'lucide-react';
import type { Instance } from '@/api/types';
import { useInstances } from '@/hooks/use-instances';
import { useDocumentTitle } from '@/hooks/use-document-title';
import { cn } from '@/lib/cn';
import { Alert, EmptyState, Skeleton } from '@/components/ui/feedback';
import { Button } from '@/components/ui/button';
import { Input, Select } from '@/components/ui/form';
import { Segmented } from '@/components/ui/segmented';
import { PageContainer, PageHeader } from '@/components/layout/page';
import { useInstanceActions } from './actions';
import { InstanceCard } from './instance-card';
import { InstanceTable } from './instance-table';

type Filter = 'all' | 'online' | 'offline';
type Sort = 'recent' | 'name' | 'status';
type View = 'grid' | 'list';

const VIEW_KEY = 'evolution-instances-view';
const readView = (): View => {
  try {
    return localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'grid';
  } catch {
    return 'grid';
  }
};

const sorters: Record<Sort, (a: Instance, b: Instance) => number> = {
  recent: (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  name: (a, b) => a.name.localeCompare(b.name, 'pt-BR', { numeric: true }),
  status: (a, b) => Number(a.connected) - Number(b.connected) || a.name.localeCompare(b.name, 'pt-BR', { numeric: true }),
};

export function InstancesPage() {
  useDocumentTitle('Instâncias');
  const actions = useInstanceActions();
  const { data, isPending, isError, error, refetch, isFetching } = useInstances();
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<Filter>('all');
  const [sort, setSort] = useState<Sort>('recent');
  const [view, setView] = useState<View>(readView);

  const all = data ?? [];
  const counts = useMemo(() => {
    const online = all.filter((i) => i.connected).length;
    return { all: all.length, online, offline: all.length - online };
  }, [all]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return all
      .filter((i) => (filter === 'all' ? true : filter === 'online' ? i.connected : !i.connected))
      .filter((i) => !q || i.name.toLowerCase().includes(q) || i.number.includes(q.replace(/\D/g, '') || '\u0000') || i.webhook.toLowerCase().includes(q))
      .sort(sorters[sort]);
  }, [all, query, filter, sort]);

  const changeView = (v: View) => {
    setView(v);
    try {
      localStorage.setItem(VIEW_KEY, v);
    } catch {
      /* ignore */
    }
  };

  const filtering = query.trim() !== '' || filter !== 'all';

  return (
    <PageContainer wide>
      <PageHeader
        title="Instâncias"
        description="Conecte, configure e acompanhe seus números de WhatsApp."
        actions={
          <Button variant="primary" onClick={actions.create}>
            <Plus className="size-4" />
            Nova instância
          </Button>
        }
      />

      {counts.all > 0 || isPending ? (
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <Input
            className="w-full sm:w-64"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Buscar por nome, número ou webhook"
            aria-label="Buscar instâncias"
            leading={<Search />}
            trailing={
              query ? (
                <Button variant="ghost" size="icon-sm" onClick={() => setQuery('')} aria-label="Limpar busca">
                  <X className="size-3.5" />
                </Button>
              ) : undefined
            }
          />
          <Segmented
            label="Filtrar por status"
            value={filter}
            onChange={setFilter}
            options={[
              { value: 'all', label: 'Todas', count: counts.all },
              { value: 'online', label: 'Conectadas', count: counts.online },
              { value: 'offline', label: 'Desconectadas', count: counts.offline },
            ]}
          />
          <div className="ml-auto flex items-center gap-2">
            <Select value={sort} onChange={(e) => setSort(e.target.value as Sort)} aria-label="Ordenar por" className="w-36">
              <option value="recent">Mais recentes</option>
              <option value="name">Nome (A–Z)</option>
              <option value="status">Desconectadas 1º</option>
            </Select>
            <Segmented
              label="Modo de exibição"
              value={view}
              onChange={changeView}
              options={[
                { value: 'grid', label: <LayoutGrid />, title: 'Cartões' },
                { value: 'list', label: <List />, title: 'Lista' },
              ]}
            />
            <Button variant="secondary" size="icon" onClick={() => void refetch()} aria-label="Atualizar lista" title="Atualizar">
              <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
            </Button>
          </div>
        </div>
      ) : null}

      {isError ? (
        <Alert
          tone="danger"
          title="Não foi possível carregar as instâncias"
          action={
            <Button size="sm" onClick={() => void refetch()}>
              Tentar de novo
            </Button>
          }
        >
          {error instanceof Error ? error.message : 'Erro desconhecido.'}
        </Alert>
      ) : isPending ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-36 rounded-card" />
          ))}
        </div>
      ) : counts.all === 0 ? (
        <div className="rounded-card border border-dashed border-line-strong bg-surface">
          <EmptyState
            icon={<Smartphone />}
            title="Nenhuma instância ainda"
            description="Crie a primeira instância e conecte um número de WhatsApp lendo um QR Code."
            action={
              <Button variant="primary" onClick={actions.create}>
                <Plus className="size-4" />
                Criar instância
              </Button>
            }
          />
        </div>
      ) : visible.length === 0 ? (
        <div className="rounded-card border border-dashed border-line-strong bg-surface">
          <EmptyState
            icon={<SearchX />}
            title="Nada encontrado"
            description="Nenhuma instância corresponde à busca ou ao filtro atual."
            action={
              filtering ? (
                <Button
                  onClick={() => {
                    setQuery('');
                    setFilter('all');
                  }}
                >
                  Limpar filtros
                </Button>
              ) : null
            }
          />
        </div>
      ) : view === 'grid' ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {visible.map((i) => (
            <InstanceCard key={i.id} instance={i} />
          ))}
        </div>
      ) : (
        <InstanceTable instances={visible} />
      )}
    </PageContainer>
  );
}
