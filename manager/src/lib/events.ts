export type EventDef = { id: string; label: string; hint: string };
export type EventGroup = { id: string; label: string; events: EventDef[] };

/** Mirrors pkg/internal/event_types on the server. Keep in sync when events are added. */
export const EVENT_GROUPS: EventGroup[] = [
  {
    id: 'messages',
    label: 'Mensagens',
    events: [
      { id: 'MESSAGE', label: 'Mensagem recebida', hint: 'Textos, mídias e demais mensagens que chegam' },
      { id: 'SEND_MESSAGE', label: 'Mensagem enviada', hint: 'Mensagens enviadas por esta instância' },
      { id: 'READ_RECEIPT', label: 'Confirmação de leitura', hint: 'Entregue, lida e reproduzida' },
      { id: 'BUTTON_CLICK', label: 'Clique em botão', hint: 'Respostas a botões e listas' },
    ],
  },
  {
    id: 'presence',
    label: 'Presença e chats',
    events: [
      { id: 'PRESENCE', label: 'Presença', hint: 'Online e visto por último' },
      { id: 'CHAT_PRESENCE', label: 'Atividade no chat', hint: 'Digitando, gravando, fixar, arquivar, silenciar' },
    ],
  },
  {
    id: 'session',
    label: 'Sessão',
    events: [
      { id: 'CONNECTION', label: 'Conexão', hint: 'Conectou, desconectou, banimento, falhas' },
      { id: 'QRCODE', label: 'QR Code e pareamento', hint: 'Novo QR, pareamento e passkey' },
      { id: 'CALL', label: 'Chamadas', hint: 'Chamadas recebidas, aceitas e encerradas' },
      { id: 'HISTORY_SYNC', label: 'Sincronização de histórico', hint: 'Histórico enviado pelo celular' },
    ],
  },
  {
    id: 'directory',
    label: 'Contatos e grupos',
    events: [
      { id: 'CONTACT', label: 'Contatos', hint: 'Nome, bloqueios e privacidade' },
      { id: 'GROUP', label: 'Grupos', hint: 'Alterações e entrada em grupos' },
      { id: 'NEWSLETTER', label: 'Canais', hint: 'Entrada, saída e atualizações' },
      { id: 'LABEL', label: 'Etiquetas', hint: 'Etiquetas de conversas e mensagens' },
      { id: 'PICTURE', label: 'Foto de perfil', hint: 'Foto de contatos e grupos alterada' },
      { id: 'USER_ABOUT', label: 'Recado', hint: 'Recado (about) de contatos alterado' },
    ],
  },
];

export const ALL_EVENT_IDS: string[] = EVENT_GROUPS.flatMap((g) => g.events.map((e) => e.id));

/** Canonical order (catalog order) so drafts and server data compare equal regardless of how they were built. */
export function normalizeEvents(events: string[]): string[] {
  return ALL_EVENT_IDS.filter((id) => events.includes(id));
}

/** The server stores ALL expanded to the concrete list, so detect it by coverage. */
export function coversAllEvents(events: string[]): boolean {
  return ALL_EVENT_IDS.every((id) => events.includes(id));
}

export function parseEvents(raw: string | undefined | null): string[] {
  if (!raw) return [];
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

/** Selection -> `subscribe` payload. ALL is sent as ["ALL"], which the server expands. */
export function toSubscribe(selected: string[], all: boolean): string[] {
  if (all) return ['ALL'];
  return normalizeEvents(selected);
}
