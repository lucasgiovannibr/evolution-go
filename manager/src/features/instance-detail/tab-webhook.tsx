import { Cable, Network, Radio, Webhook, X } from 'lucide-react';
import type { Instance, ProducerState } from '@/api/types';
import { useDraft } from '@/hooks/use-draft';
import { useSaveConnection } from '@/hooks/use-instances';
import { ALL_EVENT_IDS, EVENT_GROUPS, coversAllEvents, normalizeEvents, toSubscribe } from '@/lib/events';
import { isHttpUrl } from '@/lib/format';
import { cn } from '@/lib/cn';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { Field, Input } from '@/components/ui/form';
import { SaveBar } from '@/components/ui/save-bar';
import { Segmented } from '@/components/ui/segmented';
import { useInstanceContext } from './instance-page';

interface Draft {
  url: string;
  events: string[];
  rabbitmq: ProducerState;
  websocket: ProducerState;
  nats: ProducerState;
}

const fromInstance = (i: Instance): Draft => ({
  url: i.webhook,
  // A new instance stores no events; the server then falls back to MESSAGE, so show that effective default.
  events: normalizeEvents(i.events.length ? i.events : ['MESSAGE']),
  rabbitmq: i.producers.rabbitmq,
  websocket: i.producers.websocket,
  nats: i.producers.nats,
});

const CHANNELS = [
  { key: 'rabbitmq', label: 'RabbitMQ', hint: 'Publica os eventos em filas AMQP.', icon: <Network /> },
  { key: 'websocket', label: 'WebSocket', hint: 'Transmite os eventos em tempo real por conexão WebSocket.', icon: <Radio /> },
  { key: 'nats', label: 'NATS', hint: 'Publica os eventos em tópicos NATS.', icon: <Cable /> },
] as const;

