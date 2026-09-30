import { useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle, ArrowRight, Eye, EyeOff, Globe, KeyRound, LogOut, Network, Plug, Radio, Cable, Trash2, Unplug, Webhook } from 'lucide-react';
import type { Instance, ProducerState } from '@/api/types';
import { formatDateTime, formatPhone, maskToken, timeAgo } from '@/lib/format';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { CodeBlock } from '@/components/ui/code-block';
import { CopyButton } from '@/components/ui/copy-button';
import { useInstanceActions } from '@/features/instances/actions';
import { StatusBadge } from '@/features/instances/instance-parts';
import { useAuth } from '@/stores/auth';
import { useInstanceContext } from './instance-page';

function Row({ label, children, mono }: { label: string; children: ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-center gap-4 py-2.5 first:pt-0 last:pb-0">
      <dt className="w-32 shrink-0 text-xs text-muted">{label}</dt>
      <dd className={`min-w-0 flex-1 text-[13px] ${mono ? 'font-mono text-xs' : ''}`}>{children}</dd>
    </div>
  );
}

function producerBadge(state: ProducerState) {
  if (state === 'enabled') return <Badge tone="ok">Ativo</Badge>;
  if (state === 'disabled') return <Badge>Inativo</Badge>;
  return <Badge>Padrão do servidor</Badge>;
}

function IntegrationRow({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <li className="flex items-center gap-3 py-2 first:pt-0 last:pb-0">
      <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-surface-2 text-muted [&_svg]:size-3.5">{icon}</span>
      <span className="min-w-0 flex-1 text-[13px]">{label}</span>
      {children}
    </li>
  );
}

export function TabGeneral() {
  const { instance } = useInstanceContext();
  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-3">
      <div className="space-y-4 lg:col-span-2">
        <ConnectionCard instance={instance} />
        <CredentialsCard instance={instance} />
      </div>
      <div className="space-y-4">
        <IntegrationsCard instance={instance} />
        <DangerCard instance={instance} />
      </div>
    </div>
  );
}

function ConnectionCard({ instance }: { instance: Instance }) {
  return (
    <Card>
      <CardHeader title="Conexão" description="Estado atual da sessão com o WhatsApp." icon={<Plug />} />
      <CardBody>
        <dl className="divide-y divide-line">
          <Row label="Status">
            <StatusBadge connected={instance.connected} />
          </Row>
          <Row label="Número">{instance.number ? formatPhone(instance.number) : <span className="text-subtle">Nenhum número vinculado</span>}</Row>
          {!instance.connected && instance.disconnectReason ? (
            <Row label="Motivo da queda">
              <span className="text-warn">{instance.disconnectReason}</span>
            </Row>
          ) : null}
          <Row label="Criada em">
            {formatDateTime(instance.createdAt)} <span className="text-subtle">· {timeAgo(instance.createdAt)}</span>
          </Row>
          {instance.clientName || instance.osName ? (
            <Row label="Identificação">{[instance.clientName, instance.osName].filter(Boolean).join(' · ')}</Row>
          ) : null}
        </dl>
      </CardBody>
    </Card>
  );
}

