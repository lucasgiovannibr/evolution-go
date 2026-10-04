import { useCallback, useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ExternalLink, Fingerprint, Hash, QrCode, RefreshCw, Smartphone } from 'lucide-react';
import { connectInstance, getQr, pairPhone } from '@/api/instances';
import type { QrInfo } from '@/api/types';
import { errMsg, keys, useInstance } from '@/hooks/use-instances';
import { formatPhone } from '@/lib/format';
import { Alert, Skeleton } from '@/components/ui/feedback';
import { Button } from '@/components/ui/button';
import { CopyButton } from '@/components/ui/copy-button';
import { Dialog } from '@/components/ui/dialog';
import { Field, Input } from '@/components/ui/form';
import { Segmented } from '@/components/ui/segmented';

type Mode = 'qr' | 'phone';
type Status = 'starting' | 'ready' | 'error';

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
const alreadyLoggedIn = (e: unknown) => /already logged in/i.test(errMsg(e, ''));

const STEPS_QR = ['Abra o WhatsApp no celular', 'Toque em Configurações › Dispositivos conectados', 'Toque em Conectar um dispositivo', 'Aponte a câmera para este QR Code'];
const STEPS_PHONE = ['Abra o WhatsApp no celular', 'Toque em Configurações › Dispositivos conectados', 'Toque em Conectar um dispositivo', 'Escolha “Conectar com número de telefone” e digite o código'];

function Steps({ items }: { items: string[] }) {
  return (
    <ol className="space-y-2.5">
      {items.map((s, i) => (
        <li key={s} className="flex gap-2.5 text-[13px] text-muted">
          <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-surface-3 text-[11px] font-semibold text-fg">{i + 1}</span>
          <span className="pt-px">{s}</span>
        </li>
      ))}
    </ol>
  );
}

function PasskeyPanel({ qr }: { qr: QrInfo }) {
  const missingUrl = !qr.passkeyOpenUrl || qr.passkeyOpenUrl.includes('<SET_PASSKEY_PUBLIC_URL>');
  return (
    <div className="space-y-4 pb-2">
      <p className="text-[13px] text-muted">
        Esta conta exige uma <strong className="text-fg">chave de acesso</strong> para concluir a conexão. Não há QR Code para escanear.
      </p>
      {qr.passkeyCode ? (
        <div className="rounded-card bg-surface-2 p-3 text-center ring-1 ring-line">
          <p className="text-xs text-muted">Código de confirmação</p>
          <p className="mt-1 font-mono text-xl font-semibold tracking-[0.3em]">{qr.passkeyCode}</p>
          <p className="mt-1 text-xs text-muted">Confirme na aba do WhatsApp Web se o código é o mesmo.</p>
        </div>
      ) : null}
      <Steps
        items={[
          'Instale a extensão WhatyGo Passkey Helper no Chrome/Edge (só na primeira vez)',
          'Clique em “Abrir WhatsApp Web” abaixo',
          'Na aba aberta, use “Autenticar com chave de acesso” e confirme com biometria/PIN',
          'Volte aqui e aguarde a conexão concluir',
        ]}
      />
      {missingUrl ? (
        <Alert tone="warn" title="URL pública não configurada">
          Defina <code className="font-mono text-xs">PASSKEY_PUBLIC_URL</code> no servidor com um endereço acessível pelo navegador para habilitar o botão.
        </Alert>
      ) : null}
      <div className="flex flex-wrap gap-2">
        <Button variant="primary" disabled={missingUrl} onClick={() => qr.passkeyOpenUrl && window.open(qr.passkeyOpenUrl, '_blank', 'noopener,noreferrer')}>
          <ExternalLink className="size-4" />
          Abrir WhatsApp Web
        </Button>
        <a
          href="https://github.com/lucasgiovannibr/whatygo/tree/main/passkey-helper"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex h-8 items-center px-2 text-[13px] font-medium text-brand-text hover:underline"
        >
          Baixar a extensão
        </a>
      </div>
    </div>
  );
}

