# API de Chamadas

Documentação dos endpoints de chamadas WhatsApp: rejeitar chamadas recebidas (sempre disponível) e, com o **motor de chamadas** ligado na instância, atender, discar, controlar o vídeo e trocar áudio e vídeo por um WebSocket.

> ⚠️ **Experimental.** O motor de chamadas usa a biblioteca [`purpshell/meowcaller`](https://github.com/purpshell/meowcaller) (fixada em um commit), que implementa a sinalização e a mídia de VoIP do WhatsApp por conta própria. Foi validado ao vivo com um número real e um iPhone (áudio e vídeo, recebidas e enviadas), mas o WhatsApp pode mudar o protocolo sem aviso. Não há chamadas em grupo.

## 📋 Índice

- [Dois modos](#dois-modos)
- [Ligar o motor de chamadas](#ligar-o-motor-de-chamadas)
- [Resumo dos endpoints](#resumo-dos-endpoints)
- [Receber uma chamada](#receber-uma-chamada)
- [Fazer uma chamada](#fazer-uma-chamada)
- [O stream (WebSocket)](#o-stream-websocket)
- [Vídeo](#vídeo)
- [Eventos](#eventos)
- [Limites e configuração](#limites-e-configuração)
- [Erros](#erros)
- [Rejeitar Chamada](#rejeitar-chamada)
- [Teste ao vivo](#teste-ao-vivo)

---

## Dois modos

| Modo | Como ligar | O que faz |
|------|-----------|-----------|
| **Só rejeitar** (padrão) | nada | Recebe o evento `CallOffer` e pode rejeitar com `POST /call/reject`. Não atende nem disca. |
| **Motor de chamadas** | `callsEnabled` na instância | Atende, disca, desliga, controla o vídeo e leva o áudio e o vídeo pelo stream. |

O motor é **opt-in por instância**. Com ele ligado, **toda chamada recebida é pré-aceita** automaticamente pela biblioteca (o aparelho de quem liga passa a mostrar a chamada como em preparação). Por isso ele não vem ligado.

---

## Ligar o motor de chamadas

```bash
curl -X PUT "http://localhost:4000/instance/ID-DA-INSTANCIA/advanced-settings" \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-GLOBAL-API-KEY" \
  -d '{"callsEnabled": true}'
```

A mudança **só vale na próxima conexão** da instância: reinicie o servidor ou desconecte e conecte de novo. Depois confira com:

```bash
curl http://localhost:4000/call/active -H "apikey: TOKEN-DA-INSTANCIA"
```

```json
{ "enabled": true, "state": "active", "calls": [] }
```

| `state` | Significado |
|---------|-------------|
| `active` | Motor funcionando |
| `hook_failed` | O motor não conseguiu se acoplar ao whatsmeow; as chamadas ficam sem mídia (`error` traz o motivo) |
| `blocked_by_proxy` | A instância usa proxy. A mídia da chamada sai por UDP direto e **ignoraria o proxy** (vazaria o IP), então o motor fica desligado nessas instâncias |

`enabled: false` sem `state` significa que o cliente em execução não tem motor: as chamadas estão desligadas ou a instância ainda não reconectou depois de ligá-las.

`GET /instance/{instanceId}/runtime` e `GET /instance/runtimes` mostram `runtime.calls` (`state`, `activeCalls`) e avisos: `calls_enabled_pending_reconnect`, `calls_hook_failed` e `calls_blocked_by_proxy`.

---

## Resumo dos endpoints

Todos usam o **token da instância** no header `apikey`.

| Endpoint | Para quê |
|----------|----------|
| `GET /call/active` | Estado do motor e as chamadas que a instância acompanha |
| `GET /call/{callId}` | Uma chamada: fase, direção, vídeo e contadores do stream |
| `POST /call/answer` | Atende uma chamada que está tocando |
| `POST /call/dial` | Liga para alguém |
| `POST /call/hangup` | Encerra uma chamada em qualquer fase |
| `POST /call/stream-ticket` | Gera o bilhete para abrir o stream |
| `GET /call/stream/{callId}?ticket=…` | **WebSocket** com o áudio e o vídeo |
| `POST /call/video` | Controles de vídeo (`start`, `accept`, `stop`, `enable`, `disable`, `orientation`) |
| `POST /call/reject` | Rejeita uma chamada recebida (vale sem o motor) |

**Fases** (`phase`): `calling` (discando), `ringing` (tocando), `connecting`, `active`, `ended`, `other`. **Direção**: `incoming` ou `outgoing`.

Exemplo de `GET /call/{callId}`:

```json
{
  "callId": "0018F2C9B00F33ACD31A60AEA2486853",
  "direction": "incoming",
  "peer": "273117121392855@lid",
  "phase": "active",
  "startedAt": "2026-09-30T16:22:02Z",
  "video": true,
  "videoSending": true,
  "videoReceiving": true,
  "peerVideo": { "active": true, "upgrade": false, "orientation": 1, "state": "enabled", "stateCode": 1 },
  "stream": { "attached": true, "toClient": 620, "fromClient": 610, "videoToClient": 150 }
}
```

---

## Receber uma chamada

1. O webhook recebe `CallOffer` (assinatura `CALL`) com o `callId`. Ou consulte `GET /call/active` e procure `direction: incoming` e `phase: ringing`.
2. `POST /call/stream-ticket` com `{"callId": "...", "video": true}` (use `video: true` só se for usar o vídeo).
3. Abra o WebSocket do bilhete **antes de atender**: o stream fica ligado enquanto a chamada ainda toca e nenhum áudio se perde.
4. `POST /call/answer` com `{"callId": "..."}`.

```bash
curl -X POST http://localhost:4000/call/answer \
  -H "Content-Type: application/json" -H "apikey: TOKEN-DA-INSTANCIA" \
  -d '{"callId": "0018F2C9B00F33ACD31A60AEA2486853"}'
```

Uma chamada **atendida sem stream é desligada** depois de `CALL_STREAM_GRACE` segundos (motivo `stream_closed`). O mesmo vale se o stream de uma chamada em andamento fechar e não voltar nesse prazo.

---

## Fazer uma chamada

```bash
curl -X POST http://localhost:4000/call/dial \
  -H "Content-Type: application/json" -H "apikey: TOKEN-DA-INSTANCIA" \
  -d '{"number": "5511999990000", "video": true, "stream": true}'
```

```json
{
  "callId": "E2BDDB29A16F34D5CDCCA8101FD6F42E",
  "direction": "outgoing",
  "peer": "5511999990000@s.whatsapp.net",
  "phase": "calling",
  "video": true,
  "streamTicket": {
    "ticket": "…",
    "expiresInSeconds": 30,
    "path": "/call/stream/E2BDDB29A16F34D5CDCCA8101FD6F42E?ticket=…"
  }
}
```

- `video: true` faz uma **videochamada**.
- `stream: true` já devolve o bilhete (com vídeo se `video` for `true`), para ligar o WebSocket **antes** de o outro lado atender.
- Há um limite de chamadas por minuto e por instância (`CALL_DIAL_LIMIT`, padrão 6, contando as que falham): discar em massa é o que leva uma conta a ser sinalizada. Passou do limite, responde `429`.
- Instâncias com proxy não discam (ver `blocked_by_proxy`).

---

## O stream (WebSocket)

Abra `ws://host:4000` + o `path` do bilhete (`/call/stream/{callId}?ticket=…`).

- O **bilhete vale para um uso e expira em 30 s**. Ele evita pôr a chave de API na URL.
- Navegadores: a origem da página precisa estar em `CALL_STREAM_ORIGINS` (ou ser a do próprio servidor). Quem não manda `Origin` (um servidor, um script) é sempre aceito: o bilhete é que protege.
- O formato das mensagens segue o de Twilio Media Streams, JSON por mensagem.

**Servidor → cliente**

```json
{"event":"start","callId":"…","sampleRate":16000,"channels":1,"encoding":"audio/pcm-s16le","frameMs":60,
 "direction":"incoming","video":false,"videoStream":true}
{"event":"media","track":"inbound","seq":1,"payload":"<base64 PCM>"}
{"event":"video","track":"inbound","seq":1,"keyframe":true,"orientation":1,"payload":"<base64 H.264>"}
{"event":"video_state","active":true,"upgrade":false,"orientation":1,"state":"enabled","stateCode":1}
{"event":"keyframe_request"}
{"event":"error","code":"…","message":"…"}
{"event":"stop","reason":"peer_hangup"}
```

**Cliente → servidor**

```json
{"event":"media","payload":"<base64 PCM>"}
{"event":"video","payload":"<base64 H.264 access unit>"}
{"event":"clear"}
{"event":"stop"}
```

**Áudio**: PCM 16 bits, little-endian, **16 kHz, mono**, em base64. O servidor entrega quadros de 60 ms (960 amostras); você pode mandar pedaços de qualquer tamanho. `clear` descarta o que já estava na fila para o outro lado. Se o cliente ficar para trás, o áudio atrasado é descartado (contadores em `GET /call/{callId}`).

**Vídeo** (só em streams cujo bilhete pediu `video: true`; nos demais, mensagens `video` são ignoradas com um `error`):

- H.264 em **Annex-B** (códigos de início `00 00 01` / `00 00 00 01`), **um quadro (access unit) por mensagem**, com SPS e PPS na frente de cada keyframe. O formato MP4/AVCC (com prefixo de tamanho) é recusado.
- O servidor **não codifica nem decodifica**: só leva o H.264. Mande um keyframe primeiro e de novo **sempre que chegar `keyframe_request`** (o WhatsApp pede quando perde quadros).
- Para o vídeo que **chega**, um cliente que fica para trás perde a fila inteira e só volta a receber a partir do próximo keyframe (um quadro perdido estraga os seguintes).
- **Tamanho:** a tela da chamada no celular fica em pé. Mande vídeo **em pé** (por exemplo 360×640) para ocupar a tela toda; um vídeo deitado (640×360) aparece pequeno, com faixas.
- **Orientação do vídeo recebido:** o `orientation` de cada mensagem `video` é o número de **giros de 90° no sentido horário** que você deve aplicar à imagem para mostrá-la em pé (0 a 3). Ele acompanha a câmera em uso. O `orientation` de `video_state` é o do aparelho do outro lado e **não** serve para girar a imagem.

---

## Vídeo

`POST /call/video` com `{"callId": "...", "action": "..."}` numa chamada já atendida:

| `action` | O que faz |
|----------|-----------|
| `start` | Pede ao outro lado para transformar uma **chamada de áudio em vídeo**. O iPhone aceita e a chamada vira vídeo; depois disso você manda o vídeo pelo stream |
| `accept` | Aceita o pedido do outro lado (`video_state` com `upgrade: true`) |
| `stop` | Para de mandar vídeo; o áudio e o vídeo dele continuam |
| `disable` / `enable` | Silencia e reativa o **seu** vídeo |
| `orientation` | Informa a rotação da sua câmera (`orientation` de 0 a 3, giros horários) |

- Quando o **outro lado** liga a câmera numa chamada de áudio, o WhatsApp manda um pedido de upgrade (`video_state` com `upgrade: true`): responda com `accept`.
- **`enable` só reativa vídeo que já existia.** Numa chamada que nunca teve vídeo ele é **recusado com `409`** (o iPhone ignora "câmera ligada" sem o pedido de upgrade); use `start`.

**`video_state`** (mensagem do stream e evento `CallVideoState`): o campo `state` diz o que o outro lado sinalizou:

| `state` | `stateCode` | Significado |
|---------|-------------|-------------|
| `enabled` | 1 | Câmera ligada |
| `disabled` | 0 | Câmera silenciada |
| `stopped` | 6 | Parou de mandar vídeo |
| `upgrade_request` | 3 ou 11 | Pede para virar chamada de vídeo |
| `upgrade_accepted` | 4 | Aceitou o upgrade que você pediu |
| `upgrade_rejected` | 5 | Recusou o upgrade |
| `upgrade_cancelled` | 8 | Desistiu do pedido dele |
| `unknown` | outro | Estado sem nome; o número em `stateCode` identifica. O iPhone manda o **2** logo depois de a chamada ser atendida, antes do primeiro quadro (significado não confirmado) |

`active` e `upgrade` sozinhos não distinguem "aceitou o upgrade" de "desligou a câmera" (os dois ficam `false`); use `state`.

---

## Eventos

Com a assinatura `CALL`, além de `CallOffer`, `CallAccept`, `CallTerminate`, `CallOfferNotice`, `CallRelayLatency`, `CallPreAccept`, `CallReject`, `CallTransport` e `UnknownCallEvent` (da biblioteca do WhatsApp), o motor publica:

| Evento | Quando | Campos |
|--------|--------|--------|
| `CallReady` | A mídia da chamada subiu | `callId`, `peer`, `direction`, `video` |
| `CallEnded` | A chamada terminou | os anteriores, mais `reason` e `durationSeconds` |
| `CallVideoState` | O outro lado mudou o vídeo | `callId`, `peer`, `direction`, `video`, `active`, `upgrade`, `orientation`, `state`, `stateCode` |

**Motivos de `CallEnded` (`reason`)**

| `reason` | Quem encerrou |
|----------|---------------|
| `peer_hangup` | O outro lado desligou. O WhatsApp não manda motivo nesse caso; este nome cobre o "vazio" |
| `hangup` | Você desligou (`POST /call/hangup`) |
| `rejected` | A chamada foi rejeitada |
| `rejected_busy` | A instância já tinha `CALL_MAX_CONCURRENT` chamadas |
| `ring_timeout` | Ninguém atendeu em `CALL_RING_TIMEOUT` (só pega chamadas cujo fim nunca chegou) |
| `stream_closed` | A chamada em andamento ficou sem stream por mais de `CALL_STREAM_GRACE` |
| `server:<código>` | Erro do servidor do WhatsApp |
| outro texto | Motivo enviado pelo próprio WhatsApp |

Uma chamada em que quem liga desiste **antes de ser atendida** também termina como `peer_hangup` (não há um valor separado para "perdida").

---

## Limites e configuração

| Variável | Padrão | O que faz |
|----------|--------|-----------|
| `CALL_MAX_CONCURRENT` | `4` | Chamadas ao mesmo tempo por instância; as recebidas além disso são rejeitadas (`rejected_busy`) e as discadas recusadas |
| `CALL_RING_TIMEOUT` | `90` (s) | Tempo até largar uma chamada que ninguém atendeu |
| `CALL_STREAM_GRACE` | `10` (s) | Quanto uma chamada atendida espera o stream voltar antes de ser desligada |
| `CALL_DIAL_LIMIT` | `6` | Chamadas discadas por minuto por instância |
| `CALL_STREAM_ORIGINS` | vazio | Origens de navegador aceitas no WebSocket, separadas por vírgula (`*` aceita todas) |

Valores inválidos ou não positivos voltam ao padrão.

**Duração:** o servidor **não limita** quanto tempo dura uma chamada atendida. Se o stream cair, a chamada espera `CALL_STREAM_GRACE` segundos para o cliente abrir outro (com um bilhete novo de `POST /call/stream-ticket`) e só então é desligada (`stream_closed`).

---

## Erros

| Código | Quando |
|--------|--------|
| `400` | Corpo inválido; `callId` faltando; número para o qual não dá para ligar; ação ou orientação de vídeo desconhecidas |
| `404` | A chamada não existe (ou é de outra instância) |
| `409` | O motor não está ativo na instância; a chamada não está em um estado que permita a ação (por exemplo atender uma que já não toca, ou `enable` sem vídeo) |
| `429` | Limite de chamadas discadas por minuto, chamadas simultâneas demais (`CALL_MAX_CONCURRENT`) ou bilhetes de stream demais em aberto |
| `502` | O WhatsApp ou a biblioteca não conseguiu realizar a chamada |
| `500` | Falha ao avisar o outro lado ao desligar (a chamada termina do seu lado mesmo assim) |

---

## Rejeitar Chamada

Rejeita uma chamada recebida no WhatsApp. Vale **com ou sem** o motor de chamadas. Com o motor ligado, a rejeição passa por ele.

**Endpoint**: `POST /call/reject`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "callCreator": "5511999999999@s.whatsapp.net",
  "callId": "ABC123XYZ"
}
```

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `callCreator` | string (JID) | ✅ Sim | JID de quem está ligando |
| `callId` | string | ✅ Sim | ID da chamada |

Os dados (`callCreator` e `callId`) chegam pelo webhook no evento `CallOffer`.

**Resposta de Sucesso (200)**:
```json
{ "message": "success" }
```

### Rejeição automática

1. Receba o `CallOffer` no seu webhook.
2. Pegue o `callId` e quem ligou.
3. Chame `POST /call/reject` logo: se demorar, a chamada pode cair antes.

Para rejeitar só algumas (fora do horário, de números fora de uma lista, ou só as de vídeo), aplique a regra no seu webhook antes de chamar o endpoint. A instância também tem `rejectCall` e `msgRejectCall` nas configurações avançadas para rejeitar tudo automaticamente.

> Sem o motor de chamadas (`callsEnabled`), **não** é possível atender pela API, só rejeitar. Com ele, use `POST /call/answer`.

---

## Teste ao vivo

O repositório traz um script que exercita tudo isso com uma chamada real: `docker/fork-test/call-stream-test.py` (precisa de `pip install websockets`). Ele atende (ou disca), grava o áudio recebido em WAV e o vídeo em `.h264` (mais um `.orient` com a rotação de cada quadro), devolve o áudio em eco e pode mandar um arquivo H.264 de teste. `--help` mostra as opções e o comando de `ffmpeg` para gerar o vídeo de teste.

```bash
python call-stream-test.py --apikey TOKEN --echo                     # espera uma chamada, atende e devolve o áudio
python call-stream-test.py --apikey TOKEN --dial 5511999990000 --tone 3
python call-stream-test.py --apikey TOKEN --dial 5511999990000 --video --video-in test.h264
```

---

## Próximos Passos

- [Sistema de Eventos](../recursos-avancados/events-system.md) - Configurar webhooks
- [API de Instâncias](./api-instances.md) - `callsEnabled` nas configurações avançadas
- [Variáveis de Ambiente](../referencia/environment-variables.md)
- [Visão Geral da API](./api-overview.md)
