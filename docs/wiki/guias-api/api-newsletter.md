# API de Newsletters

Documentação dos endpoints para gerenciar Newsletters (Canais) do WhatsApp.

## 📋 Índice

- [Criar Newsletter](#criar-newsletter)
- [Listar Newsletters Inscritas](#listar-newsletters-inscritas)
- [Informações da Newsletter](#informações-da-newsletter)
- [Obter Newsletter por Link](#obter-newsletter-por-link)
- [Inscrever em Newsletter](#inscrever-em-newsletter)
- [Listar Mensagens](#listar-mensagens)

---

## O que são Newsletters?

**Newsletters** (também chamadas de **Canais**) são um recurso do WhatsApp para transmissão unidirecional de mensagens. Similar a um canal de broadcast.

**Características**:
- **Transmissão unidirecional**: Apenas admins postam, seguidores apenas leem
- **Sem limite de seguidores**: Diferente de grupos (1024) ou listas de transmissão (256)
- **Privacidade**: Seguidores não veem quem mais segue o canal
- **Reações**: Seguidores podem reagir às mensagens
- **Conteúdo público ou privado**: Configurável

---

## Criar Newsletter

Cria um novo canal (newsletter) no WhatsApp.

**Endpoint**: `POST /newsletter/create`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "name": "Notícias da Empresa",
  "description": "Canal oficial de comunicados e novidades"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `name` | string | ✅ Sim | Nome do canal |
| `description` | string | ❌ Não | Descrição do canal |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "ID": "120363XXXXXXXXXX@newsletter",
    "Name": "Notícias da Empresa",
    "Description": "Canal oficial de comunicados e novidades",
    "SubscriberCount": 0,
    "CreationTime": 1699000000,
    "Settings": {
      "ReactionCodes": ["👍", "❤️", "😂", "😮", "😢", "🙏"]
    },
    "ThreadMetadata": {
      "CreationTime": 1699000000
    }
  }
}
```

**Campos da Resposta**:
- `ID`: JID único do canal (formato @newsletter)
- `Name`: Nome do canal
- `Description`: Descrição
- `SubscriberCount`: Número de seguidores
- `Settings.ReactionCodes`: Reações permitidas

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/newsletter/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "name": "Canal de Ofertas",
    "description": "Promoções e ofertas exclusivas"
  }'
```

---

## Listar Newsletters Inscritas

Lista todos os canais que você está seguindo.

**Endpoint**: `GET /newsletter/list`

**Headers**:
```
apikey: SUA-CHAVE-API
```

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": [
    {
      "ID": "120363XXXXXXXXXX@newsletter",
      "Name": "Canal de Ofertas",
      "Description": "Promoções exclusivas",
      "SubscriberCount": 1523,
      "Role": "SUBSCRIBER",
      "Settings": {
        "ReactionCodes": ["👍", "❤️", "😂"]
      }
    },
    {
      "ID": "120363YYYYYYYYYY@newsletter",
      "Name": "Notícias Tech",
      "Description": "Últimas novidades em tecnologia",
      "SubscriberCount": 5420,
      "Role": "OWNER",
      "Settings": {
        "ReactionCodes": ["👍", "❤️"]
      }
    }
  ]
}
```

**Roles Possíveis**:
- `OWNER`: Você é o criador/dono do canal
- `ADMIN`: Você é administrador
- `SUBSCRIBER`: Você é apenas seguidor

**Exemplo cURL**:
```bash
curl -X GET http://localhost:4000/newsletter/list \
  -H "apikey: SUA-CHAVE-API"
```

---

## Informações da Newsletter

Obtém informações detalhadas de um canal específico.

**Endpoint**: `POST /newsletter/info`

**Body**:
```json
{
  "jid": "120363XXXXXXXXXX@newsletter"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string (JID) | ✅ Sim | JID do canal |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "ID": "120363XXXXXXXXXX@newsletter",
    "State": "ACTIVE",
    "Name": "Canal de Ofertas",
    "Description": "Promoções exclusivas",
    "SubscriberCount": 1523,
    "VerificationState": "VERIFIED",
    "Role": "SUBSCRIBER",
    "Settings": {
      "ReactionCodes": ["👍", "❤️", "😂", "😮", "😢", "🙏"]
    },
    "ThreadMetadata": {
      "CreationTime": 1699000000
    },
    "ViewerMetadata": {
      "MuteState": "UNMUTED",
      "ViewerRole": "SUBSCRIBER"
    }
  }
}
```

**Campos Adicionais**:
- `State`: ACTIVE, SUSPENDED, GEOSUSPENDED
- `VerificationState`: VERIFIED (canal verificado) ou não
- `ViewerMetadata.MuteState`: MUTED ou UNMUTED

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/newsletter/info \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "120363XXXXXXXXXX@newsletter"
  }'
```

---

## Obter Newsletter por Link

Obtém informações de um canal através do link de convite.

**Endpoint**: `POST /newsletter/link`

**Body**:
```json
{
  "key": "ABC123XYZ"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `key` | string | ✅ Sim | Código do link de convite |

**Nota**: O `key` é a parte após `https://whatsapp.com/channel/` no link do canal.

**Exemplo de Link**:
```
https://whatsapp.com/channel/ABC123XYZ
                              ↑
                          Este é o key
```

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "ID": "120363XXXXXXXXXX@newsletter",
    "Name": "Canal Público",
    "Description": "Nosso canal oficial",
    "SubscriberCount": 2547,
    "VerificationState": "VERIFIED",
    "Settings": {
      "ReactionCodes": ["👍", "❤️"]
    }
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/newsletter/link \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "key": "ABC123XYZ"
  }'
