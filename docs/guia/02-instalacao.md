# 2. Instalação

O caminho mais fácil: **Docker Compose**. Em cerca de 10 minutos você tem o servidor, o banco de dados e o painel funcionando. Foi testado com a imagem publicada do fork: o banco é criado sozinho, o painel abre e a API responde.

## Antes de começar

1. **Instale o Docker.**
   - Windows ou Mac: [Docker Desktop](https://www.docker.com/products/docker-desktop/).
   - Linux (servidor): `curl -fsSL https://get.docker.com | sh`
2. Confirme que funciona abrindo um terminal (no Windows: PowerShell) e digitando:

   ```bash
   docker compose version
   ```

   Deve aparecer um número de versão (2.x ou maior).

## Passo a passo

### 1. Baixe os arquivos

Com o Git instalado:

```bash
git clone https://github.com/lucasgiovannibr/whatygo.git
cd whatygo/docker/instalacao-simples
```

Sem o Git: na página do repositório, clique em **Code → Download ZIP**, extraia e abra a pasta `docker/instalacao-simples`.

Nessa pasta há dois arquivos que importam: `docker-compose.yml` (a receita) e `.env.example` (o modelo de configuração).

### 2. Crie o arquivo de configuração

Copie o modelo para um arquivo chamado `.env`:

```bash
cp .env.example .env
```

(No Windows PowerShell: `Copy-Item .env.example .env`.)

Abra o `.env` num editor de texto e preencha **duas** linhas:

| Linha | O que colocar |
|---|---|
| `POSTGRES_PASSWORD=` | Uma senha qualquer para o banco de dados. Só o servidor a usa. Evite espaços e símbolos como `@`, `:` e `/`. |
| `GLOBAL_API_KEY=` | A **chave de administrador**. Gere uma com 64 caracteres (veja abaixo). |

Gerar a chave:

```bash
openssl rand -hex 32
```

No Windows PowerShell, sem OpenSSL:

```powershell
-join ((48..57)+(97..102) | Get-Random -Count 64 | % {[char]$_})
```

> **Guarde essa chave num lugar seguro** (gerenciador de senhas). Ela é a "senha mestra": quem a tiver controla todos os números conectados.
>
> O servidor **se recusa a iniciar** com chaves de exemplo (`change-me`, `sua-chave-api-segura-aqui`...) justamente para ninguém subir um servidor com uma senha pública.

### 3. Suba o servidor

```bash
docker compose up -d
```

Na primeira vez ele baixa as imagens (alguns minutos). Quando terminar, confira:

```bash
docker compose ps
curl http://localhost:4000/health
```

A segunda linha deve responder `{"checks":{"authDb":"ok","usersDb":"ok"},"status":"ok"}`.

### 4. Abra o painel

No navegador: **http://localhost:4000/manager**

Se você mudou `HTTP_PORT` no `.env`, troque o `4000` pela porta escolhida.

### 5. Ative a licença

Veja a próxima seção.

## Ativar a licença

O projeto original exige que cada servidor seja **registrado** antes de funcionar. Este fork mantém esse mecanismo, porque ele faz parte do código herdado. Até registrar, a API responde `503` com `LICENSE_REQUIRED` (isso é normal na primeira vez).

1. Abra `http://localhost:4000/manager`.
2. Informe a **chave global** (`GLOBAL_API_KEY`) e clique em **Entrar**.
3. O painel leva você a uma página de registro, mantida pela Evolution Foundation. Complete o registro lá.
4. Você volta ao painel e a API passa a funcionar. O estado da licença fica salvo no banco de dados, e o servidor renova a ativação sozinho de tempos em tempos.

> O registro conversa com um serviço de terceiros (a Evolution Foundation). Se esse serviço estiver fora do ar, a ativação inicial pode falhar; tente de novo mais tarde.

## Atualizar para uma versão nova

```bash
cd whatygo/docker/instalacao-simples
git pull
docker compose pull
docker compose up -d
```

Antes de atualizar uma versão que mexa no banco (as notas ficam no [CHANGELOG](../../CHANGELOG.md)), **faça backup**:

```bash
docker compose exec postgres pg_dump -U postgres evogo_auth > backup-evogo_auth.sql
docker compose exec postgres pg_dump -U postgres evogo_users > backup-evogo_users.sql
```

> **Importante:** este fork atualizou a biblioteca do WhatsApp, e as migrações do banco só andam para frente. Quem volta para a imagem do projeto original depois de usar o fork **não consegue** reabrir o mesmo banco. Por isso, o backup antes de trocar.

## Parar, ligar e apagar

| O que fazer | Comando (na pasta `docker/instalacao-simples`) |
|---|---|
| Ver os logs | `docker compose logs -f evolution-go` |
| Parar (mantém os dados) | `docker compose stop` |
| Ligar de novo | `docker compose start` |
| Reiniciar | `docker compose restart evolution-go` |
| **Apagar tudo, inclusive os números conectados** | `docker compose down -v` |

## Colocar na internet (servidor de verdade)

Se for usar em um servidor (VPS):

1. Instale o Docker no servidor e repita os passos acima.
2. **Não exponha a porta 4000 diretamente.** Coloque na frente um proxy com HTTPS (Caddy, Nginx Proxy Manager, Traefik...) e um domínio, por exemplo `https://whaty.seudominio.com.br`.
3. O painel só consegue usar o microfone (para ligações) em página segura: `https` ou `localhost`.
4. Mantenha o firewall fechado para tudo que não for 80/443 (e SSH). A porta do banco **não** é publicada no arquivo simples, de propósito.

Mais sobre isso em [Configuração e segurança](./08-configuracao-e-seguranca.md).

## Outras formas de instalar

| Forma | Quando usar | Onde está |
|---|---|---|
| **Docker Compose simples** (este guia) | Quase todo mundo | `docker/instalacao-simples/` |
| Compose completo com RabbitMQ, NATS e MinIO | Quando precisa de filas ou armazenamento de mídia | `docker/examples/docker-compose.full.yml` |
| Docker Swarm | Vários servidores | `docker/examples/docker-compose.swarm.yml` |
| Compilar do código (Go) | Desenvolvedores | [Guia de desenvolvimento](../wiki/desenvolvimento/development-guide.md) |

> Os exemplos de `docker/examples/` também usam a imagem do WhatyGo (`ghcr.io/lucasgiovannibr/whatygo`). Quem tiver um arquivo antigo apontando para `evoapicloud/evolution-go` está usando a imagem do projeto original, **sem** as correções deste fork.

Próximo passo: [3. Primeiros passos](./03-primeiros-passos.md).
