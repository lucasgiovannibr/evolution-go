# O que o whatsmeow entrega e o que o projeto usa

Levantamento feito em 29/09/2026 sobre `go.mau.fi/whatsmeow v0.0.0-20260929112325-8b41cfe6d9c4` (a versão hoje na `main` do fork). Os números vêm de ferramenta (extração dos métodos exportados de `*Client` e busca por `.Nome(` no código do projeto), não de estimativa. As afirmações sobre limites foram conferidas no código-fonte da lib.

## 1. Em números

| | |
|---|---|
| Métodos públicos de `*Client` | 136 |
| Usados pelo projeto | 65 |
| Não usados | 71 (por categoria no §4) |
| Tipos de evento emitidos (`types/events`) | 75 |
| Tratados em `myEventHandler` | 42 |
| Não tratados | 33 (§5) |

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
| **Mensagens temporárias** | `SetDisappearingTimer`, `SetDefaultDisappearingTimer` | Expor `POST /chat/disappearing` e `POST /user/defaultDisappearing`. Base para resolver o #79 (junto do aprendizado do timer por chat, §2) |
| **Newsletters** | `FollowNewsletter`, `UnfollowNewsletter`, `NewsletterMarkViewed`, `NewsletterSendReaction`, `NewsletterToggleMute`, `GetNewsletterMessageUpdates` | Hoje só cria/lista/lê; faltam seguir, deixar de seguir, marcar como visto, reagir e silenciar |
| **Grupos** | `GetGroupInfoFromInvite`, `GetGroupInfoFromLink`, `JoinGroupWithInvite`, `GetSubGroups`, `GetLinkedGroupsParticipants` | Ver o grupo **antes** de entrar pelo link/convite; comunidades (sub-grupos) |
| **Contatos / negócios** | `GetContactQRLink`, `ResolveContactQRLink`, `GetBusinessProfile`, `ResolveBusinessMessageLink`, `GetOrderDetails`, `GetStatusPrivacy`, `GetUserDevices` | Perfil comercial, link/QR de contato, pedidos, privacidade do status, lista de dispositivos de um usuário |
| **Bots / IA** | `GetBotListV2`, `GetBotProfiles` | Listar bots do WhatsApp; nicho |
| **Mensagens (baixo nível)** | `BuildReaction`, `EncryptReaction`, `DecryptReaction`, `EncryptComment`, `DecryptComment`, `EncryptPollVote`, `RevokeMessage`, `BuildMessageKey`, `BuildUnavailableMessageRequest`, `ParseWebMessage` | Reação e revogação são montadas à mão / com `BuildRevoke`. `BuildUnavailableMessageRequest` (pedir ao celular reenvio de mensagem indisponível) é relevante para "não chegou / não descriptografou". Reação e comentário em **comunidade** exigem `Decrypt*` |
| **Mídia** | `DownloadAny`, `DownloadToFile`, `DownloadThumbnail`, `DownloadMediaWith*`, `DeleteMedia`, `UploadReader`, `UploadNewsletterReader`, `DownloadHistorySync`, `FetchStickerPack` | Variantes por streaming/arquivo (menos memória em mídia grande), miniatura, pacotes de figurinhas |
| **Recibos / retry** | `SendMediaRetryReceipt`, `SendProtocolMessageReceipt`, `SendHistorySyncServerErrorReceipt`, `SetForceActiveDeliveryReceipts`, `SetMaxParallelRetryReceiptHandling` | Ajustes finos de entrega; só com evidência de problema |
| **Conexão / eventos** | `ConnectContext`, `WaitForConnection`, `ResetConnection`, `SetPassive`, `MarkNotDirty`, `AddEventHandlerWithSuccessStatus`, `RemoveEventHandlers`, `GetQRChannel`, `DangerousInternals` | `ConnectContext` permitiria cancelar uma conexão pendente; `GetQRChannel` é evitado de propósito (quebra o fluxo de passkey); `ResetConnection` não faz nada com auto-reconnect desligado |
| **HTTP / proxy** | `SetWebsocketHTTPClient`, `SetMediaHTTPClient`, `SetPreLoginHTTPClient`, `SetSOCKSProxy` | Só se for preciso proxies diferentes para websocket e mídia |
| **Facebook / push** | `SendFBMessage`, `DownloadFB*`, `RegisterForPushNotifications`, `GetServerPushNotificationConfig`, `AcceptTOSNotice`, `TryFetchPrivacySettings`, `StoreLIDPNMapping` | Sem uso para este projeto |

