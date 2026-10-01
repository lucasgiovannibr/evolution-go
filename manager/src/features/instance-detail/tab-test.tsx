import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  ChartColumn,
  CircleCheck,
  CircleX,
  Contact,
  History,
  Image as ImageIcon,
  LayoutList,
  Link2,
  MapPin,
  MessageSquareText,
  MousePointerClick,
  PlugZap,
  Radio,
  Send,
  SquareStack,
  Sticker,
  Terminal,
} from 'lucide-react';
import { sendMessage } from '@/api/messages';
import { ApiError } from '@/lib/http';
import { buildCurl } from '@/lib/curl';
import { cn } from '@/lib/cn';
import { errMsg } from '@/hooks/use-instances';
import { useAuth } from '@/stores/auth';
import { Alert } from '@/components/ui/feedback';
import { Button } from '@/components/ui/button';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { CodeBlock } from '@/components/ui/code-block';
import { Field, Input, Select, Textarea } from '@/components/ui/form';
import { Segmented } from '@/components/ui/segmented';
import { Switch } from '@/components/ui/switch';
import { useInstanceActions } from '@/features/instances/actions';
import { useInstanceContext } from './instance-page';
import { defaultsOf, GROUPS, kindOf, KINDS, type FieldDef, type KindDef, type SendKind } from './send-catalog';

const ICONS: Record<SendKind, React.ReactNode> = {
  text: <MessageSquareText />,
  link: <Link2 />,
  location: <MapPin />,
  contact: <Contact />,
  poll: <ChartColumn />,
  media: <ImageIcon />,
  sticker: <Sticker />,
  button: <MousePointerClick />,
  list: <LayoutList />,
  carousel: <SquareStack />,
  'status/text': <Radio />,
  'status/media': <Radio />,
};

const NUMBER_KEY = 'evolution-test-number';
const HISTORY_LIMIT = 8;
const FALLBACK_NUMBER = '5511999990000';

const readNumber = () => {
  try {
    return localStorage.getItem(NUMBER_KEY) ?? '';
  } catch {
    return '';
  }
};
const saveNumber = (n: string) => {
  try {
    localStorage.setItem(NUMBER_KEY, n);
  } catch {
    /* the number is just not remembered */
  }
};

interface Attempt {
  n: number;
  at: Date;
  label: string;
  ok: boolean;
  detail: string;
}

const failure = (e: unknown) => {
  const msg = errMsg(e, 'Erro ao enviar.');
  if (!(e instanceof ApiError)) return msg;
  return `${msg} (HTTP ${e.status}${e.code ? ` · ${e.code}` : ''})`;
};

