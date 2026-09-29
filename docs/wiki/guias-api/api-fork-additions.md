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
{ "phone": "5511999999999", "fullName": "Maria Silva", "firstName": "Maria" }
```

Cria/atualiza o contato na lista do WhatsApp (app state) e pede ao aparelho principal que o grave também na agenda. Depois da sincronização ele aparece em `GET /user/contacts`.

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
