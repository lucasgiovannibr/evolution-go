import { useEffect, useState, type FormEvent } from 'react';
import { ChevronDown, Dices, Eye, EyeOff, Plus } from 'lucide-react';
import type { Instance } from '@/api/types';
import { errMsg, useCreateInstance } from '@/hooks/use-instances';
import { randomToken } from '@/lib/format';
import { cn } from '@/lib/cn';
import { Button } from '@/components/ui/button';
import { CopyButton } from '@/components/ui/copy-button';
import { Dialog } from '@/components/ui/dialog';
import { Field, Input } from '@/components/ui/form';
import { Alert } from '@/components/ui/feedback';

const NAME_RE = /^[A-Za-z0-9_-]+$/;

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (i: Instance) => void;
}

export function CreateInstanceDialog({ open, onClose, onCreated }: Props) {
  const create = useCreateInstance();
  const [name, setName] = useState('');
  const [token, setToken] = useState('');
  const [showToken, setShowToken] = useState(false);
  const [useProxy, setUseProxy] = useState(false);
  const [proxy, setProxy] = useState({ host: '', port: '', username: '', password: '' });
  const [touched, setTouched] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setName('');
    setToken(randomToken());
    setShowToken(false);
    setUseProxy(false);
    setProxy({ host: '', port: '', username: '', password: '' });
    setTouched(false);
    setError('');
  }, [open]);

  const nameError = !name.trim() ? 'Informe um nome.' : !NAME_RE.test(name) ? 'Use apenas letras, números, hífen (-) e underscore (_).' : null;
  const proxyMissing = useProxy && Object.values(proxy).some((v) => !v.trim());

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setTouched(true);
    setError('');
    if (nameError) return;
    if (!token.trim()) return setError('O token não pode ficar vazio.');
    if (proxyMissing) return setError('Preencha todos os campos do proxy ou desative-o.');
    try {
      const created = await create.mutateAsync({
        name: name.trim(),
        token: token.trim(),
        proxy: useProxy ? { host: proxy.host.trim(), port: proxy.port.trim(), username: proxy.username.trim(), password: proxy.password } : undefined,
      });
      onCreated(created);
    } catch (err) {
      setError(errMsg(err, 'Não foi possível criar a instância.'));
    }
  };

  const setP = (k: keyof typeof proxy) => (e: React.ChangeEvent<HTMLInputElement>) => setProxy((p) => ({ ...p, [k]: e.target.value }));

  return (
    <Dialog
      open={open}
      onClose={onClose}
      dismissible={!create.isPending}
      icon={<Plus />}
      title="Nova instância"
      description="Cada instância representa um número de WhatsApp."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={create.isPending}>
            Cancelar
          </Button>
          <Button type="submit" form="create-instance-form" variant="primary" loading={create.isPending}>
            Criar instância
          </Button>
        </>
      }
    >
      <form id="create-instance-form" onSubmit={submit} className="space-y-4 pb-3" noValidate>
        {error ? <Alert tone="danger">{error}</Alert> : null}

        <Field label="Nome" error={touched ? nameError : null} hint="Identifica a instância. Não pode ser alterado depois.">
          {(id) => (
            <Input
              id={id}
              data-autofocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="minha-instancia"
              autoComplete="off"
              spellCheck={false}
              invalid={touched && !!nameError}
            />
          )}
        </Field>

        <Field label="Token de acesso" hint="Chave usada por esta instância nas chamadas da API. Gerado automaticamente.">
          {(id) => (
            <Input
              id={id}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              type={showToken ? 'text' : 'password'}
              className="[&_input]:pr-24 [&_input]:font-mono [&_input]:text-xs"
              autoComplete="off"
              spellCheck={false}
              trailing={
                <span className="flex items-center">
                  <Button variant="ghost" size="icon-sm" onClick={() => setShowToken((v) => !v)} aria-label={showToken ? 'Ocultar token' : 'Mostrar token'}>
                    {showToken ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
                  </Button>
                  <Button variant="ghost" size="icon-sm" onClick={() => setToken(randomToken())} aria-label="Gerar novo token" title="Gerar novo token">
                    <Dices className="size-3.5" />
                  </Button>
                  <CopyButton value={token} label="Copiar token" />
                </span>
              }
            />
          )}
        </Field>

        <div className="rounded-card border border-line">
          <button
            type="button"
            onClick={() => setUseProxy((v) => !v)}
            aria-expanded={useProxy}
            className="flex w-full items-center gap-2 rounded-card px-3 py-2.5 text-left text-[13px] font-medium"
          >
            Usar proxy
            <span className="font-normal text-subtle">opcional</span>
            <ChevronDown className={cn('ml-auto size-4 text-subtle transition-transform', useProxy && 'rotate-180')} />
          </button>
          {useProxy ? (
            <div className="grid gap-3 border-t border-line p-3 sm:grid-cols-2">
              <Field label="Host">{(id) => <Input id={id} value={proxy.host} onChange={setP('host')} placeholder="proxy.exemplo.com" autoComplete="off" />}</Field>
              <Field label="Porta">{(id) => <Input id={id} value={proxy.port} onChange={setP('port')} placeholder="8080" inputMode="numeric" autoComplete="off" />}</Field>
              <Field label="Usuário">{(id) => <Input id={id} value={proxy.username} onChange={setP('username')} autoComplete="off" />}</Field>
              <Field label="Senha">{(id) => <Input id={id} type="password" value={proxy.password} onChange={setP('password')} autoComplete="new-password" />}</Field>
            </div>
          ) : null}
        </div>
      </form>
    </Dialog>
  );
}