export function TabWebhook() {
  const { instance } = useInstanceContext();
  const save = useSaveConnection(instance);
  const server = fromInstance(instance);
  const { draft, patch, dirty, reset } = useDraft(server);

  const urlInvalid = draft.url.trim() !== '' && !isHttpUrl(draft.url.trim());
  const noEvents = draft.events.length === 0;
  const allSelected = coversAllEvents(draft.events);

  const toggle = (id: string) =>
    patch({ events: normalizeEvents(draft.events.includes(id) ? draft.events.filter((e) => e !== id) : [...draft.events, id]) });

  const submit = () => {
    if (urlInvalid || noEvents) return;
    const url = draft.url.trim();
    save.mutate({
      // "" leaves the webhook untouched on the server; "disabled" is the explicit way to remove it.
      webhookUrl: url || (server.url ? 'disabled' : ''),
      subscribe: toSubscribe(draft.events, allSelected),
      rabbitmqEnable: draft.rabbitmq,
      websocketEnable: draft.websocket,
      natsEnable: draft.nats,
    });
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader title="Webhook" description="Cada evento selecionado chega como um POST JSON neste endereço." icon={<Webhook />} />
        <CardBody>
          <Field
            label="URL do webhook"
            error={urlInvalid ? 'Informe uma URL http:// ou https:// válida.' : null}
            hint={server.url && !draft.url.trim() ? 'Ao salvar com o campo vazio, o webhook será removido.' : 'Deixe vazio para não usar webhook.'}
          >
            {(id) => (
              <Input
                id={id}
                value={draft.url}
                onChange={(e) => patch({ url: e.target.value })}
                placeholder="https://seu-servidor.com/webhook"
                inputMode="url"
                spellCheck={false}
                autoComplete="off"
                invalid={urlInvalid}
                leading={<Webhook />}
                trailing={
                  draft.url ? (
                    <Button variant="ghost" size="icon-sm" onClick={() => patch({ url: '' })} aria-label="Limpar URL">
                      <X className="size-3.5" />
                    </Button>
                  ) : undefined
                }
              />
            )}
          </Field>
        </CardBody>
      </Card>

      <Card>
        <CardHeader
          title="Eventos"
          description="Escolha o que este webhook e os canais abaixo devem receber."
          actions={
            <>
              <Badge tone={allSelected ? 'brand' : 'neutral'}>{allSelected ? 'Todos' : `${draft.events.length} de ${ALL_EVENT_IDS.length}`}</Badge>
              <Button size="sm" variant="ghost" onClick={() => patch({ events: [...ALL_EVENT_IDS] })} disabled={allSelected}>
                Marcar todos
              </Button>
              <Button size="sm" variant="ghost" onClick={() => patch({ events: [] })} disabled={noEvents}>
                Limpar
              </Button>
            </>
          }
        />
        <CardBody className="space-y-5">
          {EVENT_GROUPS.map((g) => (
            <fieldset key={g.id}>
              <legend className="mb-2 text-[11px] font-medium tracking-wide text-subtle uppercase">{g.label}</legend>
              <div className="grid gap-2 sm:grid-cols-2">
                {g.events.map((ev) => {
                  const on = draft.events.includes(ev.id);
                  return (
                    <label
                      key={ev.id}
                      className={cn(
                        'flex cursor-pointer items-start gap-2.5 rounded-control border px-3 py-2 transition-colors',
                        'has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-brand',
                        on ? 'border-line-strong bg-surface shadow-card' : 'border-line bg-surface-2/40 text-muted hover:border-line-strong',
                      )}
                    >
                      <input type="checkbox" className="peer sr-only" checked={on} onChange={() => toggle(ev.id)} />
                      <span
                        aria-hidden
                        className={cn(
                          'mt-0.5 flex size-4 shrink-0 items-center justify-center rounded border transition-colors',
                          on ? 'border-brand bg-brand text-brand-fg' : 'border-line-strong bg-surface',
                        )}
                      >
                        {on ? (
                          <svg viewBox="0 0 12 12" className="size-3" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M2.5 6.5l2.2 2.2L9.5 3.8" />
                          </svg>
                        ) : null}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="flex items-baseline gap-2">
                          <span className={cn('text-[13px] font-medium', !on && 'text-muted')}>{ev.label}</span>
                          <code className="font-mono text-[10px] text-subtle">{ev.id}</code>
                        </span>
                        <span className="block text-xs text-muted">{ev.hint}</span>
                      </span>
                    </label>
                  );
                })}
              </div>
            </fieldset>
          ))}
          {noEvents ? <p className="text-xs text-danger">Selecione ao menos um evento para poder salvar.</p> : null}
        </CardBody>
      </Card>

      <Card>
        <CardHeader title="Outros canais" description="Entregam os mesmos eventos por filas e sockets, além do webhook." />
        <CardBody>
          <ul className="divide-y divide-line">
            {CHANNELS.map((c) => {
              const value = draft[c.key];
              const hasDefault = server[c.key] === '';
              return (
                <li key={c.key} className="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0">
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-control bg-surface-2 text-muted [&_svg]:size-4">{c.icon}</span>
                  <div className="min-w-0 flex-1 basis-48">
                    <p className="text-[13px] font-medium">{c.label}</p>
                    <p className="text-xs text-muted">{c.hint}</p>
                  </div>
                  <Segmented
                    label={`${c.label}: estado`}
                    size="sm"
                    value={value}
                    onChange={(v) => patch({ [c.key]: v } as Partial<Draft>)}
                    options={[
                      ...(hasDefault ? [{ value: '' as ProducerState, label: 'Padrão', title: 'Segue a configuração do servidor' }] : []),
                      { value: 'enabled' as ProducerState, label: 'Ativo' },
                      { value: 'disabled' as ProducerState, label: 'Inativo' },
                    ]}
                  />
                </li>
              );
            })}
          </ul>
        </CardBody>
      </Card>

      <SaveBar dirty={dirty} saving={save.isPending} onSave={submit} onReset={reset} disabled={urlInvalid || noEvents} />
    </div>
  );
}
