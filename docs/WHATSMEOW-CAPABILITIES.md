# O que o whatsmeow entrega e o que o projeto usa

Levantamento feito em 29/09/2026 (números recontados no fim do dia, depois das implementações listadas no §6) sobre `go.mau.fi/whatsmeow v0.0.0-20260929112325-8b41cfe6d9c4` (a versão hoje na `main` do fork). Os números vêm de ferramenta (extração dos métodos exportados de `*Client` e busca por `.Nome(` no código do projeto), não de estimativa. As afirmações sobre limites foram conferidas no código-fonte da lib.

## 1. Em números

| | |
|---|---|
| Métodos públicos de `*Client` | 136 |
| Usados pelo projeto | 79 (eram 65 no início do dia) |
| Não usados | 57 (por categoria no §4) |
| Tipos de evento emitidos (`types/events`) | 75 |
| Tratados em `myEventHandler` | 70 (eram 42 no início do dia) |
| Não tratados | 5, e nenhum deles chega sozinho ao handler (§5) |

O projeto usa bem o núcleo (conexão, envio, grupos, newsletters, privacidade, app state). O que sobra são recursos periféricos e, mais importante, **eventos operacionais** que explicam falhas relatadas nas issues.

## 2. Limites duros: o que a lib **não** permite

Vários pedidos esbarram na lib, não no projeto. Convém não prometê-los.

| Pedido | Situação na lib | Consequência |
|---|---|---|
| **Remover contato** | O encoder de app state só gera mutações `SET` (`appstate/encode.go`); o decoder entende `REMOVE`, mas não há como enviá-lo, e os helpers `Build*` não incluem contato (só mute, pin, archive, marcar como lido, label, star, apagar chat, push name) | `POST /user/contacts` cria/atualiza, **nunca remove**. Só mudaria se a lib ganhar a operação |
| **Atender / discar chamadas** | Só existe `RejectCall` e os eventos de chamada. Não há sinalização VoIP nem mídia | O fork implementa isso **fora** da lib, com a biblioteca `purpshell/meowcaller` (fixada em um commit; ela pendura um gancho nos nós de chamada do whatsmeow por reflexão e `unsafe`): é um subsistema próprio, **experimental**, ligado por instância (`callsEnabled`). Ver [`api-call.md`](wiki/guias-api/api-call.md) |
| **Encaminhar mensagem por ID** | A lib não guarda mensagens. "Encaminhar" é reenviar o conteúdo com `ContextInfo.IsForwarded`/`ForwardingScore` (o projeto já expõe `forwardingScore`) | Precisa do conteúdo: ou o cliente o envia, ou o projeto passa a persistir mensagens |
| **Botões / listas nativos** | A lib só gera o nó `<biz>` para `ButtonsMessage`, `ListMessage` e respostas; `InteractiveMessage` **não** é coberto (identico entre a versão de junho e a atual) | Renderização em conta pessoal depende do WhatsApp; ver FORK-TRIAGE.md §3 |
| **Saber o timer de mensagens temporárias de um chat** | `ChatSettingsStore` guarda só mute/pin/archive. Existe `SetDisappearingTimer`, mas não uma leitura do timer atual | Só dá para **aprender** o timer: vem em `ContextInfo.Expiration` de mensagens recebidas, em `events.GroupInfo.Ephemeral` e no history sync |
| **Reconexão automática** | O projeto liga `EnableAutoReconnect = false` e reconecta sozinho (`ReconnectClient`) | Por isso eventos que a lib só emite (não trata) precisam de tratamento no projeto |

## 3. O que a lib faz por dentro e o projeto **não** precisa duplicar

