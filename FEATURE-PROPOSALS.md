# Propostas de features e sugestões

Vem de issues e PRs abertos no upstream e do que apareceu no teste com instância real. Cada item traz origem, esforço, risco e recomendação. Contexto geral e status de todas as issues/PRs em [FORK-TRIAGE.md](FORK-TRIAGE.md).

Escala: **Esforço** = P (poucas linhas) · M · G (dias). **Recomendação**: ▶ implementar · ◐ avaliar antes · ✖ descartar por enquanto.

## 1. Implementado no fork

Documentação dos endpoints: [`docs/wiki/guias-api/api-fork-additions.md`](docs/wiki/guias-api/api-fork-additions.md). A coluna "Ao vivo" indica o que foi exercitado com uma instância real em 29/09/2026.

| Proposta | Origem | Resultado | Ao vivo |
|---|---|---|---|
| Mídia "ver uma vez" (`viewOnce`) em `/send/media` | PR #147 | Feito (JSON e multipart; helper único + teste) | ✅ aparece "abrir uma vez" |
| `POST /message/markplayed` | Issue #45 | **Já existia** na main; issue desatualizada | — |
| `POST /message/subscribe` (presença de contato) | PR #152, issue #146 | Feito; devolve a presença ao celular após 2 min se `alwaysOnline` estiver desligado (o PR original a deixava "online" para sempre) | 🟡 responde sucesso; entrega de eventos `Presence` não observada |
| `PictureURL` em `/user/info` | PR #121 | Feito com orçamento de tempo compartilhado e 429/504 | ✅ 0,34 s |
| `POST /user/lid` | PR #179 | Feito | ✅ LID → telefone |
| `POST /user/contacts` | PR #129 | Feito, com normalização do número e `saveOnPrimaryAddressbook` opcional. **Não há remoção via API** (app state só grava) | ✅ contatos de teste criados |
| Rotas de solicitações de entrada em grupo | Issue #42 | `POST /group/requests` e `/group/requests/update`; JIDs de participantes canônicos (sem `+`) | ✅ lista vazia, validações; aprovar/rejeitar de um pedido real não testado |
| `POST /send/pollVote` | Issue #26 | Feito com `BuildPollVote`; tenta o LID próprio se o segredo da enquete estiver sob ele | ✅ voto aparece no celular |
| Status seguro do proxy + `PROXY_FAIL_CLOSED` | Issue #123 | `GET /instance/proxy/{id}`, sem credenciais; opção de nunca cair para conexão direta | 🟡 só "sem proxy"; proxy real **não testado** (decisão sua) |
| Melhoria do `passkey-helper` (1Password) | Issue #173 | WebAuthn no mundo MAIN (v1.1.0) | ❌ não testado (conta sem passkey) |
| `quoted.text` | Issue #189 | Texto do card da citação (JSON e multipart) | ✅ card mostra o texto |
| QR junto do passkey | Issue #148 | `/instance/qr` mantém `qrcode` ao lado dos campos `passkey*` | — |
| Resultado por participante em `/group/participant` | Achado no teste real | `data` por participante e `failed` (antes "success" mesmo sem adicionar) | ✅ número inexistente → `Error: 404` |

## 2. Propostas ainda em aberto

### Melhorias de comportamento

