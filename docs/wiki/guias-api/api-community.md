# API de Comunidades

Documentação dos endpoints para gerenciar Comunidades WhatsApp.

## 📋 Índice

- [Criar Comunidade](#criar-comunidade)
- [Adicionar Grupos](#adicionar-grupos)
- [Remover Grupos](#remover-grupos)

---

## O que são Comunidades?

**Comunidades** são um recurso do WhatsApp que permite agrupar vários grupos relacionados sob uma única estrutura. É como um "grupo de grupos".

**Características**:
- Uma comunidade pode conter **até 50 grupos**
- Cada comunidade tem um **canal de anúncios** (grupo pai)
- Membros da comunidade veem todos os grupos vinculados
- Facilita organização de organizações, escolas, condomínios, etc.

---

## Criar Comunidade

Cria uma nova comunidade WhatsApp.

**Endpoint**: `POST /community/create`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "communityName": "Escola ABC"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `communityName` | string | ✅ Sim | Nome da comunidade |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "JID": "120363XXXXXXXXXX@g.us",
    "OwnerJID": "5511999999999@s.whatsapp.net",
    "GroupName": {
      "Name": "Escola ABC",
      "NameSetAt": "2025-11-11T10:30:00Z",
      "NameSetBy": "5511999999999@s.whatsapp.net"
    },
    "GroupCreated": "2025-11-11T10:30:00Z",
    "GroupParent": {
      "IsParent": true
    }
  }
}
```

**Campos da Resposta**:
- `JID`: ID único da comunidade (formato de grupo @g.us)
- `OwnerJID`: Criador da comunidade
- `GroupName`: Nome e metadados
- `GroupParent.IsParent`: true (indica que é uma comunidade)

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/community/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "communityName": "Condomínio Residencial"
  }'
```

---

## Adicionar Grupos

Adiciona (vincula) grupos existentes a uma comunidade.

**Endpoint**: `POST /community/add`

**Body**:
```json
{
  "communityJid": "120363XXXXXXXXXX@g.us",
  "groupJid": [
    "120363YYYYYYYYYY@g.us",
    "120363ZZZZZZZZZZ@g.us"
  ]
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `communityJid` | string | ✅ Sim | JID da comunidade |
| `groupJid` | array | ✅ Sim | Array de JIDs de grupos para adicionar |

**Nota**: Os grupos devem **já existir** e você deve ser **admin** de ambos (comunidade e grupos).

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "success": [
      "120363YYYYYYYYYY@g.us",
      "120363ZZZZZZZZZZ@g.us"
    ],
    "failed": []
  }
}
```

**Campos da Resposta**:
- `success`: Array de grupos adicionados com sucesso
- `failed`: Array de grupos que falharam (não existe, sem permissão, etc)

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/community/add \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "communityJid": "120363XXXXXXXXXX@g.us",
    "groupJid": [
      "120363YYYYYYYYYY@g.us"
    ]
  }'
