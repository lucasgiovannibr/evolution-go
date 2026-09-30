import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { LayoutList, MessageSquareText, MousePointerClick, PlugZap, Send, SquareStack } from 'lucide-react';
import { sendMessage, type SendKind } from '@/api/messages';
import { errMsg } from '@/hooks/use-instances';
import { Alert } from '@/components/ui/feedback';
import { Button } from '@/components/ui/button';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { CodeBlock } from '@/components/ui/code-block';
import { Field, Input, Select, Textarea } from '@/components/ui/form';
import { Segmented } from '@/components/ui/segmented';
import { useInstanceActions } from '@/features/instances/actions';
import { useInstanceContext } from './instance-page';
import { PRESETS, presetsOf, type PresetGroup } from './presets';

const KINDS: { value: SendKind; label: string; icon: React.ReactNode }[] = [
  { value: 'text', label: 'Texto', icon: <MessageSquareText /> },
  { value: 'button', label: 'Botões', icon: <MousePointerClick /> },
  { value: 'list', label: 'Lista', icon: <LayoutList /> },
  { value: 'carousel', label: 'Carrossel', icon: <SquareStack /> },
];

type Result = { ok: true; messageId: string; label: string } | { ok: false; error: string };

export function TabTest() {
  const { instance } = useInstanceContext();
  const actions = useInstanceActions();

  const [number, setNumber] = useState('');
  const [kind, setKind] = useState<SendKind>('text');
  const [text, setText] = useState('');
  const [presetId, setPresetId] = useState<Record<PresetGroup, string>>({
    button: 'btn_reply_1',
    list: 'list',
    carousel: 'carousel_reply',
  });
  const [touched, setTouched] = useState(false);
  const [result, setResult] = useState<Result | null>(null);

  const digits = number.replace(/\D/g, '');
  const numberError = touched && digits.length < 10 ? 'Informe o número completo com DDI, ex.: 5511999990000.' : null;

  const preset = kind === 'text' ? null : PRESETS.find((p) => p.id === presetId[kind]) ?? null;
  const payload = useMemo(() => (kind === 'text' ? { number: digits, text } : preset?.build(digits || '5511999990000') ?? {}), [kind, digits, text, preset]);

  const send = useMutation({
    mutationFn: () => sendMessage(instance.token, kind, payload),
    onSuccess: (r) => setResult({ ok: true, messageId: r.messageId, label: preset?.label ?? 'Mensagem de texto' }),
    onError: (e) => setResult({ ok: false, error: errMsg(e, 'Erro ao enviar.') }),
  });

  const submit = () => {
    setTouched(true);
    if (digits.length < 10) return;
    if (kind === 'text' && !text.trim()) return;
    setResult(null);
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
            <Field label="Número de destino" error={numberError} hint="Com código do país, apenas dígitos. O DDI do Brasil é 55.">
              {(id) => (
                <Input
                  id={id}
                  value={number}
                  onChange={(e) => setNumber(e.target.value)}
                  placeholder="5511999990000"
                  inputMode="numeric"
                  autoComplete="off"
                  invalid={!!numberError}
                />
              )}
            </Field>

            <div className="space-y-1.5">
              <p className="text-xs font-medium">Tipo</p>
              <Segmented
                label="Tipo de mensagem"
                value={kind}
                onChange={(k) => {
                  setKind(k);
                  setResult(null);
                }}
                options={KINDS.map((k) => ({ value: k.value, label: (<>{k.icon}{k.label}</>) }))}
              />
            </div>

            {kind === 'text' ? (
              <Field label="Mensagem" error={touched && !text.trim() ? 'Escreva a mensagem.' : null}>
                {(id) => <Textarea id={id} rows={4} value={text} onChange={(e) => setText(e.target.value)} placeholder="Digite sua mensagem…" invalid={touched && !text.trim()} />}
              </Field>
            ) : (
              <Field label="Modelo" hint={preset?.description}>
                {(id) => (
                  <Select id={id} value={presetId[kind]} onChange={(e) => setPresetId((s) => ({ ...s, [kind]: e.target.value }))}>
                    {presetsOf(kind).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.label}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
            )}

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
        {result ? (
          result.ok ? (
            <Alert tone="ok" title="Enviado com sucesso" className="animate-pop-in">
              {result.label}
              {result.messageId ? (
                <>
                  {' · ID '}
                  <code className="font-mono text-xs break-all">{result.messageId}</code>
                </>
              ) : null}
            </Alert>
          ) : (
            <Alert tone="danger" title="Falha no envio" className="animate-pop-in">
              {result.error}
            </Alert>
          )
        ) : null}

        {(
          <Card>
            <CardHeader title="Payload" description={`POST /send/${kind}`} />
            <CardBody>
              <CodeBlock code={JSON.stringify(payload, null, 2)} language="json" maxHeight="26rem" />
            </CardBody>
          </Card>
        )}
      </div>
    </div>
  );
}