Observação: `SetGroupDescription` é contado como "usado" pela busca só porque o serviço do projeto tem um método de mesmo nome; a lib o marca como **deprecado** (`// Deprecated: duplicate of SetGroupTopic`) e o projeto chama o método correto, `SetGroupTopic`.

## 5. Eventos emitidos e **não tratados**

Todos chegam ao handler e caem no ramo de "evento não tratado" (só log). Os que mais importam:

| Evento | O que significa | Por que importa |
|---|---|---|
| `NotifyAccountReachoutTimelock` (`EnforcementType`, `IsActive`, `TimeEnforcementEnds`) | O WhatsApp **restringiu a conta** para iniciar conversas com quem nunca falou com ela | É a causa provável do **erro 463** (#50, #124, #115). Hoje o usuário só vê "463" na hora de enviar; o evento diz até quando dura. Dá para publicá-lo como webhook e gravar o estado da instância |
| `StreamError` (`Code`, `Raw`) | `<stream:error>` com código **desconhecido** (os conhecidos viram outros eventos) | É exatamente o caso do #185 (`<ack class="status" type="media"/>`): a conexão cai e o projeto não sabe por quê. Registrar e expor no diagnóstico |
| `ClientOutdated` | O servidor rejeitou a versão do cliente (405) | Ligado à versão que só agora chega ao handshake (PR #199). Deveria gerar aviso claro e talvez forçar nova busca de versão |
| `PairError`, `QRScannedWithoutMultidevice`, `ManualLoginReconnect`, `CATRefreshError` | Falhas de pareamento/login | Hoje o usuário fica sem retorno quando o pareamento falha |
| `MediaRetry`, `MediaRetryError` | Mídia que o remetente precisa reenviar | Ligado a "a imagem só aparece depois de baixar" (#25) e a mídias que não baixam |
| `OfflineSyncPreview` | Quantas mensagens estão na fila offline | Diagnóstico do #190 (fila que só cresce) |
| `Mute`, `Pin`, `Star`, `MarkChatAsRead`, `DeleteChat`, `ClearChat`, `DeleteForMe`, `UnarchiveChatsSetting`, `UserStatusMute` | Mudanças de estado de chat feitas em **outro aparelho** (app state) | Hoje só `Archive` é publicado; integrações de CRM não sabem que o chat foi fixado, silenciado ou apagado |
| `PrivacySettings`, `Blocklist`, `BlocklistChange` | Privacidade e bloqueios | Sincronizar bloqueios com o sistema externo |
| `CallPreAccept`, `CallReject`, `CallTransport`, `UnknownCallEvent` | Eventos de chamada além de oferta/aceite/término | Completar o ciclo de chamadas nos webhooks |
| `NewsletterLiveUpdate`, `NewsletterMuteChange`, `NewsletterMessageMeta` | Atualizações de canais | Só se canais forem usados |
| `BusinessName` | Nome comercial mudou | Aparece no log como "Unhandled event" (visto no teste real) |

## 6. Recomendação (o que fazer com isso)

Em ordem de custo-benefício:

1. **Tratar `NotifyAccountReachoutTimelock`, `StreamError` e `ClientOutdated`**: publicar como evento de conexão, registrar no diagnóstico (`/instance/{id}/runtime`) e, no caso do timelock, devolver uma mensagem clara em vez de "463". Ataca #50, #124, #115, #185 e o 405, com pouco código e sem risco.
2. **`PairError` e companhia**: dar retorno ao usuário quando o pareamento falha.
3. **Eventos de estado de chat** (`Mute`, `Pin`, `Star`, `DeleteChat`...): publicar sob a assinatura `CHAT_PRESENCE` ou uma nova; útil para CRMs.
4. **Mensagens temporárias**: expor `SetDisappearingTimer` e aprender o timer por chat a partir de `ContextInfo.Expiration`, para resolver o #79.
5. **Newsletters** (seguir/silenciar/reagir) e **informações de grupo por convite**: completam APIs que hoje são parciais.
6. Não prometer: remoção de contato, atender/discar chamadas e encaminhar por ID sem persistência (§2).
