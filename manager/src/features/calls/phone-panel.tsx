import { Mic, MicOff, PhoneOff, Video, VideoOff, Volume2, X } from 'lucide-react';
import { useNow } from '@/hooks/use-now';
import { cn } from '@/lib/cn';
import { Alert, Spinner } from '@/components/ui/feedback';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { formatClock, levelPercent, peerVideoLabel } from './format';
import type { PhoneSession, PhoneVideo } from './use-phone';

function LevelBar({ label, icon, level, active }: { label: string; icon: React.ReactNode; level: number; active?: boolean }) {
  const pct = levelPercent(level);
  return (
    <div className="flex items-center gap-2.5">
      <span className="flex w-24 shrink-0 items-center gap-1.5 text-xs text-muted [&_svg]:size-3.5">
        {icon}
        {label}
      </span>
      <div
        className="h-2 flex-1 overflow-hidden rounded-full bg-surface-3"
        role="meter"
        aria-label={`Nível: ${label}`}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
      >
        <div className={cn('h-full rounded-full transition-[width] duration-100', active ? 'bg-ok' : 'bg-brand')} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

/** The other side's picture (with ours small in the corner), and a note on what its camera is doing. */
function VideoStage({ video, onCanvas }: { video: PhoneVideo; onCanvas: (el: HTMLCanvasElement | null) => void }) {
  const peer = peerVideoLabel(video.peer?.state);
  return (
    <div className="space-y-2">
      <div className="relative flex min-h-44 items-center justify-center overflow-hidden rounded-card bg-black ring-1 ring-line">
        {video.camera ? (
          <video
            className="absolute right-2 bottom-2 z-10 h-28 -scale-x-100 rounded-md object-cover shadow-lg ring-1 ring-white/30"
            aria-label="Sua câmera"
            autoPlay
            muted
            playsInline
            ref={(el) => {
              if (el && el.srcObject !== video.preview) el.srcObject = video.preview;
            }}
          />
        ) : null}
        {/* kept in the page even while hidden: the receiver draws on it as soon as pictures arrive */}
        <canvas ref={onCanvas} className={cn('block max-h-[60vh] w-auto max-w-full', !video.active && 'hidden')} />
        {!video.active ? (
          <p className="flex items-center gap-2 px-4 text-center text-xs text-white/70 [&_svg]:size-4">
            <Video />
            {video.peer && (video.peer.state === 'disabled' || video.peer.state === 'stopped') ? 'O outro lado desligou a câmera' : 'Aguardando o vídeo do outro lado'}
          </p>
        ) : null}
      </div>
      <p className="flex flex-wrap items-center gap-2 text-xs text-muted">
        {video.active ? <Badge tone="ok" dot>Recebendo vídeo · {video.width}×{video.height}</Badge> : null}
        {video.camera ? (
          <Badge tone={video.sent > 0 ? 'brand' : 'warn'} icon={<Video />}>
            {video.sent > 0 ? `Sua câmera está no ar · ${video.sent} quadros enviados` : 'Câmera ligada, ainda sem enviar (esperando a chamada aceitar vídeo)'}
          </Badge>
        ) : null}
        {peer ? <span>{peer}</span> : null}
      </p>
    </div>
  );
}

interface PhonePanelProps {
  session: PhoneSession;
  onHangup: () => void;
  onToggleMute: () => void;
  onDismiss: () => void;
  onCanvas: (el: HTMLCanvasElement | null) => void;
  onToggleCamera: () => void;
  onAcceptVideo: () => void;
  /** Requests for video are accepted the moment they arrive. */
  autoAccept: boolean;
  onAutoAccept: (on: boolean) => void;
}

/** The call that is going on through the audio of this page: timer, levels, mute and hang up. */
export function PhonePanel({ session, onHangup, onToggleMute, onDismiss, onCanvas, onToggleCamera, onAcceptVideo, autoAccept, onAutoAccept }: PhonePanelProps) {
  const now = useNow();
  const over = session.state === 'ended' || session.state === 'error';
  const live = session.state === 'live';

  return (
    <Card className={cn(live && 'border-ok/40')}>
      <CardHeader
        title="Telefone do navegador"
        description="O microfone e o alto-falante desta página são o áudio da chamada."
        actions={
          live ? (
            <Badge tone="ok" dot pulse>
              No ar · {formatClock(session.liveAt ? (now - session.liveAt) / 1000 : 0)}
            </Badge>
          ) : over ? (
            <Badge tone={session.state === 'error' ? 'danger' : 'neutral'}>{session.state === 'error' ? 'Falhou' : 'Encerrado'}</Badge>
          ) : (
            <Badge tone="info" icon={<Spinner className="size-3 text-info" />}>
              {session.state === 'connecting' ? 'Conectando o áudio' : 'Preparando o microfone'}
            </Badge>
          )
        }
      />
      <CardBody className="space-y-4">
        {over ? (
          <Alert
            tone={session.state === 'error' ? 'danger' : 'info'}
            title={session.state === 'error' ? 'Não foi possível usar o áudio' : 'O áudio da chamada terminou'}
            action={
              <Button size="sm" variant="secondary" onClick={onDismiss}>
                <X className="size-3.5" />
                Fechar
              </Button>
            }
          >
            {session.detail}
          </Alert>
        ) : (
          <>
            {session.video.hasVideo && !session.video.supported ? (
              <Alert tone="warn" title="Este navegador não mostra vídeo">
                A chamada tem vídeo, mas este navegador não decodifica H.264 (WebCodecs). O áudio funciona; para ver o vídeo, use um navegador recente do Chrome, Edge ou Safari.
              </Alert>
            ) : null}
            {session.video.peer?.state === 'upgrade_request' && live ? (
              <Alert
                tone="info"
                title="O outro lado quer passar a chamada para vídeo"
                action={
                  <Button size="sm" variant="primary" onClick={onAcceptVideo}>
                    <Video className="size-3.5" />
                    Aceitar vídeo
                  </Button>
                }
              >
                Aceite logo: o outro lado desiste do pedido em poucos segundos.{' '}
                {session.video.canSend ? 'Ao aceitar, a sua câmera também é ligada, se houver uma.' : 'Este navegador só mostra o vídeo dele: não envia a sua câmera.'}
              </Alert>
            ) : null}
            {session.video.cameraError ? <Alert tone="warn" title="Problema com o vídeo">{session.video.cameraError}</Alert> : null}
            {session.video.supported && (session.video.hasVideo || session.video.camera) ? <VideoStage video={session.video} onCanvas={onCanvas} /> : null}
            <div className="space-y-2.5">
              <LevelBar label={session.muted ? 'Você (mudo)' : 'Você'} icon={session.muted ? <MicOff /> : <Mic />} level={session.muted ? 0 : session.mic} />
              <LevelBar label="Outro lado" icon={<Volume2 />} level={session.peer} active={session.peerSpeaking} />
              <div className="flex h-5 items-center gap-2 pl-[6.5rem]">{session.peerSpeaking ? <Badge tone="ok" dot pulse>Falando</Badge> : null}</div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button onClick={onToggleMute} disabled={!live} variant={session.muted ? 'primary' : 'secondary'}>
                {session.muted ? <MicOff className="size-4" /> : <Mic className="size-4" />}
                {session.muted ? 'Ativar microfone' : 'Silenciar'}
              </Button>
              {session.video.supported && session.video.canSend ? (
                <Button onClick={onToggleCamera} disabled={!live} variant={session.video.camera ? 'primary' : 'secondary'}>
                  {session.video.camera ? <VideoOff className="size-4" /> : <Video className="size-4" />}
                  {session.video.camera ? 'Desligar câmera' : session.video.hasVideo ? 'Ligar câmera' : 'Iniciar vídeo'}
                </Button>
              ) : null}
              <Button variant="danger" onClick={onHangup}>
                <PhoneOff className="size-4" />
                Desligar
              </Button>
            </div>
            {session.video.supported ? (
              <label className="flex items-center gap-2 text-[13px] text-muted">
                <Switch checked={autoAccept} onChange={onAutoAccept} label="Aceitar pedidos de vídeo na hora" />
                Aceitar pedidos de vídeo na hora
                <span className="text-xs text-subtle">(o outro lado desiste em poucos segundos; sem câmera aqui, você só vê o vídeo dele)</span>
              </label>
            ) : null}
            <p className="text-xs text-muted">
              Use fones de ouvido: sem eles, o alto-falante volta para o microfone e a outra pessoa se ouve de volta. Se você sair desta página, o
              áudio acaba e a chamada cai em alguns segundos.
            </p>
          </>
        )}
      </CardBody>
    </Card>
  );
}
