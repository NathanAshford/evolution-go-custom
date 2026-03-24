# Códigos de Erro

Referência de códigos de erro HTTP e mensagens da aplicação.

## Códigos HTTP

| Código | Status | Descrição |
|--------|--------|-----------|
| `200` | OK | Requisição processada com sucesso |
| `201` | Created | Recurso criado com sucesso |
| `400` | Bad Request | Requisição inválida ou malformada |
| `401` | Unauthorized | Autenticação ausente ou inválida |
| `403` | Forbidden | Acesso negado ao recurso |
| `404` | Not Found | Recurso não encontrado |
| `409` | Conflict | Conflito com estado atual do recurso |
| `500` | Internal Server Error | Erro interno não tratado |

---

## Erros de Autenticação

### 401 Unauthorized

```json
{
  "error": "Unauthorized"
}
```

**Causa**: Header `apikey` ausente ou valor inválido.

**Resolução**:
```bash
curl -H "apikey: SUA-CHAVE-API" http://localhost:4000/endpoint
```

---

## Erros de Instância

### 404 Instance Not Found

```json
{
  "error": "Instance not found"
}
```

**Causa**: Instância com nome especificado não existe.

**Resolução**: Verificar nome da instância ou criar nova via `POST /instance/create`.

### 409 Instance Already Exists

```json
{
  "error": "Instance already exists"
}
```

**Causa**: Tentativa de criar instância com nome duplicado.

**Resolução**: Utilizar nome único ou deletar instância existente.

### Instance Not Connected

```json
{
  "error": "Instance is not connected"
}
```

**Causa**: Operação requer instância conectada ao WhatsApp.

**Resolução**: Conectar instância via QR Code (`POST /instance/connect`) ou pairing code.

---

## Erros de Validação

### 400 Invalid Phone Number

```json
{
  "error": "Invalid phone number format"
}
```

**Causa**: Formato de número incorreto.

**Resolução**: Utilizar formato internacional sem símbolos.
- Correto: `5511999999999` (DDI + DDD + número)
- Incorreto: `+55 (11) 99999-9999`, `11999999999`

### 400 Required Field Missing

```json
{
  "error": "Field 'instanceName' is required"
}
```

**Causa**: Campo obrigatório ausente no body da requisição.

**Resolução**: Incluir campo obrigatório no payload JSON.

### 400 Invalid JSON

```json
{
  "error": "Invalid JSON"
}
```

**Causa**: Sintaxe JSON inválida ou header Content-Type incorreto.

**Resolução**:
- Validar sintaxe JSON
- Incluir header: `Content-Type: application/json`

---

## Erros de Infraestrutura

### 500 Database Connection Error

```json
{
  "error": "Database connection failed"
}
```

**Causa**: Falha ao conectar com PostgreSQL.

**Diagnóstico**:
```bash
# Verificar status do PostgreSQL
docker-compose ps postgres

# Verificar logs
docker-compose logs postgres

# Testar conectividade
docker-compose exec evolution-go nc -zv postgres 5432
```

**Resolução**:
- Verificar variáveis `POSTGRES_AUTH_DB` e `POSTGRES_USERS_DB`
- Confirmar que PostgreSQL está rodando
- Validar credenciais de acesso

### 500 Queue Connection Error

```json
{
  "error": "Failed to connect to message queue"
}
```

**Causa**: Falha ao conectar com RabbitMQ/NATS.

**Resolução**:
- Verificar variável `AMQP_URL` ou `NATS_URL`
- Confirmar que serviço de fila está rodando
- Validar credenciais e permissões

---

## Erros de Negócio

### User Not Found on WhatsApp

```json
{
  "error": "User not found on WhatsApp"
}
```

**Causa**: Número destinatário não está registrado no WhatsApp.

**Resolução**: Verificar número ou desabilitar validação (`CHECK_USER_EXISTS=false`).

### Message Send Failed

```json
{
  "error": "Failed to send message",
  "details": "..."
}
```

**Causas comuns**:
- Número bloqueou sua instância
- Mensagem violou políticas do WhatsApp
- Instância foi bloqueada/banida
- Problema de conectividade

**Diagnóstico**:
```bash
# Verificar logs detalhados
docker-compose logs -f evolution-go

# Verificar status da instância
curl -H "apikey: SUA-CHAVE" \
  "http://localhost:4000/instance/status?instanceName=NOME"
```

---

## Debugging

### Habilitar Logs Detalhados

```env
WADEBUG=DEBUG
```

### Verificar Logs

```bash
# Docker Compose
docker-compose logs -f evolution-go

# Docker (container específico)
docker logs -f evolution-go --tail=100

# Arquivo (se LOGTYPE=file)
tail -f logs/evolution-go.log
```

### Logs Estruturados

Logs incluem:
- **Timestamp**: Data/hora do evento
- **Level**: DEBUG, INFO, WARN, ERROR
- **Module**: Componente que gerou o log
- **Message**: Descrição do evento
- **Context**: Dados adicionais (instanceName, userId, etc)

---

## Códigos de Erro por Categoria

### Autenticação (401, 403)
- API key inválida
- Token expirado
- Permissões insuficientes

### Validação (400)
- JSON malformado
- Campos obrigatórios ausentes
- Formato inválido

### Recurso (404, 409)
- Instância não encontrada
- Recurso duplicado
- Estado inconsistente

### Servidor (500, 503)
- Erro de banco de dados
- Timeout de operação
- Serviço indisponível

---

## Recursos Adicionais

- **[Debugging](../desenvolvimento/debugging.md)** - Guia completo de debugging
- **[FAQ](./faq.md)** - Problemas comuns e soluções
- **[Logs](../fundamentos/configuration.md#logs-e-debug)** - Configuração de logs

---

**Documentação Evolution GO v1.0**
