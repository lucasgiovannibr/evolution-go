# Propostas de features — para avaliação

Tudo aqui é **novo comportamento** (não correção) e por isso **não foi implementado** no fork. Vem de issues e PRs abertos no upstream. Cada item traz a origem, o tamanho, o risco e uma recomendação. Ver o contexto geral em [FORK-TRIAGE.md](FORK-TRIAGE.md).

Escala: **Esforço** = P (poucas linhas) · M · G (dias). **Recomendação**: ▶ implementar · ◐ avaliar antes · ✖ descartar por enquanto.

## Quick wins (baixo esforço, PR pronto ou quase)

| # | Proposta | Origem | Esforço | Risco | Recomendação |
|---|---|---|---|---|---|
| 1 | **Mídia "ver uma vez"** em `/send/media` (`viewOnce`) | PR [#147](https://github.com/evolution-foundation/evolution-go/pull/147) (37 linhas) | P | Baixo: campo opcional | ▶ |
| 2 | **`POST /message/markplayed`** — receipt `played` (microfone azul em áudio) | Issue [#45](https://github.com/evolution-foundation/evolution-go/issues/45) | P | Baixo: simétrico ao `markread`, usa `types.ReceiptTypePlayed` | ▶ |
| 3 | **`POST /message/subscribe`** — assinar presença de um contato (online/visto por último). Envia presença própria antes de assinar | PR [#152](https://github.com/evolution-foundation/evolution-go/pull/152), issue [#146](https://github.com/evolution-foundation/evolution-go/issues/146) | P | Médio: enviar presença própria interage com `alwaysOnline` (#55) — precisa ficar consistente | ◐ |
| 4 | **`PictureURL` em `POST /user/info`** | PR [#121](https://github.com/evolution-foundation/evolution-go/pull/121) (depende do #120, já aplicado) | M | Baixo | ▶ |
| 5 | **Resolver telefone a partir de LID** (`/user/...`) | PR [#179](https://github.com/evolution-foundation/evolution-go/pull/179) (111 linhas) | P | Baixo; útil porque as contas migram para LID | ▶ |
| 6 | **Salvar contato** na agenda do aparelho (`POST /user/contacts`) | PRs [#129](https://github.com/evolution-foundation/evolution-go/pull/129) / [#162](https://github.com/evolution-foundation/evolution-go/pull/162) | P | Baixo | ◐ |
| 7 | **Rotas de solicitações de entrada em grupo** (`GetGroupRequestParticipants` / `UpdateGroupRequestParticipants` existem no service e não têm rota) | Issue [#42](https://github.com/evolution-foundation/evolution-go/issues/42) | P | Baixo | ▶ |
| 8 | **Voto em enquete** `POST /send/pollVote` (usa `BuildPollVote` do whatsmeow; a correção do #60 já deixa o decrypt correto) | Issue [#26](https://github.com/evolution-foundation/evolution-go/issues/26) | M | Baixo/Médio | ◐ |

## Melhorias de comportamento (precisam de decisão)

| # | Proposta | Origem | Esforço | Comentário | Recomendação |
|---|---|---|---|---|---|
| 9 | **Conteúdo da mensagem citada** (`quoted`). Hoje `QuotedMessage` vai fixo vazio em ~20 pontos do `send_service.go` e o card de resposta aparece vazio/sem toque | Issue [#189](https://github.com/evolution-foundation/evolution-go/issues/189) | M | Duas opções: (a) aceitar o conteúdo no payload; (b) buscar a mensagem original no banco (`DATABASE_SAVE_MESSAGES`) — só funciona com persistência ligada. Sugiro (a) opcional + (b) como fallback | ▶ |
| 10 | **Timer de mensagens temporárias** herdado do chat nas mensagens enviadas (o destinatário vê "esta mensagem não vai desaparecer") | Issue [#79](https://github.com/evolution-foundation/evolution-go/issues/79) | M | Precisa do timer sincronizado por chat (app-state) ou um campo `expiration` opcional. Cosmético, mas visível ao usuário final | ◐ |
| 11 | **Thumbnail HQ em `/send/link`** (card grande de preview) | PR [#207](https://github.com/evolution-foundation/evolution-go/pull/207) (572 linhas), issue [#103](https://github.com/evolution-foundation/evolution-go/issues/103) | M | Envolve upload de mídia de link (`MediaLinkThumbnail`) — precisa de teste em aparelho | ◐ |
| 12 | **Status/config segura do proxy** (`GET /instance/proxy/{id}/status`, sem expor credenciais; opção `failClosed` — hoje há fallback silencioso para conexão direta) | Issue [#123](https://github.com/evolution-foundation/evolution-go/issues/123) | M | O fallback sem proxy vaza o IP real do servidor; para quem usa proxy por privacidade isso é relevante | ▶ |
| 13 | **`/instance/qr` devolver também o QR** quando existir, mesmo em conta que exige passkey | Issue [#148](https://github.com/evolution-foundation/evolution-go/issues/148) | P | Hoje devolve só o link de passkey (comportamento novo da 0.7.2) e quem integra uma tela própria perde o QR | ◐ |
| 14 | **Melhoria do `passkey-helper`** (WebAuthn/1Password): `navigator.credentials.get()` no MAIN world + service worker para as chamadas HTTP (evita o CSP do WhatsApp Web) | Issue [#173](https://github.com/evolution-foundation/evolution-go/issues/173) | M | O relator descreve a solução completa; a extensão está em `passkey-helper/`. Bloqueia quem usa gerenciador de senhas | ▶ |
| 15 | **Histórico profundo no pareamento** (`HistorySyncConfig`: 10 anos / 2 GB) | parte do PR [#133](https://github.com/evolution-foundation/evolution-go/pull/133) | P | Aumenta banda/armazenamento e tempo de sync; deveria ser configurável por env, não constante | ◐ |
| 16 | **Backoff do loop de reconexão** (até 30 min de espera) | PR [#197](https://github.com/evolution-foundation/evolution-go/pull/197) | M | Evita martelar o servidor com instância deslogada, mas atrasa recuperação legítima; janela e degraus deveriam ser configuráveis | ◐ |
| 17 | **Redesenho do ciclo de vida** (1 runtime por instância, restauração no startup, `GetQr` 409, backoff com jitter) | PRs [#145](https://github.com/evolution-foundation/evolution-go/pull/145), [#154](https://github.com/evolution-foundation/evolution-go/pull/154) | G | Resolveria de vez a família reconexão/duplicação, mas toca o coração do `StartClient` e sobrepõe o que já foi feito. Fazer só com ambiente de teste com sessões reais | ◐ |

## Features grandes (avaliar valor de produto)

| # | Proposta | Origem | Tamanho | Comentário | Recomendação |
|---|---|---|---|---|---|
| 18 | **Encaminhar mensagens** (`forward`) | PRs [#132](https://github.com/evolution-foundation/evolution-go/pull/132) / [#150](https://github.com/evolution-foundation/evolution-go/pull/150) | ~4,9 mil linhas (base `develop`) | O PR cria os arquivos em `routes/` e `sendMessage/` **na raiz do repositório**, não em `pkg/`: é uma cópia duplicada do `send_service.go` e não integra. A ideia é boa; reimplementar enxuto em `pkg/` | ◐ |
| 19 | **Evento de agenda** `POST /send/event` | PR [#90](https://github.com/evolution-foundation/evolution-go/pull/90) | 702 linhas, base `develop` | Recurso de nicho | ✖ por ora |
| 20 | **Chamadas**: atender/discar/controlar e stream de áudio/vídeo por WebSocket | PR [#141](https://github.com/evolution-foundation/evolution-go/pull/141) | 5,2 mil linhas / 19 arquivos | Muito valor para integrações de voz, mas é um subsistema novo com WebSocket, mídia e segurança próprias; exige projeto separado | ✖ por ora |
| 21 | **UI de chat no "sender"** (enviar/receber em tela) | PR [#182](https://github.com/evolution-foundation/evolution-go/pull/182) | 1,6 mil linhas | Ferramenta de teste; não é núcleo da API | ✖ por ora |
| 22 | **Manager: drawer mobile e ações visíveis em touch** | PR [#184](https://github.com/evolution-foundation/evolution-go/pull/184) | 5 arquivos de UI | Melhoria de usabilidade, baixo risco; não avaliei visualmente | ◐ (abrir o manager no celular antes de decidir) |

## Documentação a fazer

- Documentar `POST /group/settings` (ações `announcement`, `not_announcement`, `locked`, `unlocked`, `approval_on/off`, `admin_add`, `all_member_add`) — resolve as issues #42/#98/#113.
- Regenerar o swagger (`/group/description` está registrada e não aparece).
- Documentar o comportamento de `/chat/archive` (usa `chat`, não `number`) e o status "TODO: not working" das rotas de archive.
- Botões e listas em conta pessoal: registrar que dependem do whatsmeow/WhatsApp (ver FORK-TRIAGE.md §3) até haver validação.
- README: instruções de Windows (PRs #201/#202 são um começo, mas duplicam o bloco "Setup").