```

---

## Remover Grupos

Remove (desvincula) grupos de uma comunidade.

**Endpoint**: `POST /community/remove`

**Body**:
```json
{
  "communityJid": "120363XXXXXXXXXX@g.us",
  "groupJid": [
    "120363YYYYYYYYYY@g.us"
  ]
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `communityJid` | string | ✅ Sim | JID da comunidade |
| `groupJid` | array | ✅ Sim | Array de JIDs de grupos para remover |

**Nota**: Remover um grupo da comunidade **não deleta o grupo**, apenas o desvincula.

**Resposta de Sucesso (200)**:
```json
{
  "message": "success",
  "data": {
    "success": [
      "120363YYYYYYYYYY@g.us"
    ],
    "failed": []
  }
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/community/remove \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "communityJid": "120363XXXXXXXXXX@g.us",
    "groupJid": [
      "120363YYYYYYYYYY@g.us"
    ]
  }'
```

---

## Fluxo Completo de Gerenciamento

### 1. Criar Comunidade e Organizar Grupos

**Fluxo completo**:

1. **Criar a comunidade** usando `POST /community/create`
2. **Criar os grupos** que farão parte da comunidade usando `POST /group/create`
3. **Adicionar os grupos à comunidade** usando `POST /community/add` com os JIDs obtidos nos passos anteriores

Exemplo:
- Primeiro, crie uma comunidade "Escola Municipal" e guarde o `JID` retornado
- Depois, crie dois grupos "1º Ano A" e "2º Ano B" e guarde os JIDs
- Por fim, adicione os grupos à comunidade usando os JIDs

### 2. Reorganizar Comunidade

Para reorganizar uma comunidade (remover grupos antigos e adicionar novos):

1. Use `POST /community/remove` para remover grupos que não fazem mais parte
2. Use `POST /community/add` para adicionar novos grupos

Exemplo: Se você precisa substituir o grupo "120363OLD1@g.us" por dois novos grupos, primeiro remova o antigo e depois adicione os novos usando os respectivos endpoints.

---

## Casos de Uso

### 1. Escola / Universidade

```
Comunidade: "Universidade XYZ"
├── Grupo: Administração
├── Grupo: Engenharia - 1º Ano
├── Grupo: Engenharia - 2º Ano
├── Grupo: Medicina - 1º Ano
└── Grupo: Eventos
```

### 2. Condomínio

```
Comunidade: "Condomínio Residencial"
├── Grupo: Síndico e Zeladores
├── Grupo: Bloco A
├── Grupo: Bloco B
├── Grupo: Churrasqueira (Reservas)
└── Grupo: Comunicados Gerais
```

### 3. Empresa

```
Comunidade: "Empresa LTDA"
├── Grupo: Diretoria
├── Grupo: TI
├── Grupo: Vendas
├── Grupo: Marketing
└── Grupo: RH
```

### 4. Igreja / Instituição Religiosa

```
Comunidade: "Igreja Central"
├── Grupo: Liderança
├── Grupo: Jovens
├── Grupo: Mulheres
├── Grupo: Eventos
└── Grupo: Voluntários
```

---

## Permissões e Limitações

### Permissões Necessárias

Para gerenciar comunidades, você precisa ser:
1. **Criador** da comunidade (para adicionar/remover grupos)
2. **Admin** dos grupos que deseja vincular

### Limitações do WhatsApp

| Limitação | Valor |
|-----------|-------|
| Máximo de grupos por comunidade | 50 |
| Máximo de participantes por grupo | 1024 |
| Máximo de admins por grupo | Ilimitado |

### Comportamento

- **Desvincular grupo**: Não deleta o grupo, apenas remove da comunidade
- **Deletar comunidade**: Não deleta os grupos vinculados
- **Sair da comunidade**: Você sai de todos os grupos vinculados

---

## Códigos de Erro Comuns

| Código | Erro | Solução |
|--------|------|---------|
| 400 | `community name is required` | Forneça o nome da comunidade |
| 400 | `community jid is required` | Forneça o JID da comunidade |
| 400 | `group jid is required` | Forneça array de JIDs de grupos |
| 500 | `instance not found` | Instância não conectada |
| 500 | `error parse community jid` | JID inválido |
| 500 | `error link group` | Sem permissão ou grupo não existe |

---

## Boas Práticas

### 1. Planeje a Estrutura

Antes de criar, defina a organização:
```
Comunidade (Tema geral)
├── Grupo 1 (Categoria específica)
├── Grupo 2 (Categoria específica)
└── Grupo 3 (Categoria específica)
```

### 2. Nomeação Clara

Use nomes descritivos:
- ✅ "Escola ABC - 1º Ano A"
- ✅ "Condomínio XYZ - Bloco A"
- ❌ "Grupo 1"
- ❌ "ABC"

### 3. Canal de Anúncios

A comunidade em si funciona como canal de anúncios para todos os grupos. Use-a para:
- Comunicados gerais
- Eventos importantes
- Informações que afetam todos

### 4. Organize por Hierarquia

```
Comunidade (Nível mais alto)
└── Grupos (Nível específico)
    └── Participantes (Nível individual)
```

### 5. Limite de Grupos

Não exceda 50 grupos por comunidade. Se precisar de mais:
- Crie múltiplas comunidades
- Reorganize grupos por temas
- Arquive grupos inativos

---

## Diferenças: Comunidade vs Grupo

| Aspecto | Comunidade | Grupo |
|---------|------------|-------|
| Função | Agrupar grupos relacionados | Conversação direta |
| Limite de membros | 50 grupos × 1024 = 51.200 | 1024 |
| Estrutura | Hierárquica (pais) | Plana (participantes) |
| Mensagens | Canal de anúncios | Conversa completa |
| Admin | Admin da comunidade | Admin do grupo |

---

## Próximos Passos

- [API de Grupos](./api-groups.md) - Criar e gerenciar grupos
- [API de Mensagens](./api-messages.md) - Enviar anúncios
- [API de Usuários](./api-user.md) - Gerenciar contatos
- [Visão Geral da API](./api-overview.md)

---

**Documentação gerada para WhatyGo v1.0**
