# O que o whatsmeow entrega e o que o projeto usa

Levantamento feito em 29/09/2026 (números recontados no fim do dia, depois das implementações listadas no §6) sobre `go.mau.fi/whatsmeow v0.0.0-20260929112325-8b41cfe6d9c4` (a versão hoje na `main` do fork). Os números vêm de ferramenta (extração dos métodos exportados de `*Client` e busca por `.Nome(` no código do projeto), não de estimativa. As afirmações sobre limites foram conferidas no código-fonte da lib.

## 1. Em números

| | |
|---|---|
| Métodos públicos de `*Client` | 136 |
| Usados pelo projeto | 76 (eram 65 no início do dia) |
| Não usados | 60 (por categoria no §4) |
| Tipos de evento emitidos (`types/events`) | 75 |
| Tratados em `myEventHandler` | 56 (eram 42 no início do dia) |
| Não tratados | 19 (§5) |

O projeto usa bem o núcleo (conexão, envio, grupos, newsletters, privacidade, app state). O que sobra são recursos periféricos e, mais importante, **eventos operacionais** que explicam falhas relatadas nas issues.

## 2. Limites duros: o que a lib **não** permite

Vários pedidos esbarram na lib, não no projeto. Convém não prometê-los.

| Pedido | Situação na lib | Consequência |
|---|---|---|
| **Remover contato** | O encoder de app state só gera mutações `SET` (`appstate/encode.go`); o decoder entende `REMOVE`, mas não há como enviá-lo, e os helpers `Build*` não incluem contato (só mute, pin, archive, marcar como lido, label, star, apagar chat, push name) | `POST /user/contacts` cria/atualiza, **nunca remove**. Só mudaria se a lib ganhar a operação |
| **Atender / discar chamadas** | Só existe `RejectCall` e os eventos de chamada. Não há sinalização VoIP nem mídia | O PR #141 (5,2 mil linhas) implementa isso **fora** da lib: é um subsistema próprio, com risco e manutenção próprios |
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
| **Contatos / negócios** | `GetContactQRLink`, `ResolveContactQRLink`, `GetBusinessProfile`, `ResolveBusinessMessageLink`, `GetOrderDetails`, `GetStatusPrivacy`, `GetUserDevices` | Perfil comercial, link/QR de contato, pedidos, privacidade do status, lista de dispositivos de um usuário |
| **Bots / IA** | `GetBotListV2`, `GetBotProfiles` | Listar bots do WhatsApp; nicho |
| **Mensagens (baixo nível)** | `BuildReaction`, `EncryptReaction`, `DecryptReaction`, `EncryptComment`, `DecryptComment`, `EncryptPollVote`, `RevokeMessage`, `BuildMessageKey`, `ParseWebMessage` | Reação e revogação são montadas à mão / com `BuildRevoke`. Reação e comentário em **comunidade** exigem `Decrypt*`. `BuildUnavailableMessageRequest` **já é usado** (`POST /message/rerequest`) |
| **Mídia** | `DownloadAny`, `DownloadToFile`, `DownloadThumbnail`, `DownloadMediaWith*`, `DeleteMedia`, `UploadReader`, `UploadNewsletterReader`, `DownloadHistorySync`, `FetchStickerPack` | Variantes por streaming/arquivo (menos memória em mídia grande), miniatura, pacotes de figurinhas |
| **Recibos / retry** | `SendMediaRetryReceipt`, `SendProtocolMessageReceipt`, `SendHistorySyncServerErrorReceipt`, `SetForceActiveDeliveryReceipts`, `SetMaxParallelRetryReceiptHandling` | Ajustes finos de entrega; só com evidência de problema |
| **Conexão / eventos** | `ConnectContext`, `WaitForConnection`, `ResetConnection`, `SetPassive`, `MarkNotDirty`, `AddEventHandlerWithSuccessStatus`, `RemoveEventHandlers`, `GetQRChannel`, `DangerousInternals` | `ConnectContext` permitiria cancelar uma conexão pendente; `GetQRChannel` é evitado de propósito (quebra o fluxo de passkey); `ResetConnection` não faz nada com auto-reconnect desligado |
| **HTTP / proxy** | `SetWebsocketHTTPClient`, `SetMediaHTTPClient`, `SetPreLoginHTTPClient`, `SetSOCKSProxy` | Só se for preciso proxies diferentes para websocket e mídia |
| **Facebook / push** | `SendFBMessage`, `DownloadFB*`, `RegisterForPushNotifications`, `GetServerPushNotificationConfig`, `AcceptTOSNotice`, `TryFetchPrivacySettings`, `StoreLIDPNMapping` | Sem uso para este projeto |

Observação: `SetGroupDescription` é contado como "usado" pela busca só porque o serviço do projeto tem um método de mesmo nome; a lib o marca como **deprecado** (`// Deprecated: duplicate of SetGroupTopic`) e o projeto chama o método correto, `SetGroupTopic`.

## 5. Eventos emitidos e **não tratados**

