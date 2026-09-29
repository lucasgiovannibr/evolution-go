# Endpoints e opções adicionados no fork

Complementos à API do upstream (v0.7.2). Todos usam o header `apikey` da instância, exceto `GET /instance/proxy/{instanceId}`, que exige a **chave global**.

## Mensagens

### Mídia "ver uma vez" — `POST /send/media`

Novo campo opcional `viewOnce` (booleano). Vale para imagem, vídeo, áudio e vídeo-nota; documentos não suportam.

```json
{ "number": "5511999999999", "type": "image", "url": "https://exemplo.com/foto.jpg", "viewOnce": true }
```

### Votar em enquete — `POST /send/pollVote`

A instância precisa ter enviado ou recebido a enquete (o segredo da mensagem fica no store do whatsmeow).

```json
{
  "number": "120363000000000000@g.us",
  "pollMessageId": "3EB0DBF1C91EA77B149327",
  "participant": "5511888888888@s.whatsapp.net",
  "selectedOptions": ["Opção 1"]
}
```

| Campo | Descrição |
|---|---|
| `number` | Chat da enquete (número ou JID de grupo) |
| `pollMessageId` | ID da enquete (o `messageId` devolvido por `/send/poll`) |
| `participant` | Autor da enquete quando **não** foi esta instância. Obrigatório em grupos; em conversa 1:1 o padrão é `number` |
| `fromMe` | `true` quando a enquete foi enviada por esta instância |
| `selectedOptions` | Nomes das opções, exatamente como na enquete. Lista vazia remove o voto |

### Texto da citação — `quoted.text`

Campo opcional em qualquer envio com `quoted`. É o texto exibido no card da resposta; sem ele o WhatsApp mostra o card vazio.

### Assinar presença de um contato — `POST /message/subscribe`

```json
{ "number": "5511999999999" }
```

Passa a receber eventos `Presence` (online/offline/último visto) do contato (assinatura `PRESENCE`). O WhatsApp só entrega presença de outros enquanto a instância está "disponível"; por isso a instância fica disponível ao assinar e, **se `alwaysOnline` estiver desligado**, volta a "indisponível" após 2 minutos para não silenciar as notificações do celular. As assinaturas são perdidas ao reconectar. O evento agora traz também `from` e, quando conhecido, `lastSeen` (Unix).

## Usuário

### LID → telefone — `POST /user/lid`

```json
{ "lid": "123456789012345@lid", "groupJid": "120363000000000000@g.us" }
```

Consulta o mapeamento local do whatsmeow (o WhatsApp não tem consulta de servidor LID→telefone). `groupJid` é opcional: se o mapeamento não existir, a lista de participantes desse grupo é atualizada e a consulta repetida.

### Salvar contato — `POST /user/contacts`

```json
{ "phone": "5511999999999", "fullName": "Maria Silva", "firstName": "Maria", "saveOnPrimaryAddressbook": true }
```

Cria/atualiza o contato na lista do WhatsApp (app state) e, por padrão, pede ao aparelho principal que o grave também na agenda (`saveOnPrimaryAddressbook`, opcional; use `false` para manter só na lista interna). **Não existe remoção via API**: mutações de app state só gravam. Depois da sincronização ele aparece em `GET /user/contacts`.

### `PictureURL` em `POST /user/info`

Cada usuário passa a trazer `PictureURL` (além de `PictureID`). É best effort: as consultas dividem um orçamento de 5 s por chamada e param ao atingir limite de taxa do WhatsApp, então um lote grande nunca trava a resposta. Erros do WhatsApp agora saem como 429 (limite de taxa) ou 504 (timeout).

## Grupos

- `POST /group/requests` — lista quem aguarda aprovação para entrar (`{"groupJid": "...@g.us"}`).
- `POST /group/requests/update` — aprova ou rejeita (`{"groupJid": "...@g.us", "action": "approve", "participants": ["5511999999999"]}`; `action`: `approve` | `reject`).
- Os JIDs de participantes de `create`, `participant` e `requests/update` passam a ser enviados em forma canônica (sem `+`).

## Instância

### Status do proxy — `GET /instance/proxy/{instanceId}` (chave global)

```json
{
  "configured": true,
  "source": "instance",
  "protocol": "http",
  "host": "proxy.exemplo.com",
  "port": "8080",
  "hasAuth": true,
  "failClosed": false,
  "runtimeEnabled": true,
  "fallbackWithoutProxy": false,
  "lastAppliedAt": "2026-09-29T14:00:00Z"
}
```

Diz se o proxy está configurado **e** em uso pelo cliente em execução. Nunca devolve usuário, senha ou URL com credenciais. `fallbackWithoutProxy: true` significa que a autenticação no proxy falhou e o cliente conectou direto (expondo o IP do servidor). Para impedir isso, defina `PROXY_FAIL_CLOSED=true`: com ela o cliente **não conecta** quando o proxy falha.

### QR junto do passkey — `GET /instance/qr`

Quando há QR disponível, `qrcode`/`code` continuam vindo junto dos campos `passkey*`.

## Extensão `passkey-helper` 1.1.0

A chamada WebAuthn passou para o mundo `MAIN` da página, o que faz gerenciadores de senha (1Password, Bitwarden) aparecerem em vez de "insira sua chave de segurança". Requer Chrome/Edge 111+. Detalhes em `passkey-helper/README.md`.

## Saúde e diagnóstico

### `GET /health` — prontidão (pública)

Complementa `GET /server/ok`, que só diz que o processo está de pé (liveness) e continua sempre `200`.

```json
{ "status": "ok", "checks": { "usersDb": "ok", "authDb": "ok" } }
```

