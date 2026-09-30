import { api } from '@/lib/http';
import { toInstance, type BehaviorSettings, type Instance, type ProducerState, type QrInfo, type RawInstance } from './types';

interface Envelope<T> {
  message?: string;
  data: T;
}

export async function listInstances(): Promise<Instance[]> {
  const res = await api<Envelope<RawInstance[] | null>>('/instance/all', { query: { t: Date.now() } });
  return (res.data ?? []).map(toInstance);
}

export async function getInstance(id: string): Promise<Instance> {
  const res = await api<Envelope<RawInstance>>(`/instance/info/${encodeURIComponent(id)}`);
  return toInstance(res.data);
}

export interface CreateInstanceInput {
  name: string;
  token: string;
  proxy?: { host: string; port: string; username: string; password: string };
}

export async function createInstance(input: CreateInstanceInput): Promise<Instance> {
  const res = await api<Envelope<RawInstance>>('/instance/create', { method: 'POST', body: input });
  return toInstance(res.data);
}

export async function deleteInstance(id: string): Promise<void> {
  await api(`/instance/delete/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export interface ConnectInput {
  webhookUrl?: string;
  subscribe?: string[];
  rabbitmqEnable?: ProducerState | string;
  websocketEnable?: ProducerState | string;
  natsEnable?: ProducerState | string;
  phone?: string;
}

/**
 * Applies webhook/events/producers and starts the instance if it is not running.
 * Empty fields are left as they are on the server ("disabled" clears the webhook).
 */
export async function connectInstance(token: string, input: ConnectInput = {}): Promise<void> {
  await api('/instance/connect', {
    method: 'POST',
    apikey: token,
    body: {
      webhookUrl: input.webhookUrl ?? '',
      subscribe: input.subscribe ?? [],
      rabbitmqEnable: input.rabbitmqEnable ?? '',
      websocketEnable: input.websocketEnable ?? '',
      natsEnable: input.natsEnable ?? '',
      phone: input.phone ?? '',
    },
  });
}

export interface IntegrationsInput {
  webhookUrl: string;
  subscribe: string[];
  rabbitmqEnable: ProducerState;
  websocketEnable: ProducerState;
  natsEnable: ProducerState;
}

/**
 * Stores webhook, events and producer switches without starting the instance; a running one
 * picks them up immediately. Same field semantics as connect (empty keeps, "disabled" clears the webhook).
 */
export async function saveIntegrations(id: string, input: IntegrationsInput): Promise<void> {
  await api(`/instance/${encodeURIComponent(id)}/integrations`, { method: 'PUT', body: input });
}

export async function getQr(token: string): Promise<QrInfo> {
  const res = await api<Envelope<Partial<QrInfo>>>('/instance/qr', { apikey: token });
  const d = res.data ?? {};
  return {
    qrcode: d.qrcode ?? '',
    code: d.code ?? '',
    passkeyStage: d.passkeyStage || undefined,
    passkeyOpenUrl: d.passkeyOpenUrl || undefined,
    passkeyCode: d.passkeyCode || undefined,
  };
}

export async function pairPhone(token: string, phone: string, subscribe: string[] = []): Promise<string> {
  const res = await api<Envelope<{ PairingCode?: string }>>('/instance/pair', {
    method: 'POST',
    apikey: token,
    body: { phone, subscribe },
  });
  return res.data?.PairingCode ?? '';
}

/** Closes the connection but keeps the linked session, so it can reconnect without a QR. */
export async function disconnectInstance(token: string): Promise<void> {
  await api('/instance/disconnect', { method: 'POST', apikey: token });
}

/** Unlinks the device from WhatsApp; a new QR Code is needed afterwards. */
export async function logoutInstance(token: string): Promise<void> {
  await api('/instance/logout', { method: 'DELETE', apikey: token });
}

export async function updateBehavior(id: string, settings: BehaviorSettings): Promise<void> {
  await api(`/instance/${encodeURIComponent(id)}/advanced-settings`, { method: 'PUT', body: settings });
}
