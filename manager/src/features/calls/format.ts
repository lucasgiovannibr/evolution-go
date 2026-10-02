import type { CallDirection, CallOutcome, CallPhase } from '@/api/calls';
import type { Tone } from '@/components/ui/badge';
import { formatPhone, numberFromJid } from '@/lib/format';

export const PHASE_LABEL: Record<CallPhase, string> = {
  calling: 'Chamando',
  ringing: 'Tocando',
  connecting: 'Conectando',
  active: 'Em andamento',
  ended: 'Encerrada',
  other: 'Aguardando',
};

export const PHASE_TONE: Record<CallPhase, Tone> = {
  calling: 'info',
  ringing: 'warn',
  connecting: 'info',
  active: 'ok',
  ended: 'neutral',
  other: 'neutral',
};

export const OUTCOME_LABEL: Record<CallOutcome, string> = {
  answered: 'Atendida',
  missed: 'Perdida',
  rejected: 'Recusada',
  cancelled: 'Cancelada',
  unanswered: 'Sem resposta',
  busy: 'Ocupado',
  failed: 'Falhou',
};

export const OUTCOME_TONE: Record<CallOutcome, Tone> = {
  answered: 'ok',
  missed: 'warn',
  rejected: 'neutral',
  cancelled: 'neutral',
  unanswered: 'neutral',
  busy: 'warn',
  failed: 'danger',
};

export const DIRECTION_LABEL: Record<CallDirection, string> = { incoming: 'Recebida', outgoing: 'Discada' };

/** Why a call ended, in words (the technical reason is kept in the title of the cell). */
export function reasonLabel(reason: string): string {
  if (reason.startsWith('server:')) return 'Erro do servidor do WhatsApp';
  switch (reason) {
    case 'peer_hangup':
      return 'O outro lado desligou';
    case 'hangup':
      return 'Desligada por este lado';
    case 'rejected':
      return 'Recusada';
    case 'rejected_busy':
      return 'Instância no limite de chamadas';
    case 'ring_timeout':
      return 'Ninguém atendeu a tempo';
    case 'stream_closed':
      return 'O áudio foi desconectado';
    case 'media_stalled':
      return 'Ficou sem áudio';
    case 'max_duration':
      return 'Passou da duração máxima';
    case 'silence_timeout':
      return 'Silêncio prolongado';
    case 'instance_stopped':
      return 'A instância foi parada';
    default:
      return reason || '—';
  }
}

/** '03:07', or '1:02:03' from an hour on. */
export function formatClock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${String(m).padStart(2, '0')}:${ss}`;
}

export function elapsedSeconds(startedAt: string, now = Date.now()): number {
  const t = new Date(startedAt).getTime();
  return Number.isNaN(t) ? 0 : Math.max(0, (now - t) / 1000);
}

/** The other side of a call for people: the phone when known, otherwise what the JID allows. */
export function peerLabel(peer: string, phone?: string): string {
  if (phone) return formatPhone(phone);
  if (peer.endsWith('@s.whatsapp.net')) return formatPhone(numberFromJid(peer));
  if (peer.endsWith('@lid')) return `Contato oculto (…${numberFromJid(peer).slice(-4)})`;
  return peer || '—';
}

/** The WebSocket address of a stream: the API's own address with http(s) turned into ws(s) and the ticket path after it. */
export function streamWsUrl(apiUrl: string, path: string): string {
  const url = new URL(apiUrl);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  const [pathname, query = ''] = path.split('?');
  url.pathname = `${url.pathname.replace(/\/+$/, '')}/${(pathname ?? '').replace(/^\/+/, '')}`;
  url.search = query ? `?${query}` : '';
  return url.toString();
}

/** What to tell the person when the browser refuses the microphone. */
export function micErrorMessage(err: unknown): string {
  if (typeof window !== 'undefined' && !window.isSecureContext) {
    return 'O navegador só libera o microfone em páginas seguras: abra o painel por https ou por localhost.';
  }
  const name = err instanceof DOMException ? err.name : '';
  if (name === 'NotAllowedError' || name === 'SecurityError') return 'O acesso ao microfone foi negado. Libere-o nas permissões do site e tente de novo.';
  if (name === 'NotFoundError' || name === 'OverconstrainedError') return 'Nenhum microfone foi encontrado neste aparelho.';
  if (name === 'NotReadableError') return 'O microfone está em uso por outro programa.';
  return err instanceof Error && err.message ? err.message : 'Não foi possível usar o microfone.';
}

/** A level (rms, 0 to 1) as a 0 to 100 bar that moves for speech instead of staying at the bottom. */
export function levelPercent(rmsValue: number): number {
  if (rmsValue <= 0) return 0;
  const db = 20 * Math.log10(rmsValue); // 0 dBFS at full scale
  return Math.max(0, Math.min(100, Math.round(((db + 60) / 60) * 100)));
}