Cada verificação é `ok`, `slow` (o ping respondeu, mas em mais de 500 ms: é o que um pool esgotado ou sobrecarregado parece) ou `error` (falhou ou passou de 2 s). Qualquer `error` devolve **503** e `status: "unavailable"`; `slow` devolve 200 com `status: "degraded"`. As verificações rodam em paralelo (teto ~2 s). A rota é pública como `/server/ok` e só expõe estados, sem contagens nem identificadores.

Use `/server/ok` como *liveness* e `/health` como *readiness* no orquestrador: reiniciar o container porque o banco está lento não ajuda.

### `GET /instance/{instanceId}/runtime` — diagnóstico de uma instância

Chave global **ou** token da própria instância. Compara o que o banco diz com o que **este processo está executando**:

```json
{
  "instanceId": "…",
  "database": { "connected": true, "jid": "5531…:82@s.whatsapp.net", "alwaysOnline": false },
  "runtime": {
    "clientRegistered": true, "websocketConnected": true, "loggedIn": true,
    "deviceJid": "5531…:82@s.whatsapp.net",
    "runtimeActive": true, "killChannel": true, "supervisorCurrent": true,
    "reconnectInProgress": false,
    "qrCount": 0, "qrMax": 5, "passkeyCeremonyActive": false,
    "connectedSince": "…", "lastEventType": "Receipt", "lastEventAt": "…", "eventsSeen": 1234,
    "proxy": { "runtimeEnabled": true, "fallbackWithoutProxy": false }
  },
  "warnings": []
}
```

`warnings` lista cada inconsistência com um `code` estável:

| code | significa |
|---|---|
| `runtime_without_client` | um runtime é dono da instância, mas não há cliente (normal por alguns segundos ao iniciar) |
| `client_without_runtime` | cliente registrado sem supervisor: o kill/teardown de QR não o alcança |
| `supervisor_mismatch` | o estado do supervisor não pertence ao cliente registrado |
| `no_kill_channel` | Disconnect/teardown de QR não conseguem parar o runtime |
| `paired_but_offline` | dispositivo pareado, websocket caído e nenhuma reconexão em andamento |
| `qr_limit_near` | aguardando leitura do QR e perto do limite (o runtime vai reiniciar) |
| `db_connected_runtime_offline` / `db_disconnected_runtime_online` | banco e runtime discordam |
| `paired_in_db_unpaired_runtime` | o banco tem um JID pareado, mas o cliente em execução é um dispositivo novo (sessão perdida ou substituída) |
| `paired_without_runtime` | instância pareada sem nada rodando (chame `/instance/connect`) |
| `runtime_for_deleted_instance` | o processo ainda executa algo para uma instância que não existe mais |

### `GET /instance/runtimes` — todas as instâncias (chave global)

Devolve o mesmo diagnóstico de cada instância (inclusive de runtimes sem linha no banco), um `summary` (`instances`, `connected`, `withWarnings`) e estatísticas do processo (`uptimeSeconds`, `goroutines`, `heapAllocMb`, `sysMb`, `numGc`, `goVersion`). Um número de goroutines que só cresce é como um vazamento aparece.

### `ENABLE_PPROF=true` — profiler (opcional)

Expõe `/debug/pprof/*` (goroutine, heap, cpu…) **somente com a chave global**. Desligado por padrão. Ex.: `GET /debug/pprof/goroutine?debug=1` lista as pilhas agrupadas.

## Eventos de conexão novos

Três eventos que o whatsmeow já emitia e o projeto ignorava (só aparecia "Unhandled event" no log). Chegam pela assinatura **`CONNECTION`** (webhook, RabbitMQ, NATS, WebSocket e filas globais), como os demais eventos de conexão, e também aparecem em `GET /instance/{id}/runtime`.

### `ReachoutTimelock` — conta restrita para iniciar conversas

O WhatsApp restringiu a conta: ela não pode iniciar conversa com quem nunca falou com ela. Enviar para esse contato falha com o erro **463** (`NackCallerReachoutTimelocked`). Contatos que já conversaram continuam funcionando.

```json
{ "event": "ReachoutTimelock", "data": { "active": true, "enforcementType": "…", "endsAt": "2026-10-01T12:00:00Z" } }
```

`active: false` avisa que a restrição foi levantada. `endsAt` só aparece quando o WhatsApp informa o fim. Além do evento:

- o **erro do envio** deixa de ser "server returned error 463" e passa a explicar o que houve (e até quando, se a restrição é conhecida). O texto original continua na mensagem;
- o diagnóstico mostra `runtime.reachoutTimelock` e o aviso `reachout_timelock_active`.

O estado fica em memória: depois de reiniciar o processo ele só volta quando o WhatsApp o enviar de novo.

### `StreamError` — erro de stream desconhecido

```json
{ "event": "StreamError", "data": { "code": "…", "raw": "{…}" } }
```

`<stream:error>` com um código que a lib não conhece (os conhecidos viram outros eventos). É o que antecede a conexão morta do issue #185 do upstream. O diagnóstico traz `runtime.lastStreamError` e, por 30 minutos, o aviso `recent_stream_error`.

### `ClientOutdated` — versão do cliente recusada (405)

```json
{ "event": "ClientOutdated", "data": { "versionPinned": false } }
```

O WhatsApp recusou a versão do cliente. O projeto **descarta o cache da versão** (que valia 1 hora e faria todas as novas tentativas repetirem a versão recusada) para a próxima reconexão buscar a atual. Se `versionPinned` for `true`, as variáveis `WHATSAPP_VERSION_*` fixam a versão e precisam ser atualizadas ou removidas. O diagnóstico traz `runtime.clientOutdatedAt` e, por 30 minutos, o aviso `client_outdated`.