function Body({ instanceId, onClose }: { instanceId: string; onClose: () => void }) {
  const qc = useQueryClient();
  const { data: inst } = useInstance(instanceId, { fast: true });
  const token = inst?.token;
  const connected = !!inst?.connected;

  const [mode, setMode] = useState<Mode>('qr');
  const [status, setStatus] = useState<Status>('starting');
  const [error, setError] = useState('');
  const [qr, setQr] = useState<QrInfo | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [phone, setPhone] = useState('');
  const [pairCode, setPairCode] = useState('');
  const [pairBusy, setPairBusy] = useState(false);
  const [pairError, setPairError] = useState('');
  const [refreshing, setRefreshing] = useState(false);

  const refreshInstance = useCallback(() => {
    void qc.invalidateQueries({ queryKey: keys.one(instanceId) });
    void qc.invalidateQueries({ queryKey: keys.list });
  }, [qc, instanceId]);

  // Start the client and wait for the first QR (the server holds the request until one exists).
  useEffect(() => {
    if (!token || connected || mode !== 'qr') return;
    let cancelled = false;
    void (async () => {
      setStatus('starting');
      setError('');
      try {
        await connectInstance(token);
        for (let i = 0; i < 8 && !cancelled; i++) {
          let q: QrInfo | null = null;
          try {
            q = await getQr(token);
          } catch (e) {
            if (alreadyLoggedIn(e)) return refreshInstance();
            throw e;
          }
          if (q.qrcode || q.passkeyStage) {
            if (!cancelled) {
              setQr(q);
              setStatus('ready');
            }
            return;
          }
          await sleep(1500);
        }
        if (!cancelled) {
          setStatus('error');
          setError('O QR Code ainda não ficou disponível. Tente novamente em alguns segundos.');
        }
      } catch (e) {
        if (!cancelled) {
          setStatus('error');
          setError(errMsg(e, 'Não foi possível iniciar a conexão.'));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [token, connected, mode, attempt, refreshInstance]);

  // WhatsApp rotates the QR every ~20s; keep it fresh while waiting.
  useEffect(() => {
    if (!token || connected || mode !== 'qr' || status !== 'ready') return;
    const id = window.setInterval(async () => {
      try {
        const q = await getQr(token);
        if (q.qrcode || q.passkeyStage) setQr(q);
      } catch (e) {
        if (alreadyLoggedIn(e)) refreshInstance();
      }
    }, 5000);
    return () => window.clearInterval(id);
  }, [token, connected, mode, status, refreshInstance]);

  const manualRefresh = async () => {
    if (!token) return;
    setRefreshing(true);
    try {
      const q = await getQr(token);
      if (q.qrcode || q.passkeyStage) setQr(q);
    } catch (e) {
      if (alreadyLoggedIn(e)) refreshInstance();
      else setError(errMsg(e));
    } finally {
      setRefreshing(false);
    }
  };

  const generatePairCode = async () => {
    if (!token) return;
    const digits = phone.replace(/\D/g, '');
    if (digits.length < 10) return setPairError('Informe o número completo com DDI, ex.: 5511999990000.');
    setPairError('');
    setPairBusy(true);
    try {
      await connectInstance(token);
      const code = await pairPhone(token, `+${digits}`);
      if (!code) throw new Error('O servidor não retornou um código de pareamento.');
      setPairCode(code);
    } catch (e) {
      setPairError(errMsg(e, 'Não foi possível gerar o código.'));
    } finally {
      setPairBusy(false);
    }
  };

  const title = connected ? 'WhatsApp conectado' : 'Conectar WhatsApp';

  return (
    <Dialog
      open
      onClose={onClose}
      size="lg"
      icon={connected ? <CheckCircle2 /> : <Smartphone />}
      title={title}
      description={
        connected ? undefined : (
          <>
            Vincule um número à instância <strong className="font-medium text-fg">{inst?.name ?? '…'}</strong>
          </>
        )
      }
      footer={
        connected ? (
          <Button variant="primary" onClick={onClose}>
            Concluir
          </Button>
        ) : (
          <>
            {mode === 'qr' && status === 'ready' && !qr?.passkeyStage ? (
              <Button onClick={manualRefresh} loading={refreshing} className="sm:mr-auto">
                {!refreshing ? <RefreshCw className="size-3.5" /> : null}
                Atualizar QR Code
              </Button>
            ) : null}
            <Button variant="secondary" onClick={onClose}>
              Fechar
            </Button>
          </>
        )
      }
    >
      {!inst ? (
        <div className="space-y-3 pb-3">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-56 w-full" />
        </div>
      ) : connected ? (
        <div className="flex flex-col items-center gap-3 py-8 text-center">
          <span className="flex size-14 items-center justify-center rounded-full bg-ok-soft text-ok ring-8 ring-ok-soft/50">
            <CheckCircle2 className="size-7" />
          </span>
          <div>
            <p className="text-[15px] font-semibold">{inst.name}</p>
            <p className="font-mono text-[13px] text-muted">{inst.number ? formatPhone(inst.number) : 'Número vinculado'}</p>
          </div>
          <p className="max-w-xs text-[13px] text-muted">A instância já está online e pronta para enviar e receber mensagens.</p>
        </div>
      ) : (
        <div className="space-y-4 pb-3">
          <Segmented
            label="Método de conexão"
            value={mode}
            onChange={(m) => {
              setMode(m);
              setPairError('');
            }}
            options={[
              { value: 'qr', label: (<><QrCode /> QR Code</>) },
              { value: 'phone', label: (<><Hash /> Código por telefone</>) },
            ]}
          />

          {mode === 'qr' ? (
            status === 'error' ? (
              <Alert
                tone="danger"
                title="Não foi possível gerar o QR Code"
                action={
                  <Button size="sm" onClick={() => setAttempt((a) => a + 1)}>
                    Tentar de novo
                  </Button>
                }
              >
                {error}
              </Alert>
            ) : qr?.passkeyStage ? (
              <div className="flex items-start gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-control bg-brand-soft text-brand-text">
                  <Fingerprint className="size-[18px]" />
                </span>
                <div className="min-w-0 flex-1">
                  <PasskeyPanel qr={qr} />
                </div>
              </div>
            ) : (
              <div className="grid gap-5 sm:grid-cols-[13.5rem_minmax(0,1fr)] sm:items-center">
                <div className="mx-auto w-full max-w-[13.5rem]">
                  <div className="relative aspect-square overflow-hidden rounded-card bg-white p-2.5 ring-1 ring-line-strong">
                    {status === 'ready' && qr?.qrcode ? (
                      <img src={qr.qrcode} alt="QR Code para conectar o WhatsApp" className="size-full" draggable={false} />
                    ) : (
                      <div className="flex size-full flex-col items-center justify-center gap-2 text-center">
                        <RefreshCw className="size-6 animate-spin text-neutral-400" />
                        <p className="text-xs text-neutral-500">Gerando QR Code…</p>
                      </div>
                    )}
                  </div>
                  <p className="mt-2 text-center text-xs text-subtle">Renova automaticamente</p>
                </div>
                <Steps items={STEPS_QR} />
              </div>
            )
          ) : (
            <div className="grid gap-5 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
              <div className="space-y-3">
                <Field label="Número do WhatsApp" hint="Com código do país, apenas dígitos." error={pairError}>
                  {(id) => (
                    <Input
                      id={id}
                      autoFocus
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                      onKeyDown={(e) => e.key === 'Enter' && void generatePairCode()}
                      placeholder="5511999990000"
                      inputMode="numeric"
                      autoComplete="off"
                      invalid={!!pairError}
                    />
                  )}
                </Field>
                <Button variant="primary" onClick={() => void generatePairCode()} loading={pairBusy} disabled={!phone.trim()}>
                  {pairCode ? 'Gerar novo código' : 'Gerar código'}
                </Button>
                {pairCode ? (
                  <div className="rounded-card bg-surface-2 p-3 text-center ring-1 ring-line">
                    <p className="text-xs text-muted">Código de pareamento</p>
                    <div className="mt-1 flex items-center justify-center gap-1">
                      <p className="font-mono text-2xl font-semibold tracking-[0.18em]">{pairCode}</p>
                      <CopyButton value={pairCode} label="Copiar código" />
                    </div>
                  </div>
                ) : null}
              </div>
              <Steps items={STEPS_PHONE} />
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}

export function ConnectDialog({ instanceId, onClose }: { instanceId: string | null; onClose: () => void }) {
  if (!instanceId) return null;
  return <Body key={instanceId} instanceId={instanceId} onClose={onClose} />;
}
