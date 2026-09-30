import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, ChevronRight, FlaskConical, Play, RefreshCw, Search, SearchX, Terminal, Wand2 } from 'lucide-react';
import { rawRequest, type RawResponse } from '@/lib/http';
import { buildCurl } from '@/lib/curl';
import { formatBytes } from '@/lib/format';
import { cn } from '@/lib/cn';
import { useInstances } from '@/hooks/use-instances';
import { useDocumentTitle } from '@/hooks/use-document-title';
import { useAuth } from '@/stores/auth';
import { Alert, EmptyState, Skeleton } from '@/components/ui/feedback';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { CodeBlock } from '@/components/ui/code-block';
import { CopyButton } from '@/components/ui/copy-button';
import { Field, Input, Select, Textarea } from '@/components/ui/form';
import { Segmented } from '@/components/ui/segmented';
import { exampleFromRef, groupByTag, parseSpec, type Endpoint, type Method, type SwaggerSpec } from './swagger';

const METHOD_STYLE: Record<Method, string> = {
  GET: 'bg-ok-soft text-ok',
  POST: 'bg-info-soft text-info',
  PUT: 'bg-warn-soft text-warn',
  PATCH: 'bg-brand-soft text-brand-text',
  DELETE: 'bg-danger-soft text-danger',
};

function MethodTag({ method, className }: { method: Method; className?: string }) {
  return <span className={cn('inline-flex h-5 w-12 shrink-0 items-center justify-center rounded font-mono text-[10px] font-bold', METHOD_STYLE[method], className)}>{method}</span>;
}

type AuthMode = 'global' | 'instance' | 'custom';

function useSpec() {
  const apiUrl = useAuth((s) => s.apiUrl);
  return useQuery({
    queryKey: ['swagger', apiUrl],
    queryFn: async () => {
      const res = await rawRequest('/swagger/doc.json', { apikey: null });
      if (res.status !== 200 || typeof res.data !== 'object' || !res.data) throw new Error('Não foi possível carregar /swagger/doc.json.');
      return res.data as SwaggerSpec;
    },
    staleTime: 5 * 60_000,
  });
}

export function ExplorerPage() {
  useDocumentTitle('Explorador da API');
  const { data: spec, isPending, isError, refetch, isFetching } = useSpec();
  const [selected, setSelected] = useState<string | null>(null);
  const [query, setQuery] = useState('');

  const endpoints = useMemo(() => (spec ? parseSpec(spec) : []), [spec]);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return endpoints;
    return endpoints.filter((e) => e.path.toLowerCase().includes(q) || e.method.toLowerCase().includes(q) || e.summary.toLowerCase().includes(q) || e.tag.toLowerCase().includes(q));
  }, [endpoints, query]);
  const groups = useMemo(() => groupByTag(filtered), [filtered]);
  const endpoint = endpoints.find((e) => e.key === selected) ?? null;

  return (
    <div className="flex h-[calc(100dvh-3rem)] lg:h-dvh">
      <aside className={cn('flex w-full shrink-0 flex-col border-r border-line bg-surface lg:w-80', endpoint && 'max-lg:hidden')}>
        <div className="space-y-3 border-b border-line p-3">
          <div className="flex items-center gap-2">
            <span className="flex size-7 items-center justify-center rounded-md bg-surface-2 text-muted">
              <FlaskConical className="size-4" />
            </span>
            <h1 className="flex-1 text-[15px] font-semibold tracking-tight">Explorador da API</h1>
            <Button variant="ghost" size="icon-sm" onClick={() => void refetch()} aria-label="Recarregar especificação" title="Recarregar">
              <RefreshCw className={cn('size-3.5', isFetching && 'animate-spin')} />
            </Button>
          </div>
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Filtrar por caminho, método ou tag" aria-label="Filtrar endpoints" leading={<Search />} />
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {isPending ? (
            <div className="space-y-2 p-1">
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className="h-8" />
              ))}
            </div>
          ) : isError ? (
            <Alert tone="danger" title="Falha ao carregar" className="m-1">
              Não foi possível ler /swagger/doc.json. Verifique se o servidor está online.
            </Alert>
          ) : groups.length === 0 ? (
            <EmptyState icon={<SearchX />} title="Nenhum endpoint" description="Nada corresponde ao filtro." className="py-10" />
          ) : (
            groups.map(([tag, items]) => <TagGroup key={tag} tag={tag} items={items} selected={selected} onSelect={setSelected} forceOpen={query.trim() !== ''} />)
          )}
        </div>
      </aside>

      <section className={cn('min-w-0 flex-1 overflow-y-auto', !endpoint && 'max-lg:hidden')}>
        {endpoint && spec ? (
          <RequestPanel key={endpoint.key} endpoint={endpoint} spec={spec} onBack={() => setSelected(null)} />
        ) : (
          <div className="flex h-full items-center justify-center">
            <EmptyState icon={<Terminal />} title="Selecione um endpoint" description="Escolha uma rota na lista para montar a requisição, enviar e ver a resposta." />
          </div>
        )}
      </section>
    </div>
  );
}