```

---

## Inscrever em Newsletter

Inscreve-se (segue) um canal do WhatsApp.

**Endpoint**: `POST /newsletter/subscribe`

**Body**:
```json
{
  "jid": "120363XXXXXXXXXX@newsletter"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string (JID) | ✅ Sim | JID do canal |

**Nota**: Para obter o JID, primeiro use `/newsletter/link` com o código do convite.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/newsletter/subscribe \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "120363XXXXXXXXXX@newsletter"
  }'
```

---

## Listar Mensagens

Obtém mensagens de um canal.

**Endpoint**: `POST /newsletter/messages`

**Body**:
```json
{
  "jid": "120363XXXXXXXXXX@newsletter",
  "count": 20,
  "before_id": 0
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string (JID) | ✅ Sim | JID do canal |
| `count` | int | ❌ Não | Número de mensagens (padrão: 20, máx: 100) |
| `before_id` | int | ❌ Não | ID da mensagem para paginação (0 = mais recentes) |

**Paginação**: Use o `ServerID` da última mensagem como `before_id` para buscar mensagens mais antigas.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": [
    {
      "MessageServerID": 12345,
      "ViewsCount": 1234,
      "Message": {
        "conversation": "Bem-vindos ao nosso canal!"
      },
      "ReactionCounts": {
        "👍": 45,
        "❤️": 89
      }
    },
    {
      "MessageServerID": 12344,
      "ViewsCount": 2156,
      "Message": {
        "imageMessage": {
          "url": "https://...",
          "caption": "Promoção imperdível!"
        }
      },
      "ReactionCounts": {
        "👍": 124,
        "😮": 23
      }
    }
  ]
}
```

**Campos da Resposta**:
- `MessageServerID`: ID único da mensagem
- `ViewsCount`: Número de visualizações
- `Message`: Conteúdo da mensagem (texto, imagem, etc)
- `ReactionCounts`: Contagem de cada reação

**Exemplo cURL**:
```bash
# Buscar últimas 20 mensagens
curl -X POST http://localhost:4000/newsletter/messages \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "120363XXXXXXXXXX@newsletter",
    "count": 20
  }'

# Buscar 20 mensagens anteriores (paginação)
curl -X POST http://localhost:4000/newsletter/messages \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "120363XXXXXXXXXX@newsletter",
    "count": 20,
    "before_id": 12340
  }'
```

---

## Fluxos Completos

### 1. Criar e Configurar Canal

```bash
# 1. Criar canal
NEWSLETTER_JID=$(curl -s -X POST http://localhost:4000/newsletter/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "name": "Ofertas Especiais",
    "description": "Promoções exclusivas para você"
  }' | jq -r '.data.ID')

echo "Canal criado: $NEWSLETTER_JID"