| # | Proposta | Origem | Esforço | Comentário | Recomendação |
|---|---|---|---|---|---|
| 10 | **Timer de mensagens temporárias** herdado do chat nas mensagens enviadas (o destinatário vê "esta mensagem não vai desaparecer") | Issue [#79](https://github.com/evolution-foundation/evolution-go/issues/79) | M | Precisa do timer sincronizado por chat (app-state) ou de um campo `expiration` opcional. Cosmético, mas visível ao usuário final | ◐ |
| 11 | **Thumbnail HQ em `/send/link`** (card grande de preview) | PR [#207](https://github.com/evolution-foundation/evolution-go/pull/207) (572 linhas), issue [#103](https://github.com/evolution-foundation/evolution-go/issues/103) | M | Envolve upload de mídia de link (`MediaLinkThumbnail`); precisa de teste em aparelho | ◐ |
| 15 | **Histórico profundo no pareamento** (`HistorySyncConfig`: 10 anos / 2 GB) | parte do PR [#133](https://github.com/evolution-foundation/evolution-go/pull/133) | P | Aumenta banda/armazenamento e tempo de sync; deveria ser configurável por env, não constante | ◐ |
| 16 | **Backoff do loop de reconexão** (até 30 min de espera) | PR [#197](https://github.com/evolution-foundation/evolution-go/pull/197) | M | Evita martelar o servidor com instância deslogada, mas atrasa recuperação legítima; janela e degraus deveriam ser configuráveis | ◐ |
| 17 | **Redesenho do ciclo de vida** (restauração no startup, `GetQr` 409, backoff com jitter) | PRs [#145](https://github.com/evolution-foundation/evolution-go/pull/145), [#154](https://github.com/evolution-foundation/evolution-go/pull/154) | G | O núcleo (um runtime por instância, canal de kill próprio, restauração de `Reconnecting`, instância apagada não reinicia) **já foi feito** de forma cirúrgica. O que sobra é opcional | ◐ |

### Features grandes (avaliar valor de produto)

| # | Proposta | Origem | Tamanho | Comentário | Recomendação |
|---|---|---|---|---|---|
| 18 | **Encaminhar mensagens** (`forward`) | PRs [#132](https://github.com/evolution-foundation/evolution-go/pull/132) / [#150](https://github.com/evolution-foundation/evolution-go/pull/150) | ~4,9 mil linhas (base `develop`) | O PR cria os arquivos em `routes/` e `sendMessage/` **na raiz do repositório**, não em `pkg/`: é uma cópia duplicada do `send_service.go` e não integra. A ideia é boa; reimplementar enxuto em `pkg/` | ◐ |
| 19 | **Evento de agenda** `POST /send/event` | PR [#90](https://github.com/evolution-foundation/evolution-go/pull/90) | 702 linhas, base `develop` | Recurso de nicho | ✖ por ora |
| 20 | **Chamadas**: atender/discar/controlar e stream de áudio/vídeo por WebSocket | PR [#141](https://github.com/evolution-foundation/evolution-go/pull/141) | 5,2 mil linhas / 19 arquivos | Muito valor para integrações de voz, mas é um subsistema novo com WebSocket, mídia e segurança próprias; exige projeto separado | ✖ por ora |
| 21 | **UI de chat no "sender"** (enviar/receber em tela) | PR [#182](https://github.com/evolution-foundation/evolution-go/pull/182) | 1,6 mil linhas | Ferramenta de teste; não é núcleo da API | ✖ por ora |
| 22 | **Manager: drawer mobile e ações visíveis em touch** | PR [#184](https://github.com/evolution-foundation/evolution-go/pull/184) | 5 arquivos de UI | Melhoria de usabilidade, baixo risco; não avaliei visualmente | ◐ (abrir o manager no celular antes de decidir) |

## 3. Sugestões novas (surgiram no teste real; ainda não implementadas)

| # | Sugestão | Por quê | Esforço | Recomendação |
|---|---|---|---|---|
| 23 | **Endpoint de diagnóstico do runtime** (`GET /instance/{id}/runtime`: cliente registrado, conectado, logado, runtime ativo, contagem de QR, último evento) | O bug do runtime duplicado só foi achado lendo logs; hoje não há como ver o estado interno. Ajuda a reproduzir relatos como #85 e #185 | P/M | ▶ |
| 24 | **Health check que enxergue o Postgres e os runtimes** (`/server/ok` responde 200 mesmo com o pool esgotado) | No #175 o health check ficou cego justo quando o pool esgotou | P | ▶ |
| 25 | **Métricas Prometheus** (instâncias conectadas, reconexões, eventos entregues/falhos, conexões do pool) | Observar uso prolongado e alertar antes de a instância cair | M | ◐ |
| 26 | **Resultado por item nas operações em lote** (participantes de grupo já foi feito; falta `subscribe` com lista de números, como pediu o #146) | Evita "success" que esconde falha parcial | P | ◐ |
| 27 | **Reativar o agendador de presença quando `alwaysOnline` é ligado em tempo de execução** | Hoje ele só nasce no evento `Connected`; ligar depois não tem efeito até reconectar | P | ◐ |
| 28 | **Remover contato via API** | Não existe: a lib só grava mutações de app state. Só seria possível se o whatsmeow ganhar a operação `REMOVE` | — | ✖ (depende da lib) |
| 29 | **Regenerar o swagger sem regressão** (o `swag init` reescreve ~1.000 linhas e remove as rotas de licença) | Documentação da API desatualizada (`/group/description`, `/group/settings`, novos endpoints) | M | ◐ |

## 4. Documentação pendente

- Os endpoints novos, `POST /group/settings` e `quoted.text` estão no wiki; falta o swagger (item 29).
- Documentar o comportamento de `/chat/archive` (usa `chat`, não `number`) e o status "TODO: not working" das rotas de archive.
- Botões e listas em conta pessoal: já registrado em `api-interactive.md`; revisar quando houver validação em aparelho.
- README: instruções de Windows (PRs #201/#202 são um começo, mas duplicam o bloco "Setup").