function CredentialsCard({ instance }: { instance: Instance }) {
  const [reveal, setReveal] = useState(false);
  const apiUrl = useAuth((s) => s.apiUrl).replace(/\/$/, '');
  const shown = reveal ? instance.token : maskToken(instance.token);
  const snippet = (token: string) =>
    `curl -X POST '${apiUrl}/send/text' \\\n  -H 'apikey: ${token}' \\\n  -H 'Content-Type: application/json' \\\n  -d '{"number": "5511999990000", "text": "Olá!"}'`;

  return (
    <Card>
      <CardHeader title="Credenciais" description="Use o token desta instância para chamar a API em nome dela." icon={<KeyRound />} />
      <CardBody className="space-y-4">
        <dl className="divide-y divide-line">
          <Row label="ID da instância" mono>
            <span className="flex items-center gap-1">
              <span className="truncate">{instance.id}</span>
              <CopyButton value={instance.id} label="Copiar ID" />
            </span>
          </Row>
          <Row label="Token" mono>
            <span className="flex items-center gap-1">
              <span className="truncate">{shown}</span>
              <Button variant="ghost" size="icon-sm" onClick={() => setReveal((v) => !v)} aria-label={reveal ? 'Ocultar token' : 'Mostrar token'}>
                {reveal ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
              </Button>
              <CopyButton value={instance.token} label="Copiar token" />
            </span>
          </Row>
        </dl>
        <div>
          <p className="mb-1.5 text-xs font-medium">Exemplo: enviar uma mensagem</p>
          <CodeBlock code={snippet(shown)} copyValue={snippet(instance.token)} language="bash" maxHeight="10rem" />
        </div>
      </CardBody>
    </Card>
  );
}

function IntegrationsCard({ instance }: { instance: Instance }) {
  const base = `/manager/instances/${instance.id}`;
  return (
    <Card>
      <CardHeader
        title="Integrações"
        description="Para onde os eventos são enviados."
        icon={<Webhook />}
        actions={
          <Link to={`${base}/webhook`} className="inline-flex items-center gap-1 text-xs font-medium text-brand-text hover:underline">
            Editar <ArrowRight className="size-3" />
          </Link>
        }
      />
      <CardBody>
        <ul className="divide-y divide-line">
          <IntegrationRow icon={<Webhook />} label="Webhook">
            {instance.webhook ? (
              <span className="max-w-[10rem] truncate font-mono text-xs text-muted" title={instance.webhook}>
                {instance.webhook.replace(/^https?:\/\//, '')}
              </span>
            ) : (
              <Badge>Não configurado</Badge>
            )}
          </IntegrationRow>
          <IntegrationRow icon={<Globe />} label="Eventos">
            <Badge tone={instance.events.length ? 'brand' : 'neutral'}>{instance.allEvents ? 'Todos' : instance.events.length ? `${instance.events.length} selecionados` : 'Padrão'}</Badge>
          </IntegrationRow>
          <IntegrationRow icon={<Network />} label="RabbitMQ">
            {producerBadge(instance.producers.rabbitmq)}
          </IntegrationRow>
          <IntegrationRow icon={<Radio />} label="WebSocket">
            {producerBadge(instance.producers.websocket)}
          </IntegrationRow>
          <IntegrationRow icon={<Cable />} label="NATS">
            {producerBadge(instance.producers.nats)}
          </IntegrationRow>
          {instance.proxy ? (
            <IntegrationRow icon={<Globe />} label="Proxy">
              <span className="font-mono text-xs text-muted">
                {instance.proxy.host}:{instance.proxy.port}
              </span>
            </IntegrationRow>
          ) : null}
        </ul>
      </CardBody>
    </Card>
  );
}

function DangerCard({ instance }: { instance: Instance }) {
  const actions = useInstanceActions();
  return (
    <Card className="border-danger/30">
      <CardHeader title="Zona de perigo" icon={<AlertTriangle className="text-danger" />} className="border-danger/20" />
      <CardBody className="space-y-3">
        {instance.connected ? (
          <>
            <div className="flex items-center gap-3">
              <div className="min-w-0 flex-1">
                <p className="text-[13px] font-medium">Desconectar</p>
                <p className="text-xs text-muted">Encerra a conexão e mantém o número vinculado.</p>
              </div>
              <Button size="sm" onClick={() => actions.disconnect(instance)}>
                <Unplug className="size-3.5" />
                Desconectar
              </Button>
            </div>
            <div className="flex items-center gap-3 border-t border-line pt-3">
              <div className="min-w-0 flex-1">
                <p className="text-[13px] font-medium">Desvincular do WhatsApp</p>
                <p className="text-xs text-muted">Sai da conta; exige novo QR Code.</p>
              </div>
              <Button size="sm" variant="danger-soft" onClick={() => actions.logout(instance)}>
                <LogOut className="size-3.5" />
                Desvincular
              </Button>
            </div>
          </>
        ) : null}
        <div className={`flex items-center gap-3 ${instance.connected ? 'border-t border-line pt-3' : ''}`}>
          <div className="min-w-0 flex-1">
            <p className="text-[13px] font-medium">Excluir instância</p>
            <p className="text-xs text-muted">Remove a instância e todas as configurações.</p>
          </div>
          <Button size="sm" variant="danger" onClick={() => actions.remove(instance)}>
            <Trash2 className="size-3.5" />
            Excluir
          </Button>
        </div>
      </CardBody>
    </Card>
  );
}