# 2. Obter informações
curl -X POST http://localhost:4000/newsletter/info \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d "{"jid": "$NEWSLETTER_JID"}"
```

### 2. Inscrever-se via Link

Para se inscrever em um canal usando um link de convite:

1. **Obter informações do canal**:
   - Use `POST /newsletter/link` com a `key` do convite
   - A resposta inclui nome, descrição, contador de seguidores e o JID do canal

2. **Inscrever-se**:
   - Use `POST /newsletter/subscribe` com o JID obtido
   - Se retornar status 200, a inscrição foi bem-sucedida

### 3. Ler Mensagens Recentes

Para buscar mensagens publicadas em um canal:

1. Use `POST /newsletter/messages` com:
   - `jid`: JID do canal
   - `count`: Quantas mensagens buscar (ex: 20)
   - `before_id`: ID da mensagem anterior (use 0 para as mais recentes)

2. A resposta inclui para cada mensagem:
   - `MessageServerID`: ID da mensagem
   - `ViewsCount`: Número de visualizações
   - `Message`: Conteúdo da mensagem
   - `ReactionCounts`: Contadores de reações por emoji

---

## Casos de Uso

### 1. Empresa / Marca

- **Lançamentos**: Anunciar novos produtos
- **Promoções**: Divulgar ofertas exclusivas
- **Novidades**: Comunicados corporativos
- **Blog**: Compartilhar artigos

### 2. Influencer / Criador de Conteúdo

- **Conteúdo exclusivo**: Posts para seguidores
- **Bastidores**: Fotos e vídeos dos bastidores
- **Anúncios**: Novos vídeos, lives, eventos

### 3. Mídia / Jornalismo

- **Notícias**: Breaking news
- **Análises**: Artigos de opinião
- **Entretenimento**: Conteúdo exclusivo

### 4. Educação

- **Dicas**: Conteúdo educativo
- **Aulas**: Links para aulas ao vivo
- **Materiais**: Compartilhar PDFs, links

---

## Enviar Mensagens no Canal

Para enviar mensagens no seu canal, use a **API de Mensagens** normal, passando o JID do canal:

```bash
curl -X POST http://localhost:4000/send/text \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "number": "120363XXXXXXXXXX@newsletter",
    "text": "🔥 Promoção Relâmpago! 50% OFF em todos os produtos!"
  }'
```

Funciona com todos os tipos de mensagem:
- Texto
- Imagem
- Vídeo
- Documento
- Localização
- Poll (enquete)

---

## Limitações e Observações

### Permissões

- **Criar canal**: Qualquer usuário
- **Postar**: Apenas OWNER e ADMIN
- **Ver mensagens**: Todos os seguidores
- **Reagir**: Todos os seguidores (se permitido)

### Limitações do WhatsApp

| Aspecto | Limitação |
|---------|-----------|
| Número de seguidores | Ilimitado |
| Mensagens por dia | Não documentado |
| Tamanho da descrição | ~255 caracteres |
| Reações | Até 7 emojis configuráveis |

### Diferenças: Newsletter vs Grupo vs Lista

| Aspecto | Newsletter | Grupo | Lista de Transmissão |
|---------|-----------|-------|---------------------|
| Seguidores | Ilimitado | 1024 | 256 |
| Comunicação | Unidirecional | Bidirecional | Unidirecional |
| Privacidade | Anônimo | Todos veem todos | Anônimo |
| Reações | Sim | Sim | Não |
| Respostas | Não | Sim | Sim (privado) |

---

## Códigos de Erro Comuns

| Código | Erro | Solução |
|--------|------|---------|
| 400 | `name is required` | Forneça o nome do canal |
| 400 | `jid is required` | Forneça o JID do canal |
| 400 | `key is required` | Forneça o código do link |
| 500 | `instance not found` | Instância não conectada |
| 500 | `error create newsletter` | Falha ao criar (tente nome diferente) |
| 500 | `error list newsletter` | Falha ao buscar canais |

---

## Boas Práticas

### 1. Nome e Descrição Claros

```json
{
  "name": "Tech News Daily",
  "description": "Últimas notícias de tecnologia todos os dias às 9h"
}
```

### 2. Consistência de Postagens

- Defina frequência (diária, semanal)
- Mantenha horários regulares
- Avise se houver pausas

### 3. Conteúdo de Qualidade

- Textos concisos
- Imagens de alta qualidade
- Informações relevantes
- Evite spam

### 4. Engajamento

- Use enquetes
- Permita reações relevantes
- Responda dúvidas (em outros canais)

### 5. Divulgação

Compartilhe o link do canal:
```
https://whatsapp.com/channel/ABC123XYZ
```

---

## Próximos Passos

- [API de Mensagens](./api-messages.md) - Enviar conteúdo no canal
- [API de Grupos](./api-groups.md) - Comparar com grupos
- [API de Comunidades](./api-community.md) - Organizar múltiplos canais
- [Visão Geral da API](./api-overview.md)

---

**Documentação gerada para WhatyGo v1.0**
