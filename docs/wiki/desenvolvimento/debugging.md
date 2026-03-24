# Debugging e Troubleshooting

Guia para resolver problemas comuns no Evolution GO.

## Índice

- [Logs e Debug Mode](#logs-e-debug-mode)
- [Problemas Comuns](#problemas-comuns)
- [Debugging com IDE](#debugging-com-ide)
- [Profiling de Performance](#profiling-de-performance)
- [Análise de Banco de Dados](#análise-de-banco-de-dados)
- [Network Debugging](#network-debugging)
- [Ferramentas Úteis](#ferramentas-úteis)

---

## Logs e Debug Mode

### Habilitar Debug Mode

```env
# .env
WADEBUG=DEBUG
LOGTYPE=console  # ou 'file'
```

### Visualizar Logs

```bash
# Logs em console
make dev

# Logs em arquivo
tail -f logs/evolution-go.log

# Docker
docker-compose logs -f evolution-go

# Filtrar logs de erro
docker-compose logs evolution-go | grep ERROR
```

### Níveis de Log

- **DEBUG**: Detalhes de execução (desenvolvimento)
- **INFO**: Informações gerais (produção)
- **WARN**: Avisos (produção)
- **ERROR**: Erros (sempre)

---

## Problemas Comuns

### 1. Erro: "port 4000 already in use"

**Causa**: Outra aplicação usando a porta 4000.

**Solução**:

```bash
# Verificar processo usando a porta
lsof -i :4000

# Ou
netstat -tuln | grep 4000

# Matar processo
kill -9 <PID>

# Ou alterar porta no .env
SERVER_PORT=4001
```

### 2. Erro: "connection refused" (PostgreSQL)

**Causa**: PostgreSQL não está rodando ou inacessível.

**Solução**:

```bash
# Verificar se PostgreSQL está rodando
sudo systemctl status postgresql

# Ou via Docker
docker ps | grep postgres

# Iniciar PostgreSQL
sudo systemctl start postgresql

# Ou via Docker
docker start postgres

# Testar conexão
psql -h localhost -U postgres -d evogo_auth
```

### 3. Erro: "database does not exist"

**Causa**: Databases `evogo_auth` ou `evogo_users` não criados.

**Solução**:

```bash
# Criar databases
sudo -u postgres psql << EOF
CREATE DATABASE evogo_auth;
CREATE DATABASE evogo_users;
EOF

# Ou via Docker
docker exec -i postgres psql -U postgres << EOF
CREATE DATABASE evogo_auth;
CREATE DATABASE evogo_users;
EOF
```

### 4. Erro: "invalid API key"

**Causa**: API key incorreta ou não configurada.

**Solução**:

```bash
# Verificar .env
cat .env | grep GLOBAL_API_KEY

# Testar com API key correta
curl -H "apikey: SUA-CHAVE" http://localhost:4000/server/ok
```

### 5. QR Code não Aparece

**Causa**: Instância já conectada ou erro de criação.

**Solução**:

```bash
# Verificar logs
docker-compose logs -f evolution-go | grep QR

# Deletar e recriar instância
curl -X DELETE http://localhost:4000/instance/delete/NOME \
  -H "apikey: SUA-CHAVE"

curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE" \
  -d '{"instanceName": "NOME"}'
```

### 6. Webhook não Funciona

**Causa**: URL inválida ou endpoint indisponível.

**Solução**:

```bash
# Testar webhook manualmente
curl -X POST https://seu-webhook.com/endpoint \
  -H "Content-Type: application/json" \
  -d '{"test": true}'

# Verificar logs
docker-compose logs evolution-go | grep webhook

# Verificar configuração
curl http://localhost:4000/instance/connectionState/NOME \
  -H "apikey: SUA-CHAVE"
```

### 7. Mensagem não Envia

**Causa**: Instância desconectada, número inválido, ou erro de rede.

**Solução**:

```bash
# Verificar conexão
curl http://localhost:4000/instance/connectionState/NOME \
  -H "apikey: SUA-CHAVE"

# Verificar logs
docker-compose logs -f evolution-go

# Verificar formato do número
# Correto: 5511999999999 (DDI + DDD + número)
# Errado: 11999999999, +55 11 99999-9999
```

---

## Debugging com IDE

### VSCode

1. Abrir `Run and Debug` (Ctrl+Shift+D)
2. Selecionar "Launch Evolution GO"
3. Adicionar breakpoints (F9)
4. Iniciar debug (F5)

**launch.json**:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Evolution GO",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/evolution-go",
      "args": ["-dev"],
      "showLog": true
    }
  ]
}
```

### GoLand

1. **Run → Debug 'Evolution GO'**
2. Adicionar breakpoints (Ctrl+F8)
3. Debug com F5

---

## Profiling de Performance

### CPU Profiling

```bash
# Executar com pprof
go run cmd/evolution-go/main.go -dev &
PID=$!

# Gerar CPU profile (30 segundos)
curl http://localhost:4000/debug/pprof/profile?seconds=30 > cpu.prof

# Analisar
go tool pprof cpu.prof

# Comandos no pprof:
# top10 - Top 10 funções
# list <func> - Código da função
# web - Visualização gráfica (requer graphviz)
```

### Memory Profiling

```bash
# Heap snapshot
curl http://localhost:4000/debug/pprof/heap > heap.prof

# Analisar
go tool pprof heap.prof

# Visualizar em browser
go tool pprof -http=:8081 heap.prof
```

### Goroutine Leaks

```bash
# Ver goroutines ativas
curl http://localhost:4000/debug/pprof/goroutine?debug=1

# Profile de goroutines
curl http://localhost:4000/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof
```

---

## Análise de Banco de Dados

### Queries Lentas

```sql
-- Habilitar log de queries lentas (PostgreSQL)
ALTER DATABASE evogo_users SET log_min_duration_statement = 1000;

-- Ver queries lentas
SELECT query, calls, total_time, mean_time
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;
```

### Ver Conexões Ativas

```sql
-- PostgreSQL
SELECT count(*) FROM pg_stat_activity;

SELECT datname, usename, state, query
FROM pg_stat_activity
WHERE datname IN ('evogo_auth', 'evogo_users');
```

### Índices Faltantes

```sql
-- Verificar scans sequenciais (ruim)
SELECT schemaname, tablename, seq_scan, seq_tup_read
FROM pg_stat_user_tables
WHERE seq_scan > 0
ORDER BY seq_tup_read DESC
LIMIT 10;
```

---

## Network Debugging

### Testar Conectividade

```bash
# Testar se porta está aberta
nc -zv localhost 4000

# Testar endpoint
curl -v http://localhost:4000/server/ok

# Com proxy
curl -x http://proxy:8080 http://localhost:4000/server/ok
```

### Capturar Tráfego HTTP

```bash
# tcpdump
sudo tcpdump -i any -A 'port 4000'

# Wireshark
# Filtro: tcp.port == 4000
```

---

## Ferramentas Úteis

### 1. delve (Go Debugger)

```bash
# Instalar
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug cmd/evolution-go/main.go -- -dev

# Comandos:
# break main.main - Breakpoint
# continue - Continuar
# next - Próxima linha
# print var - Imprimir variável
```

### 2. Postman/Insomnia

Importar collection Swagger:
- URL: http://localhost:4000/swagger/doc.json

### 3. pgAdmin / DBeaver

Conectar ao PostgreSQL:
- Host: localhost
- Port: 5432
- Database: evogo_users
- Username: postgres

### 4. Docker Stats

```bash
# Ver uso de recursos
docker stats evolution-go

# Inspecionar container
docker inspect evolution-go
```

---

## Checklist de Troubleshooting

Quando algo não funciona:

1. ✅ Verificar logs: `docker-compose logs -f`
2. ✅ Verificar .env: variáveis corretas?
3. ✅ Verificar PostgreSQL: está rodando?
4. ✅ Verificar porta: 4000 livre?
5. ✅ Verificar API key: está correta?
6. ✅ Testar health check: `/server/ok`
7. ✅ Verificar Swagger: documentação atualizada?
8. ✅ Verificar versão Go: 1.24+?

---

## Recursos Adicionais

- **Go Debugging**: https://go.dev/doc/gdb
- **GORM Debugging**: https://gorm.io/docs/logger.html
- **Gin Debugging**: https://gin-gonic.com/docs/examples/

---

**Mantido por**: Equipe EvoAI Services
