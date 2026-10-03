# 🚀 Guia Rápido de Comandos

Comandos mais usados no dia a dia de desenvolvimento do WhatyGo.

## 📋 Comandos Essenciais

### Desenvolvimento

```bash
# Ver todos os comandos disponíveis
make help

# Rodar em modo desenvolvimento
make dev

# Rodar com hot reload (recarrega automaticamente ao salvar)
make watch
```

### Build e Instalação

```bash
# Compilar a aplicação
make build

# Compilar para todas as plataformas (Linux, Windows, macOS)
make build-all

# Instalar no GOPATH
make install
```

### Testes

```bash
# Rodar todos os testes
make test

# Rodar testes com relatório de cobertura
make test-coverage

# Rodar testes com verificação de race conditions
make test-race

# Rodar benchmarks
make bench
```

### Qualidade de Código

```bash
# Formatar código
make fmt

# Executar linter
make lint

# Executar go vet
make vet

# Executar todas as verificações (fmt + vet + lint + test)
make check
```

### Docker

```bash
# Build da imagem Docker
make docker-build

# Rodar container Docker
make docker-run

# Iniciar todos os serviços com docker-compose
make docker-compose-up

# Parar todos os serviços
make docker-compose-down

# Ver logs do docker-compose
make docker-compose-logs
```

### Banco de Dados

```bash
# Executar migrations
make migrate-up

# Reverter migrations
make migrate-down
```

### Documentação

```bash
# Gerar documentação Swagger
make swagger

# Abrir documentação local
make docs
```

### Dependências

```bash
# Instalar dependências
make deps

# Atualizar dependências
make deps-update

# Limpar dependências não utilizadas
make deps-clean
```

### Limpeza

```bash
# Remover arquivos de build
make clean

# Limpeza completa (incluindo cache do Go)
make clean-all
```

## 🎯 Workflows Comuns

### Setup Inicial

```bash
# 1. Clone o repositório
git clone https://github.com/lucasgiovannibr/evolution-go.git
cd evolution-go

# 2. Setup completo do ambiente
make setup

# 3. Configure o .env
cp .env.example .env
# Edite o .env com suas configurações

# 4. Rode em modo desenvolvimento
make dev
```

### Desenvolvimento Diário

```bash
# Rodar com hot reload
make watch

# Em outro terminal, rodar testes em watch mode
# (requer: go install github.com/cespare/reflex@latest)
reflex -r '\.go$' -s -- make test
```

### Antes de Fazer Commit

```bash
# Executar todas as verificações
make check

# Se tudo passar, commit e push
git add .
git commit -m "feat: sua feature"
git push
```

### Preparar para Deploy

```bash
# 1. Rodar todos os testes
make test

# 2. Build da imagem Docker
make docker-build

# 3. Testar localmente
make docker-run

# 4. Se tudo ok, fazer tag e push
docker tag whatygo:latest seu-registry/whatygo:v1.0.0
docker push seu-registry/whatygo:v1.0.0
```

### Debug e Profiling

```bash
# Ver status da aplicação
make status

# Ver logs em tempo real
make logs

# Profile de CPU (aplicação deve estar rodando)
make profile-cpu

# Profile de memória (aplicação deve estar rodando)
make profile-mem

# Ver informações de versão
make version
```

## 🔧 Ferramentas Opcionais

Instale estas ferramentas para habilitar recursos adicionais:

```bash
# Hot reload
go install github.com/cosmtrek/air@latest

# Linter
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Swagger generator
go install github.com/swaggo/swag/cmd/swag@latest

# Watch mode para testes
go install github.com/cespare/reflex@latest
```

## 💡 Dicas

1. **Use `make help`** sempre que esquecer um comando
2. **Use `make watch`** durante desenvolvimento para hot reload automático
3. **Use `make check`** antes de fazer commit
4. **Use `make setup`** em uma nova máquina para configurar tudo de uma vez
5. **Configure seu IDE** para rodar `make fmt` ao salvar arquivos

## 📚 Documentação Completa

Para documentação completa da API e guias detalhados:

- **[Documentação Oficial](./docs/wiki/README.md)** - Guias, tutoriais e referência
- **[API Swagger](http://localhost:4000/swagger/index.html)** - Documentação interativa (quando servidor estiver rodando)

## ❓ Problemas Comuns

### "make: command not found"
- **macOS**: `xcode-select --install`
- **Ubuntu/Debian**: `sudo apt-get install build-essential`
- **Windows**: Use WSL ou instale Make para Windows

### "air: command not found" ao usar `make watch`
```bash
go install github.com/cosmtrek/air@latest
```

### "golangci-lint: command not found" ao usar `make lint`
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Porta 4000 já em uso
```bash
# Mude a porta no .env
SERVER_PORT=8080
```

---

**Dica Final**: Rode `make help` para ver a lista completa e atualizada de comandos!