function FieldControl({ def, value, onChange, error }: { def: FieldDef; value: string; onChange: (v: string) => void; error: boolean }) {
  return (
    <Field label={def.label} hint={def.hint} optional={def.optional}>
      {(id) => {
        if (def.control === 'textarea') {
          return <Textarea id={id} rows={def.rows ?? 3} value={value} onChange={(e) => onChange(e.target.value)} placeholder={def.placeholder} invalid={error && !def.optional && !value.trim()} />;
        }
        if (def.control === 'select') {
          return (
            <Select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
              {def.options?.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          );
        }
        return <Input id={id} value={value} onChange={(e) => onChange(e.target.value)} placeholder={def.placeholder} inputMode={def.inputMode} autoComplete="off" invalid={error && !def.optional && !value.trim()} />;
      }}
    </Field>
  );
}

function KindPicker({ value, onChange }: { value: SendKind; onChange: (k: SendKind) => void }) {
  return (
    <div role="radiogroup" aria-label="Tipo de mensagem" className="space-y-2.5">
      {GROUPS.map((g) => (
        <div key={g.id} className="flex flex-wrap items-center gap-1.5">
          <span className="w-20 shrink-0 text-[11px] font-medium tracking-wide text-subtle uppercase">{g.label}</span>
          {KINDS.filter((k) => k.group === g.id).map((k) => {
            const active = k.id === value;
            return (
              <button
                key={k.id}
                type="button"
                role="radio"
                aria-checked={active}
                title={k.description}
                onClick={() => onChange(k.id)}
                className={cn(
                  'inline-flex h-7 items-center gap-1.5 rounded-control border px-2.5 text-[13px] font-medium whitespace-nowrap transition-colors pointer-coarse:h-9 [&_svg]:size-3.5',
                  active ? 'border-brand bg-brand-soft text-brand-text' : 'border-line-strong bg-surface text-muted hover:border-subtle hover:text-fg',
                )}
              >
                {ICONS[k.id]}
                {k.label}
              </button>
            );
          })}
        </div>
      ))}
    </div>
  );
}

export function TabTest() {
  const { instance } = useInstanceContext();
  const actions = useInstanceActions();
  const apiUrl = useAuth((s) => s.apiUrl);

  const [number, setNumber] = useState(readNumber);
  const [kind, setKind] = useState<SendKind>('text');
  // Each kind keeps what was typed while the person looks at another one.
  const [values, setValues] = useState(defaultsOf);
  const [delay, setDelay] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [touched, setTouched] = useState(false);
  const [view, setView] = useState<'json' | 'curl'>('json');
  const [attempts, setAttempts] = useState<Attempt[]>([]);
  const [last, setLast] = useState<Attempt | null>(null);

  const def: KindDef = kindOf(kind);
  const v = values[kind];
  const digits = number.replace(/\D/g, '');
  const numberError = touched && def.recipient && digits.length < 10 ? 'Informe o número completo com DDI, ex.: 5511999990000.' : null;
  const formError = touched ? def.validate(v) : null;

  const payload = useMemo(() => {
    const body = def.build(digits || FALLBACK_NUMBER, v);
    const ms = Number(delay);
    if (def.recipient && Number.isFinite(ms) && ms > 0) body.delay = Math.round(ms);
    return body;
  }, [def, digits, v, delay]);

  const curl = useMemo(
    () => buildCurl({ method: 'POST', url: `${apiUrl.replace(/\/+$/, '')}/send/${kind}`, headers: { apikey: '<token da instância>', 'Content-Type': 'application/json' }, body: JSON.stringify(payload) }),
    [apiUrl, kind, payload],
  );

  const record = (a: Omit<Attempt, 'n' | 'at'>) => {
    const entry = { ...a, n: Date.now(), at: new Date() };
    setLast(entry);
    setAttempts((list) => [entry, ...list].slice(0, HISTORY_LIMIT));
  };

  const send = useMutation({
    mutationFn: () => sendMessage(instance.token, kind, payload),
    onSuccess: (r) =>
      record({
        label: r.fallback === 'buttons' ? `${def.label} (enviada como botões, ${r.parts ?? 1} ${r.parts === 1 ? 'mensagem' : 'mensagens'})` : def.label,
        ok: true,
        detail: r.messageId,
      }),
    onError: (e) => record({ label: def.label, ok: false, detail: failure(e) }),
  });

  const setField = (key: string, value: string) =>
    setValues((all) => ({ ...all, [kind]: def.patch ? def.patch(key, value, all[kind]) : { ...all[kind], [key]: value } }));

  const pickKind = (k: SendKind) => {
    setKind(k);
    setLast(null);
    setConfirmed(false);
    setTouched(false);
  };

  const submit = () => {
    setTouched(true);
    if (def.recipient && digits.length < 10) return;
    if (def.validate(v)) return;
    if (def.confirm && !confirmed) return;
    setLast(null);
    saveNumber(number);
    send.mutate();
  };

  const disabled = !instance.connected;

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-5">
      <div className="space-y-4 lg:col-span-3">
        {disabled ? (
          <Alert
            tone="warn"
            title="Instância desconectada"
            action={
              <Button size="sm" variant="primary" onClick={() => actions.connect(instance)}>
                <PlugZap className="size-3.5" />
                Conectar
              </Button>
            }
          >
            Conecte o número para poder enviar mensagens de teste.
          </Alert>
        ) : null}

        <Card>
          <CardHeader title="Mensagem de teste" description="Envia uma mensagem real pela API usando o token desta instância." icon={<Send />} />
          <CardBody className="space-y-4">
            {def.recipient ? (
              <Field label="Número de destino" error={numberError} hint="Com código do país, apenas dígitos. O DDI do Brasil é 55.">
                {(id) => (
                  <Input id={id} value={number} onChange={(e) => setNumber(e.target.value)} placeholder="5511999990000" inputMode="numeric" autoComplete="off" invalid={!!numberError} />
                )}
              </Field>
            ) : null}

            <div className="space-y-1.5">
              <p className="text-xs font-medium">Tipo</p>
              <KindPicker value={kind} onChange={pickKind} />
              <p className="text-xs text-muted">{def.description}</p>
            </div>

            {def.note ? (
              <Alert tone={def.note.tone === 'warn' ? 'warn' : 'info'} className="text-[13px]">
                {def.note.text}
              </Alert>
            ) : null}

            {def.fields
              .filter((f) => !f.showIf || f.showIf(v))
              .map((f) => (
                <FieldControl key={f.key} def={f} value={v[f.key] ?? ''} onChange={(val) => setField(f.key, val)} error={touched} />
              ))}

            {def.recipient ? (
              <Field label="Atraso de digitação (ms)" optional hint="Mostra “digitando…” por esse tempo antes de enviar.">
                {(id) => <Input id={id} value={delay} onChange={(e) => setDelay(e.target.value.replace(/\D/g, ''))} inputMode="numeric" placeholder="0" className="max-w-40" />}
              </Field>
            ) : null}

            {def.confirm ? (
              <div className="flex items-start gap-3 rounded-control border border-warn/30 bg-warn-soft p-3">
                <Switch checked={confirmed} onChange={setConfirmed} label="Confirmo a publicação do status" />
                <p className="text-[13px] leading-snug">{def.confirm}</p>
              </div>
            ) : null}

            {formError ? <p className="text-xs text-danger">{formError}</p> : null}
            {touched && def.confirm && !confirmed && !formError ? <p className="text-xs text-danger">Confirme a publicação para enviar.</p> : null}

            <div className="flex items-center gap-3">
              <Button variant="primary" onClick={submit} loading={send.isPending} disabled={disabled}>
                {!send.isPending ? <Send className="size-3.5" /> : null}
                Enviar teste
              </Button>
            </div>
          </CardBody>
        </Card>
      </div>

      <div className="space-y-4 lg:col-span-2">
        {last ? (
          last.ok ? (
            <Alert tone="ok" title="Enviado com sucesso" className="animate-pop-in">
              {last.label}
              {last.detail ? (
                <>
                  {' · ID '}
                  <code className="font-mono text-xs break-all">{last.detail}</code>
                </>
              ) : null}
            </Alert>
          ) : (
            <Alert tone="danger" title="Falha no envio" className="animate-pop-in">
              {last.detail}
            </Alert>
          )
        ) : null}

        <Card>
          <CardHeader
            title={view === 'json' ? 'Payload' : 'cURL'}
            description={`POST /send/${kind}`}
            icon={view === 'json' ? undefined : <Terminal />}
            actions={
              <Segmented
                size="sm"
                label="Formato da requisição"
                value={view}
                onChange={setView}
                options={[
                  { value: 'json', label: 'JSON' },
                  { value: 'curl', label: 'cURL' },
                ]}
              />
            }
          />
          <CardBody>
            {view === 'json' ? (
              <CodeBlock code={JSON.stringify(payload, null, 2)} language="json" maxHeight="26rem" />
            ) : (
              <CodeBlock code={curl} language="bash" maxHeight="26rem" />
            )}
          </CardBody>
        </Card>

        {attempts.length > 0 ? (
          <Card>
            <CardHeader title="Envios desta sessão" description="Some ao sair da página." icon={<History />} />
            <CardBody className="space-y-2">
              {attempts.map((a) => (
                <div key={a.n} className="flex items-start gap-2 text-[13px]">
                  {a.ok ? <CircleCheck className="mt-0.5 size-4 shrink-0 text-ok" /> : <CircleX className="mt-0.5 size-4 shrink-0 text-danger" />}
                  <div className="min-w-0">
                    <p className="font-medium">
                      {a.label} <span className="font-normal text-subtle tabular-nums">· {a.at.toLocaleTimeString('pt-BR')}</span>
                    </p>
                    <p className="font-mono text-[11px] break-all text-muted">{a.detail || 'sem ID retornado'}</p>
                  </div>
                </div>
              ))}
            </CardBody>
          </Card>
        ) : null}
      </div>
    </div>
  );
}