function TagGroup({ tag, items, selected, onSelect, forceOpen }: { tag: string; items: Endpoint[]; selected: string | null; onSelect: (k: string) => void; forceOpen: boolean }) {
  const hasSelected = items.some((e) => e.key === selected);
  const [open, setOpen] = useState(hasSelected);
  const expanded = open || forceOpen || hasSelected;
  return (
    <div className="mb-1">
      <button
        type="button"
        onClick={() => setOpen(!expanded)}
        aria-expanded={expanded}
        className="flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-left text-[11px] font-semibold tracking-wide text-muted uppercase hover:bg-surface-2"
      >
        <ChevronRight className={cn('size-3 transition-transform', expanded && 'rotate-90')} />
        <span className="flex-1 truncate">{tag}</span>
        <span className="font-normal text-subtle">{items.length}</span>
      </button>
      {expanded ? (
        <ul className="mt-0.5 space-y-px">
          {items.map((e) => (
            <li key={e.key}>
              <button
                type="button"
                onClick={() => onSelect(e.key)}
                aria-current={e.key === selected}
                className={cn('flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors', e.key === selected ? 'bg-brand-soft' : 'hover:bg-surface-2')}
              >
                <MethodTag method={e.method} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-mono text-xs">{e.path}</span>
                  {e.summary ? <span className="block truncate text-[11px] text-muted">{e.summary}</span> : null}
                </span>
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

interface Sent {
  method: Method;
  res: RawResponse;
  url: string;
  headers: Record<string, string>;
  body?: string;
}

function RequestPanel({ endpoint, spec, onBack }: { endpoint: Endpoint; spec: SwaggerSpec; onBack: () => void }) {
  const apiUrl = useAuth((s) => s.apiUrl).replace(/\/$/, '');
  const globalKey = useAuth((s) => s.apiKey);
  const { data: instances = [] } = useInstances();

  const pathParams = endpoint.parameters.filter((p) => p.in === 'path');
  const queryParams = endpoint.parameters.filter((p) => p.in === 'query');
  const bodyParam = endpoint.parameters.find((p) => p.in === 'body');
  const canHaveBody = ['POST', 'PUT', 'PATCH', 'DELETE'].includes(endpoint.method);

  const [auth, setAuth] = useState<AuthMode>('global');
  const [instanceId, setInstanceId] = useState('');
  const [custom, setCustom] = useState('');
  const [pathVals, setPathVals] = useState<Record<string, string>>({});
  const [queryVals, setQueryVals] = useState<Record<string, string>>({});
  const [body, setBody] = useState(() => {
    const ex = exampleFromRef(spec, bodyParam?.schema?.$ref);
    return ex ? JSON.stringify(ex, null, 2) : '';
  });
  const [extraHeaders, setExtraHeaders] = useState('');
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState<Sent | null>(null);
  const [error, setError] = useState('');
  const [tab, setTab] = useState<'body' | 'headers' | 'request'>('body');

  useEffect(() => {
    if (auth === 'instance' && !instanceId) {
      const first = instances.find((i) => i.connected) ?? instances[0];
      if (first) setInstanceId(first.id);
    }
  }, [auth, instanceId, instances]);

  const key = auth === 'global' ? globalKey : auth === 'custom' ? custom : instances.find((i) => i.id === instanceId)?.token ?? '';

  const resolvedPath = useMemo(() => {
    let p = endpoint.path;
    for (const [k, v] of Object.entries(pathVals)) p = p.replace(`{${k}}`, encodeURIComponent(v));
    return p;
  }, [endpoint.path, pathVals]);

  const query = useMemo(() => Object.fromEntries(Object.entries(queryVals).filter(([, v]) => v !== '')), [queryVals]);

  const bodyState = useMemo(() => {
    if (!body.trim()) return 'empty' as const;
    try {
      JSON.parse(body);
      return 'valid' as const;
    } catch {
      return 'invalid' as const;
    }
  }, [body]);

  const parseHeaders = (raw: string): Record<string, string> => {
    const out: Record<string, string> = {};
    if (!raw.trim()) return out;
    try {
      const j = JSON.parse(raw) as Record<string, unknown>;
      for (const [k, v] of Object.entries(j)) out[k] = String(v);
      return out;
    } catch {
      /* fall back to "Key: value" lines */
    }
    for (const line of raw.split('\n')) {
      const i = line.indexOf(':');
      if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + 1).trim();
    }
    return out;
  };

  const unresolved = pathParams.some((p) => !pathVals[p.name]?.trim());

  const send = async () => {
    setError('');
    if (unresolved) return setError('Preencha todos os parâmetros de caminho.');
    if (canHaveBody && bodyState === 'invalid') return setError('O corpo não é um JSON válido.');
    setBusy(true);
    const headers = parseHeaders(extraHeaders);
    const sendBody = canHaveBody && body.trim() ? body : undefined;
    if (sendBody) headers['Content-Type'] ??= 'application/json';
    if (key) headers.apikey = key;
    const qs = new URLSearchParams(query as Record<string, string>).toString();
    const url = `${apiUrl}${resolvedPath}${qs ? `?${qs}` : ''}`;
    try {
      const res = await rawRequest(resolvedPath, { method: endpoint.method, query, body: sendBody, apikey: null, headers });
      setSent({ method: endpoint.method, res, url, headers, body: sendBody });
      setTab('body');
    } catch (e) {
      setSent(null);
      setError(e instanceof Error ? e.message : 'Falha de rede.');
    } finally {
      setBusy(false);
    }
  };

  const curl = useMemo(() => {
    const headers: Record<string, string> = {};
    if (canHaveBody && body.trim()) headers['Content-Type'] = 'application/json';
    if (key) headers.apikey = key;
    const qs = new URLSearchParams(query as Record<string, string>).toString();
    return buildCurl({ method: endpoint.method, url: `${apiUrl}${resolvedPath}${qs ? `?${qs}` : ''}`, headers, body: canHaveBody && body.trim() ? body : undefined });
  }, [endpoint.method, apiUrl, resolvedPath, query, key, body, canHaveBody]);

  const formatBody = () => {
    try {
      setBody(JSON.stringify(JSON.parse(body), null, 2));
    } catch {
      /* leave as is; the validity hint already tells the user */
    }
  };

  return (
    <div
      className="mx-auto max-w-4xl space-y-5 p-4 sm:p-6"
      onKeyDown={(e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') void send();
      }}
    >
      <div>
        <Button variant="ghost" size="sm" onClick={onBack} className="-ml-2 mb-2 lg:hidden">
          <ArrowLeft className="size-3.5" />
          Endpoints
        </Button>
        <div className="flex flex-wrap items-center gap-2.5">
          <MethodTag method={endpoint.method} className="h-6 w-14 text-xs" />
          <code className="min-w-0 flex-1 font-mono text-sm font-medium break-all">{endpoint.path}</code>
          <CopyButton value={curl} text="cURL" label="Copiar como cURL" variant="secondary" />
        </div>
        {endpoint.summary ? <p className="mt-2 text-[13px] font-medium">{endpoint.summary}</p> : null}
        {endpoint.description && endpoint.description !== endpoint.summary ? <p className="mt-0.5 text-[13px] text-muted">{endpoint.description}</p> : null}
        <Badge className="mt-2">{endpoint.tag}</Badge>
      </div>

      <div className="space-y-4 rounded-card border border-line bg-surface p-4 shadow-card">
        <div className="space-y-2">
          <p className="text-xs font-medium">Autenticação (header apikey)</p>
          <div className="flex flex-wrap items-center gap-2">
            <Segmented
              label="Origem da chave"
              size="sm"
              value={auth}
              onChange={setAuth}
              options={[
                { value: 'global', label: 'Chave global' },
                { value: 'instance', label: 'Token de instância' },
                { value: 'custom', label: 'Outra' },
              ]}
            />
            {auth === 'instance' ? (
              instances.length ? (
                <Select value={instanceId} onChange={(e) => setInstanceId(e.target.value)} aria-label="Instância" className="w-48">
                  {instances.map((i) => (
                    <option key={i.id} value={i.id}>
                      {i.name}
                    </option>
                  ))}
                </Select>
              ) : (
                <span className="text-xs text-muted">Nenhuma instância cadastrada.</span>
              )
            ) : null}
            {auth === 'custom' ? <Input className="w-64" value={custom} onChange={(e) => setCustom(e.target.value)} placeholder="Cole a apikey" aria-label="Chave personalizada" spellCheck={false} /> : null}
          </div>
        </div>

        {pathParams.length ? (
          <ParamGrid title="Parâmetros de caminho" params={pathParams} values={pathVals} onChange={(k, v) => setPathVals((s) => ({ ...s, [k]: v }))} />
        ) : null}
        {queryParams.length ? (
          <ParamGrid title="Parâmetros de consulta" params={queryParams} values={queryVals} onChange={(k, v) => setQueryVals((s) => ({ ...s, [k]: v }))} />
        ) : null}

        {canHaveBody ? (
          <Field
            label="Corpo (JSON)"
            hint={
              bodyState === 'invalid' ? <span className="text-danger">JSON inválido</span> : bodyState === 'valid' ? 'JSON válido' : 'Sem corpo'
            }
          >
            {(id) => (
              <div className="relative">
                <Textarea id={id} value={body} onChange={(e) => setBody(e.target.value)} rows={Math.min(16, Math.max(5, body.split('\n').length + 1))} spellCheck={false} className="font-mono text-xs leading-relaxed" invalid={bodyState === 'invalid'} />
                <Button variant="secondary" size="sm" onClick={formatBody} disabled={bodyState !== 'valid'} className="absolute top-2 right-2">
                  <Wand2 className="size-3" />
                  Formatar
                </Button>
              </div>
            )}
          </Field>
        ) : null}

        <details className="group">
          <summary className="cursor-pointer text-xs font-medium text-muted select-none hover:text-fg">Cabeçalhos extras</summary>
          <Textarea className="mt-2 font-mono text-xs" rows={3} value={extraHeaders} onChange={(e) => setExtraHeaders(e.target.value)} placeholder={'X-Custom: valor\nou {"X-Custom": "valor"}'} spellCheck={false} aria-label="Cabeçalhos extras" />
        </details>

        {error ? <Alert tone="danger">{error}</Alert> : null}

        <div className="flex items-center gap-3">
          <Button variant="primary" onClick={() => void send()} loading={busy}>
            {!busy ? <Play className="size-3.5" /> : null}
            Enviar requisição
          </Button>
          <span className="hidden text-xs text-subtle sm:block">
            <kbd className="rounded border border-line-strong bg-surface-2 px-1 font-mono text-[10px]">Ctrl</kbd> + <kbd className="rounded border border-line-strong bg-surface-2 px-1 font-mono text-[10px]">Enter</kbd>
          </span>
        </div>
      </div>

      {sent ? <ResponsePanel sent={sent} tab={tab} onTab={setTab} /> : null}
    </div>
  );
}

function ParamGrid({ title, params, values, onChange }: { title: string; params: Endpoint['parameters']; values: Record<string, string>; onChange: (k: string, v: string) => void }) {
  return (
    <div className="space-y-2">
      <p className="text-xs font-medium">{title}</p>
      <div className="grid gap-3 sm:grid-cols-2">
        {params.map((p) => (
          <Field key={p.name} label={p.name} optional={!p.required} hint={p.description}>
            {(id) => <Input id={id} value={values[p.name] ?? ''} onChange={(e) => onChange(p.name, e.target.value)} placeholder={p.type ?? 'string'} spellCheck={false} autoComplete="off" />}
          </Field>
        ))}
      </div>
    </div>
  );
}

function ResponsePanel({ sent, tab, onTab }: { sent: Sent; tab: 'body' | 'headers' | 'request'; onTab: (t: 'body' | 'headers' | 'request') => void }) {
  const { res } = sent;
  const ref = useRef<HTMLDivElement>(null);
  // The response renders below the form; bring it into view so a send visibly "does something".
  useEffect(() => ref.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }), [sent]);
  const tone = res.status < 300 ? 'ok' : res.status < 400 ? 'info' : res.status < 500 ? 'warn' : 'danger';
  const bodyText = typeof res.data === 'string' ? res.data : JSON.stringify(res.data, null, 2);
  const isJson = typeof res.data === 'object' && res.data !== null;

  return (
    <div ref={ref} className="animate-pop-in space-y-3 rounded-card border border-line bg-surface p-4 shadow-card">
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone={tone} className="h-6 px-2.5 text-xs">
          {res.status} {res.statusText}
        </Badge>
        <span className="text-xs text-muted">{res.ms} ms</span>
        <span className="text-xs text-muted">{formatBytes(res.size)}</span>
        <Segmented
          className="ml-auto"
          size="sm"
          label="Seção da resposta"
          value={tab}
          onChange={onTab}
          options={[
            { value: 'body', label: 'Corpo' },
            { value: 'headers', label: 'Cabeçalhos' },
            { value: 'request', label: 'Requisição' },
          ]}
        />
      </div>
      {tab === 'body' ? (
        bodyText ? <CodeBlock code={bodyText} language={isJson ? 'json' : 'text'} maxHeight="32rem" /> : <p className="py-4 text-center text-xs text-muted">Resposta vazia.</p>
      ) : null}
      {tab === 'headers' ? <CodeBlock code={JSON.stringify(res.headers, null, 2)} language="json" maxHeight="24rem" /> : null}
      {tab === 'request' ? (
        <CodeBlock code={buildCurl({ method: sent.method, url: sent.url, headers: sent.headers, body: sent.body })} language="bash" maxHeight="24rem" />
      ) : null}
    </div>
  );
}
