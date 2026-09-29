# Propostas de features — para avaliação

Tudo aqui é **novo comportamento** (não correção) e por isso **não foi implementado** no fork. Vem de issues e PRs abertos no upstream. Cada item traz a origem, o tamanho, o risco e uma recomendação. Ver o contexto geral em [FORK-TRIAGE.md](FORK-TRIAGE.md).

Escala: **Esforço** = P (poucas linhas) · M · G (dias). **Recomendação**: ▶ implementar · ◐ avaliar antes · ✖ descartar por enquanto.

## Implementado no fork

Documentação dos endpoints: [`docs/wiki/guias-api/api-fork-additions.md`](docs/wiki/guias-api/api-fork-additions.md).

| Proposta | Origem | Resultado |
|---|---|---|
| Mídia "ver uma vez" (`viewOnce`) em `/send/media` | PR #147 | Feito (helper único + teste) |
| `POST /message/markplayed` | Issue #45 | **Já existia** na main; issue desatualizada |
| `POST /message/subscribe` (presença de contato) | PR #152, issue #146 | Feito; devolve a presença ao celular após 2 min se `alwaysOnline` estiver desligado (o PR original a deixava "online" para sempre) |
| `PictureURL` em `/user/info` | PR #121 | Feito com orçamento de tempo compartilhado e 429/504 |
| `POST /user/lid` | PR #179 | Feito |
| `POST /user/contacts` | PR #129 | Feito, com normalização do número (o PR usava o número cru) |
| Rotas de solicitações de entrada em grupo | Issue #42 | `POST /group/requests` e `/group/requests/update`; JIDs de participantes canônicos (sem `+`) |
| `POST /send/pollVote` | Issue #26 | Feito com `BuildPollVote`; testes da identidade da enquete |
| Status seguro do proxy + `PROXY_FAIL_CLOSED` | Issue #123 | Feito: `GET /instance/proxy/{id}`, sem credenciais; opção de nunca cair para conexão direta |
| Melhoria do `passkey-helper` (1Password) | Issue #173 | Feito: WebAuthn no mundo MAIN (v1.1.0) |
| `quoted.text` | Issue #189 | Feito antes |
| QR junto do passkey | Issue #148 | Feito antes |

## Ainda em aberto

## Melhorias de comportamento (precisam de decisão)

| # | Proposta | Origem | Esforço | Comentário | Recomendação |
|---|---|---|---|---|---|
| 10 | **Timer de mensagens temporárias** herdado do chat nas mensagens enviadas (o destinatário vê "esta mensagem não vai desaparecer") | Issue [#79](https://github.com/evolution-foundation/evolution-go/issues/79) | M | Precisa do timer sincronizado por chat (app-state) ou um campo `expiration` opcional. Cosmético, mas visível ao usuário final | ◐ |
| 11 | **Thumbnail HQ em `/send/link`** (card grande de preview) | PR [#207](https://github.com/evolution-foundation/evolution-go/pull/207) (572 linhas), issue [#103](https://github.com/evolution-foundation/evolution-go/issues/103) | M | Envolve upload de mídia de link (`MediaLinkThumbnail`) — precisa de teste em aparelho | ◐ |
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

## Documentação pendente

- `POST /group/settings` e o campo `quoted.text` já estão no wiki. Falta o swagger: o `swag init` reescreve ~1.000 linhas e remove as rotas de licença, então precisa ser ajustado antes de regenerar (inclui `/group/description`, que está registrada e não aparece).
- Documentar o comportamento de `/chat/archive` (usa `chat`, não `number`) e o status "TODO: not working" das rotas de archive.
- Botões e listas em conta pessoal: registrar que dependem do whatsmeow/WhatsApp (ver FORK-TRIAGE.md §3) até haver validação.
- README: instruções de Windows (PRs #201/#202 são um começo, mas duplicam o bloco "Setup").