Os ✅ desta tabela já são tratados; os demais chegam ao handler e caem no ramo de "evento não tratado" (só log). Ainda não tratados (19): `Blocklist`, `BlocklistChange`, `BusinessName`, `CallPreAccept`, `CallReject`, `CallTransport`, `FBMessage`, `ManualLoginReconnect`, `MediaRetry`, `MediaRetryError`, `MexNotificationData`, `NewsletterLiveUpdate`, `NewsletterMessageMeta`, `NewsletterMuteChange`, `OfflineSyncPreview`, `PrivacySettings`, `PushNameSetting`, `RotateADVSecret`, `UnknownCallEvent`. Os que mais importam:

| Evento | O que significa | Por que importa |
|---|---|---|
| ✅ **Tratado** — `NotifyAccountReachoutTimelock` (`EnforcementType`, `IsActive`, `TimeEnforcementEnds`) | O WhatsApp **restringiu a conta** para iniciar conversas com quem nunca falou com ela | É a causa provável do **erro 463** (#50, #124, #115). Hoje o usuário só vê "463" na hora de enviar; o evento diz até quando dura. Dá para publicá-lo como webhook e gravar o estado da instância |
| ✅ **Tratado** — `StreamError` (`Code`, `Raw`) | `<stream:error>` com código **desconhecido** (os conhecidos viram outros eventos) | É exatamente o caso do #185 (`<ack class="status" type="media"/>`): a conexão cai e o projeto não sabe por quê. Registrar e expor no diagnóstico |
| ✅ **Tratado** — `ClientOutdated` | O servidor rejeitou a versão do cliente (405) | Ligado à versão que só agora chega ao handshake (PR #199). Deveria gerar aviso claro e talvez forçar nova busca de versão |
| ✅ **Tratados** — `PairError`, `QRScannedWithoutMultidevice`, `CATRefreshError` (`ManualLoginReconnect` não é emitido com o auto-reconnect desligado do projeto) | Falhas de pareamento/login | Publicados sob `QRCODE` e `CONNECTION`; o usuário passa a ter retorno quando o pareamento falha |
| `MediaRetry`, `MediaRetryError` | Mídia que o remetente precisa reenviar | Ligado a "a imagem só aparece depois de baixar" (#25) e a mídias que não baixam |
| `OfflineSyncPreview` | Quantas mensagens estão na fila offline | Diagnóstico do #190 (fila que só cresce) |
| ✅ **Tratados** — `Mute`, `Pin`, `Star`, `MarkChatAsRead`, `DeleteChat`, `ClearChat`, `DeleteForMe`, `UnarchiveChatsSetting`, `UserStatusMute` | Mudanças de estado de chat feitas em **outro aparelho** (app state) | Publicados sob `CHAT_PRESENCE`, onde `Archive` já estava; o full sync depois do pareamento não é publicado. `UndecryptableMessage` também passou a ser publicado (`MESSAGE`) |
| `PrivacySettings`, `Blocklist`, `BlocklistChange` | Privacidade e bloqueios | Sincronizar bloqueios com o sistema externo |
| `CallPreAccept`, `CallReject`, `CallTransport`, `UnknownCallEvent` | Eventos de chamada além de oferta/aceite/término | Completar o ciclo de chamadas nos webhooks |
| `NewsletterLiveUpdate`, `NewsletterMuteChange`, `NewsletterMessageMeta` | Atualizações de canais | Só se canais forem usados |
| `BusinessName` | Nome comercial mudou | Aparece no log como "Unhandled event" (visto no teste real) |

## 6. Recomendação (o que fazer com isso)

**Feito em 29/09/2026** (item 1): `NotifyAccountReachoutTimelock`, `StreamError` e `ClientOutdated` agora são publicados como eventos de conexão, aparecem no diagnóstico do runtime e têm efeito prático: o erro 463 do envio passa a explicar a restrição (e até quando), e o 405 descarta o cache da versão para a próxima reconexão buscar a atual. Detalhes em `docs/wiki/guias-api/api-fork-additions.md`.

**Feito depois disso**: eventos de pareamento (`PairError`...) e de estado de chat (`Mute`, `Pin`, `Star`...); mensagens temporárias (`POST /chat/disappearing`, `POST /user/defaultDisappearing`, timer aprendido e aplicado no envio, resolve o #79); grupo por convite (`/group/inviteinfo`, `/group/joininvite`); canais (seguir, deixar de seguir, silenciar, marcar como visto, reagir); `BuildUnavailableMessageRequest` (`POST /message/rerequest` e o evento `UndecryptableMessage`). Detalhes em `docs/wiki/guias-api/api-fork-additions.md` e, para o que foi corrigido no caminho (rotas de chat, bloqueio, rótulos, fila de webhook...), em `FORK-TRIAGE.md`.

O que resta:

1. Os demais eventos não tratados de §5, só quando houver quem precise. Os mais úteis seriam `MediaRetry`/`MediaRetryError` (mídia que não baixa), `OfflineSyncPreview` (fila offline que só cresce, #190) e `Blocklist*`/`PrivacySettings` (sincronizar com sistemas externos).
2. Não prometer: remoção de contato, atender/discar chamadas e encaminhar por ID sem persistência (§2).
