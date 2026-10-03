# API de Chats

Documentação completa dos endpoints para gerenciar conversas (pin, arquivo, mute).

## 📋 Índice

- [Fixar Conversa](#fixar-conversa)
- [Desfixar Conversa](#desfixar-conversa)
- [Arquivar Conversa](#arquivar-conversa)
- [Desarquivar Conversa](#desarquivar-conversa)
- [Silenciar Conversa](#silenciar-conversa)
- [Dessilenciar Conversa](#dessilenciar-conversa)
- [Sincronizar Histórico](#sincronizar-histórico)

---

## Fixar Conversa

Fixa uma conversa no topo da lista de chats.

**Endpoint**: `POST /chat/pin`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "chat": "5511999999999@s.whatsapp.net"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `chat` | string | ✅ Sim | JID do chat (individual ou grupo) |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

**Exemplo cURL**:
```bash
# Fixar chat individual
curl -X POST http://localhost:4000/chat/pin \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "5511999999999@s.whatsapp.net"
  }'

# Fixar grupo
curl -X POST http://localhost:4000/chat/pin \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "120363XXXXXXXXXX@g.us"
  }'
```

---

## Desfixar Conversa

Remove a fixação de uma conversa.

**Endpoint**: `POST /chat/unpin`

**Body**:
```json
{
  "chat": "5511999999999@s.whatsapp.net"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `chat` | string | ✅ Sim | JID do chat |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/chat/unpin \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "5511999999999@s.whatsapp.net"
  }'
```

---

## Arquivar Conversa

Move uma conversa para o arquivo.

**Endpoint**: `POST /chat/archive`

**Body**:
```json
{
  "chat": "5511999999999@s.whatsapp.net"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `chat` | string | ✅ Sim | JID do chat |

**Nota**: Conversas arquivadas não aparecem na lista principal, mas ainda recebem mensagens.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/chat/archive \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "5511999999999@s.whatsapp.net"
  }'
```

---

## Desarquivar Conversa

Restaura uma conversa arquivada para a lista principal.

**Endpoint**: `POST /chat/unarchive`

**Body**:
```json
{
  "chat": "5511999999999@s.whatsapp.net"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `chat` | string | ✅ Sim | JID do chat |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/chat/unarchive \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "5511999999999@s.whatsapp.net"
  }'
```

---

## Silenciar Conversa

Silencia notificações de uma conversa por 1 hora.

**Endpoint**: `POST /chat/mute`

**Body**:
```json
{
  "chat": "5511999999999@s.whatsapp.net"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `chat` | string | ✅ Sim | JID do chat |

**Nota**: O silenciamento é fixo por **1 hora** (hard-coded no código: `1*time.Hour`). Para silenciar permanentemente, use o aplicativo WhatsApp.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/chat/mute \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "5511999999999@s.whatsapp.net"
  }'
```

---

## Dessilenciar Conversa

Remove o silenciamento de uma conversa.

**Endpoint**: `POST /chat/unmute`

**Body**:
```json
{
  "chat": "5511999999999@s.whatsapp.net"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `chat` | string | ✅ Sim | JID do chat |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/chat/unmute \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "chat": "5511999999999@s.whatsapp.net"
  }'
```

---

## Sincronizar Histórico

Solicita sincronização de histórico de mensagens antigas (WhatsApp Multi-Device).

**Endpoint**: `POST /chat/history-sync`

**Body**:
```json
{
  "messageInfo": {
    "Chat": "5511999999999@s.whatsapp.net",
    "IsFromMe": false,
    "IsGroup": false,
    "ID": "3EB0C5A277F7F9B6C599",
    "Timestamp": "2025-11-11T10:00:00Z"
  },
  "count": 50
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `messageInfo` | object | ✅ Sim | Informações da mensagem de referência |
| `messageInfo.Chat` | string | ✅ Sim | JID do chat |
| `messageInfo.IsFromMe` | bool | ✅ Sim | Se a mensagem foi enviada por você |
| `messageInfo.IsGroup` | bool | ✅ Sim | Se é um grupo |
| `messageInfo.ID` | string | ✅ Sim | ID da mensagem de referência |
| `messageInfo.Timestamp` | string | ✅ Sim | Timestamp da mensagem |
| `count` | int | ✅ Sim | Número de mensagens para sincronizar |

**Nota**: Este endpoint é usado para sincronizar mensagens antigas do histórico do WhatsApp Multi-Device. Requer uma mensagem de referência válida.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "Timestamp": "2025-11-11T10:30:00Z",
    "ID": "abc123",
    "ServerID": 12345
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/chat/history-sync \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "messageInfo": {
      "Chat": "5511999999999@s.whatsapp.net",
      "IsFromMe": false,
      "IsGroup": false,
      "ID": "3EB0C5A277F7F9B6C599",
      "Timestamp": "2025-11-11T10:00:00Z"
    },
    "count": 50
  }'
```

---

## Fluxos de Uso Comuns

### Organizar Conversas Prioritárias

```bash
# 1. Fixar conversas importantes
curl -X POST http://localhost:4000/chat/pin \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{"chat": "5511999999999@s.whatsapp.net"}'

# 2. Arquivar conversas antigas
curl -X POST http://localhost:4000/chat/archive \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{"chat": "5511888888888@s.whatsapp.net"}'
```

### Gerenciar Notificações

```bash
# Silenciar grupo temporariamente (1 hora)
curl -X POST http://localhost:4000/chat/mute \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{"chat": "120363XXXXXXXXXX@g.us"}'

# Reativar notificações
curl -X POST http://localhost:4000/chat/unmute \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{"chat": "120363XXXXXXXXXX@g.us"}'
```

### Limpar Lista de Chats

```bash
# Arquivar múltiplas conversas
CHATS=("5511111111111@s.whatsapp.net" "5511222222222@s.whatsapp.net" "5511333333333@s.whatsapp.net")

for chat in "\${CHATS[@]}"; do
  curl -X POST http://localhost:4000/chat/archive \
    -H "Content-Type: application/json" \
    -H "apikey: SUA-CHAVE-API" \
    -d "{"chat": "$chat"}"
done
```

---

## Códigos de Erro Comuns

| Código | Erro | Solução |
|--------|------|---------|
| 400 | `chat is required` | Forneça o campo `chat` |
| 500 | `instance not found` | Instância não conectada |
| 500 | `invalid phone number` | JID inválido (formato incorreto) |

---

## Boas Práticas

### 1. Validar JID
Sempre use JIDs no formato correto:
- Chat individual: `5511999999999@s.whatsapp.net`
- Grupo: `120363XXXXXXXXXX@g.us`

### 2. Gerenciar Estado
Mantenha controle do estado dos chats na sua aplicação:
- Registre quais chats estão fixados, arquivados e silenciados
- Lembre-se: um chat não pode estar fixado e arquivado ao mesmo tempo
- Ao fixar um chat, ele é automaticamente desarquivado pelo WhatsApp

### 3. Silenciamento Temporário
Lembre-se que o mute dura apenas 1 hora. Para silenciamento permanente:
- Use o aplicativo WhatsApp nativo
- Ou implemente lógica para re-silenciar periodicamente

### 4. Limite de Chats Fixados
O WhatsApp permite fixar até **3 chats** no topo. Não há validação na API, mas isso é uma limitação do WhatsApp.

### 5. Sincronização de Histórico
Use com cuidado - sincronizar histórico consome recursos. Recomendações:
- Sincronize no máximo **100 mensagens** por vez
- Aguarde resposta antes de nova sincronização
- Use apenas quando necessário (nova instalação, restauração)

---

## Diferenças de Comportamento

### Fixar vs Arquivar
- **Fixar**: Mantém conversa no topo (máx 3)
- **Arquivar**: Remove da lista principal, mas mantém notificações

### Mute vs Archive
- **Mute**: Silencia notificações, mas chat permanece visível
- **Archive**: Oculta chat, mas notificações continuam (a menos que também esteja mutado)

### Para Combinar
```bash
# Arquivar E silenciar (para "esconder" completamente)
curl -X POST http://localhost:4000/chat/archive ...
curl -X POST http://localhost:4000/chat/mute ...

# Fixar (automaticamente desarquiva)
curl -X POST http://localhost:4000/chat/pin ...
```

---

## Próximos Passos

- [API de Labels](./api-labels.md) - Organizar chats com etiquetas
- [API de Mensagens](./api-messages.md) - Gerenciar mensagens
- [API de Grupos](./api-groups.md) - Gerenciar grupos
- [Visão Geral da API](./api-overview.md)

---

**Documentação gerada para WhatyGo v1.0**