- **Proxy**: `SetProxy`/`SetProxyAddress` configuram **um transporte** usado pelo websocket e pelas requisições HTTP de mídia (existe a opção `NoWebsocket`). O receio de "mídia sai pelo IP real" não se confirma; o que vazava era o fallback silencioso para conexão direta, já tratado (`PROXY_FAIL_CLOSED`).
- **LID ↔ telefone**: a lib mantém o mapeamento (`Store.LIDs`) e, desde a versão atual, usa LID em DMs. O projeto só consulta (`/user/lid`).
- **Segredos de mensagem** (edição, voto, reação em comunidade): a chave depende do JID **como recebido**; por isso decifrar antes de qualquer normalização foi a correção certa (#60, #62).
- **Retry/re-request de mensagens não decifráveis**: `AutomaticMessageRerequestFromPhone` (já exposto como `REREQUEST_FROM_PHONE`).
- **Atualização da versão do cliente**: existe `GetLatestVersion`; o projeto tem sua própria busca em `web.whatsapp.com/sw.js` e agora aplica o resultado no handshake (`store.SetWAVersion`).

## 4. Métodos não usados, por categoria, e o que dá para fazer

| Categoria | Métodos | Utilidade / o que se pode fazer |
|---|---|---|
| **Mensagens temporárias** | `SetDisappearingTimer`, `SetDefaultDisappearingTimer` | **Feito**: `POST /chat/disappearing`, `POST /user/defaultDisappearing` e aprendizado do timer por chat (#79) |
| **Newsletters** | `FollowNewsletter`, `UnfollowNewsletter`, `NewsletterMarkViewed`, `NewsletterSendReaction`, `NewsletterToggleMute`, `GetNewsletterMessageUpdates` | **Feito**: seguir, deixar de seguir, marcar como visto, reagir e silenciar (sem teste ao vivo). Falta `GetNewsletterMessageUpdates` |
| **Grupos** | `GetGroupInfoFromInvite`, `GetGroupInfoFromLink`, `JoinGroupWithInvite`, `GetSubGroups`, `GetLinkedGroupsParticipants` | **Feito** para link e convite (`/group/inviteinfo`, `/group/joininvite`); falta comunidades (sub-grupos) |
| **Contatos / negócios** | `GetContactQRLink`, `ResolveContactQRLink`, `ResolveBusinessMessageLink`, `GetOrderDetails` | **Feito**: `GetBusinessProfile` (`POST /user/business`), `GetStatusPrivacy` (`GET /user/statusprivacy`) e `GetUserDevices` (`POST /user/devices`). Sobram link/QR de contato e pedidos |
| **Bots / IA** | `GetBotListV2`, `GetBotProfiles` | Listar bots do WhatsApp; nicho |
| **Mensagens (baixo nível)** | `BuildReaction`, `EncryptReaction`, `DecryptReaction`, `EncryptComment`, `DecryptComment`, `EncryptPollVote`, `RevokeMessage`, `BuildMessageKey`, `ParseWebMessage` | Reação e revogação são montadas à mão / com `BuildRevoke`. Reação e comentário em **comunidade** exigem `Decrypt*`. `BuildUnavailableMessageRequest` **já é usado** (`POST /message/rerequest`) |
| **Mídia** | `DownloadAny`, `DownloadToFile`, `DownloadThumbnail`, `DownloadMediaWith*`, `DeleteMedia`, `UploadReader`, `UploadNewsletterReader`, `DownloadHistorySync`, `FetchStickerPack` | Variantes por streaming/arquivo (menos memória em mídia grande), miniatura, pacotes de figurinhas |
| **Recibos / retry** | `SendMediaRetryReceipt`, `SendProtocolMessageReceipt`, `SendHistorySyncServerErrorReceipt`, `SetForceActiveDeliveryReceipts`, `SetMaxParallelRetryReceiptHandling` | Ajustes finos de entrega; só com evidência de problema |
| **Conexão / eventos** | `ConnectContext`, `ResetConnection`, `SetPassive`, `MarkNotDirty`, `AddEventHandlerWithSuccessStatus`, `RemoveEventHandlers`, `GetQRChannel`, `DangerousInternals` | **`WaitForConnection` já é usado** (`utils.WaitForClient`, no lugar dos `Sleep` fixos de 2 s e 3 s). `ConnectContext` permitiria cancelar uma conexão pendente; `GetQRChannel` é evitado de propósito (quebra o fluxo de passkey); `ResetConnection` não faz nada com auto-reconnect desligado |
| **HTTP / proxy** | `SetWebsocketHTTPClient`, `SetMediaHTTPClient`, `SetPreLoginHTTPClient`, `SetSOCKSProxy` | Só se for preciso proxies diferentes para websocket e mídia |
| **Facebook / push** | `SendFBMessage`, `DownloadFB*`, `RegisterForPushNotifications`, `GetServerPushNotificationConfig`, `AcceptTOSNotice`, `TryFetchPrivacySettings`, `StoreLIDPNMapping` | Sem uso para este projeto |

Observação: `SetGroupDescription` é contado como "usado" pela busca só porque o serviço do projeto tem um método de mesmo nome; a lib o marca como **deprecado** (`// Deprecated: duplicate of SetGroupTopic`) e o projeto chama o método correto, `SetGroupTopic`.

## 5. Eventos emitidos: o que foi tratado e o que sobra

**Todos os eventos que a lib entrega ao handler estão tratados.** Os 5 tipos que restam na contagem não chegam sozinhos:

| Tipo | Por que não precisa de tratamento |
|---|---|
| `BlocklistChange` | Vem dentro de `Blocklist.Changes` (publicado no evento `Blocklist`) |
| `MediaRetryError` | Vem dentro de `MediaRetry.Error` (`errorCode` no evento `MediaRetry`) |
| `NewsletterMessageMeta` | É um campo do evento de mensagem de canal, não um evento |
| `MexNotificationData` | Campo interno dos eventos "mex" (por exemplo `NewsletterMuteChange`) |
| `FBMessage` | Só existe em sessões Messenger/Instagram (`MessengerConfig`), que o projeto nunca liga |

Para onde cada evento vai (assinatura → evento publicado):

| Assinatura | Eventos |
|---|---|
| `CONNECTION` | `ReachoutTimelock` (o erro 463), `StreamError` (#185), `ClientOutdated` (405), `CATRefreshError`, `OfflineSyncPreview` (quantas mensagens estão na fila offline, #190) |
| `QRCODE` | `PairError`, `QRScannedWithoutMultidevice` (além de QR e passkey) |
| `CHAT_PRESENCE` | `Archive`, `Mute`, `Pin`, `Star`, `MarkChatAsRead`, `ClearChat`, `DeleteChat`, `DeleteForMe`, `UnarchiveChatsSetting`, `UserStatusMute` (mudanças de outro aparelho; o full sync não é publicado) |
| `CONTACT` | `Blocklist` (`action`, `dhash`, `changes`; `modify` sem `changes` = buscar a lista de novo), `PrivacySettings` (novos valores e o que mudou), `BusinessName` |
| `CALL` | `CallOffer`, `CallAccept`, `CallTerminate`, `CallOfferNotice`, `CallRelayLatency` e agora `CallPreAccept`, `CallTransport`, `CallReject`, `UnknownCallEvent`; do motor de chamadas do fork (não vêm da lib): `CallReady`, `CallEnded`, `CallVideoState` |
| `MESSAGE` | `Message`, `UndecryptableMessage` e `MediaRetry` (o remetente precisa reenviar a mídia; sem o texto cifrado) |
| `NEWSLETTER` | `NewsletterJoin`, `NewsletterLeave`, `NewsletterLiveUpdate`, `NewsletterMuteChange` |

Tratados sem publicação: `RotateADVSecret` (só um aviso no log: antes caía na linha genérica "Unhandled event", que imprimia o segredo ADV **antigo e o novo** da sessão no log da instância) e `ManualLoginReconnect` (não é emitido, porque o projeto mantém o login com reconexão automática). `PushNameSetting` já era tratado (renova o estado "Connected").

Como verificar a lista: `grep -c 'case \*events\.'` não basta, porque vários `case` agrupam tipos; a contagem acima procura cada tipo de `types/events` no código do serviço.

## 6. Recomendação (o que fazer com isso)

**Feito em 29/09/2026** (item 1): `NotifyAccountReachoutTimelock`, `StreamError` e `ClientOutdated` agora são publicados como eventos de conexão, aparecem no diagnóstico do runtime e têm efeito prático: o erro 463 do envio passa a explicar a restrição (e até quando), e o 405 descarta o cache da versão para a próxima reconexão buscar a atual. Detalhes em `docs/wiki/guias-api/api-fork-additions.md`.

**Feito depois disso**: eventos de pareamento (`PairError`...) e de estado de chat (`Mute`, `Pin`, `Star`...); mensagens temporárias (`POST /chat/disappearing`, `POST /user/defaultDisappearing`, timer aprendido e aplicado no envio, resolve o #79); grupo por convite (`/group/inviteinfo`, `/group/joininvite`); canais (seguir, deixar de seguir, silenciar, marcar como visto, reagir); `BuildUnavailableMessageRequest` (`POST /message/rerequest` e o evento `UndecryptableMessage`). Detalhes em `docs/wiki/guias-api/api-fork-additions.md` e, para o que foi corrigido no caminho (rotas de chat, bloqueio, rótulos, fila de webhook...), em `FORK-TRIAGE.md`.

**Eventos**: fechados em 29/09/2026 (§5).

Feitos depois: a espera pela conexão (`WaitForConnection`) e as consultas de dispositivos, privacidade de status e perfil comercial. O que resta é escolher quais dos 57 métodos ainda não usados (§4) valem a implementação; os candidatos são mídia por streaming (`UploadReader`, `DownloadToFile`, `DownloadThumbnail`) e, se comunidades forem usadas, `GetSubGroups`/`GetLinkedGroupsParticipants`; nenhum deles é necessário para corrigir algo que hoje funciona errado. Não prometer: remoção de contato, atender/discar chamadas e encaminhar por ID sem persistência (§2).
