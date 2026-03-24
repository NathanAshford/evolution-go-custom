# Início Rápido

Tutorial prático para configurar sua primeira instância WhatsApp e enviar mensagens via Evolution GO.

## Pré-requisitos

- Evolution GO instalado e em execução
- Chave de API configurada (GLOBAL_API_KEY)
- Smartphone com WhatsApp ativo

Se ainda não instalou, consulte o [Guia de Instalação](./installation.md).

---

## 1. Verificar API

Confirme que o servidor está respondendo:

```bash
curl http://localhost:4000/server/ok
```

**Resposta esperada:**
```json
{
  "status": "ok"
}
```

---

## 2. Criar Instância

Uma instância representa uma conexão única do WhatsApp. Crie sua primeira:

```bash
curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-GLOBAL-API-KEY" \
  -d '{
    "name": "minha-instancia",
    "token": "token-secreto-da-instancia"
  }'
```

> **Importante:**
> - `apikey` header: Use a `GLOBAL_API_KEY` configurada no servidor
> - `token`: Crie um token único para esta instância (será usado para autenticar requisições desta instância)
> - `name`: Nome identificador da instância

**Resposta:**
```json
{
  "message": "success",
  "data": {
    "instanceId": "uuid-gerado",
    "name": "minha-instancia",
    "token": "token-secreto-da-instancia",
    "status": "created"
  }
}
```

A instância foi criada mas ainda não está autenticada no WhatsApp.

---

## 3. Autenticar via QR Code

### Iniciar Conexão

```bash
curl -X POST http://localhost:4000/instance/connect \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia"
```

> **Nota:** Agora use o `token` da instância no header `apikey` (não mais a GLOBAL_API_KEY).

### Obter QR Code

**Via navegador:**
```
http://localhost:4000/instance/qr
```
(com o header `apikey: token-secreto-da-instancia`)

**Via API:**
```bash
curl "http://localhost:4000/instance/qr" \
  -H "apikey: token-secreto-da-instancia"
```

### Escanear no WhatsApp

1. Abra o WhatsApp no smartphone
2. Acesse **Mais opções (⋮)** → **Aparelhos conectados**
3. Toque em **Conectar um aparelho**
4. Escaneie o QR Code exibido

A conexão deve ser estabelecida em aproximadamente 10 segundos.

---

## 4. Verificar Status

Confirme que a instância está conectada:

```bash
curl "http://localhost:4000/instance/status" \
  -H "apikey: token-secreto-da-instancia"
```

**Resposta quando conectado:**
```json
{
  "message": "success",
  "data": {
    "instanceId": "uuid-da-instancia",
    "name": "minha-instancia",
    "status": "open",
    "profilePictureUrl": "https://...",
    "profileName": "Nome do Perfil"
  }
}
```

**Status possíveis:**
- `created` - Instância criada, aguardando autenticação
- `connecting` - Processo de conexão em andamento
- `open` - Conectado e operacional
- `close` - Desconectado

---

## 5. Enviar Mensagem

### Mensagem de Texto

```bash
curl -X POST http://localhost:4000/send/text \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia" \
  -d '{
    "number": "5511999999999",
    "text": "Mensagem de teste via Evolution GO"
  }'
```

> **Nota:** A instância é identificada pelo token no header `apikey`. Não é necessário enviar `instanceName` no body.

**Formato do número:**
- DDI + DDD + Número (apenas dígitos)
- Exemplo Brasil: `5511999999999` (55=Brasil, 11=São Paulo)

**Resposta:**
```json
{
  "message": "success",
  "data": {
    "messageId": "3EB0...",
    "status": "sent"
  }
}
```

### Mensagem com Mídia

```bash
curl -X POST http://localhost:4000/send/media \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia" \
  -d '{
    "number": "5511999999999",
    "mediaUrl": "https://exemplo.com/imagem.jpg",
    "caption": "Legenda da imagem"
  }'
```

### Mensagem com Documento

```bash
curl -X POST http://localhost:4000/send/media \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia" \
  -d '{
    "number": "5511999999999",
    "mediaUrl": "https://exemplo.com/documento.pdf",
    "fileName": "relatorio.pdf"
  }'
```

---

## Operações de Gerenciamento

### Listar Instâncias

```bash
curl "http://localhost:4000/instance/all" \
  -H "apikey: SUA-GLOBAL-API-KEY"
```

> **Nota:** Use a `GLOBAL_API_KEY` para listar todas as instâncias.

### Detalhes da Instância

```bash
curl "http://localhost:4000/instance/info/uuid-da-instancia" \
  -H "apikey: SUA-GLOBAL-API-KEY"
```

> **Nota:** Use o `instanceId` (UUID) da instância na URL.

### Desconectar

```bash
curl -X POST http://localhost:4000/instance/disconnect \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia"
```

### Reconectar (novo QR Code)

```bash
curl -X POST http://localhost:4000/instance/reconnect \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia"
```

### Deletar

```bash
curl -X DELETE "http://localhost:4000/instance/delete/uuid-da-instancia" \
  -H "apikey: SUA-GLOBAL-API-KEY"
```

> **Nota:** Use a `GLOBAL_API_KEY` e o `instanceId` (UUID) para deletar.

---

## Documentação Interativa (Swagger)

Acesse a interface Swagger para explorar e testar todos os endpoints:

```
http://localhost:4000/swagger/index.html
```

**Passos:**
1. Clique em **Authorize** no topo da página
2. Insira sua chave de API
3. Confirme a autenticação
4. Explore e teste os endpoints disponíveis

---

## Resolução de Problemas

### QR Code Expira

QR Code possui validade limitada (~60 segundos). Se expirar:

```bash
curl -X POST http://localhost:4000/instance/reconnect \
  -H "Content-Type: application/json" \
  -H "apikey: token-secreto-da-instancia"
```

### Instância Não Encontrada

Verifique se está usando o token correto:

```bash
# Listar todas as instâncias
curl "http://localhost:4000/instance/all" \
  -H "apikey: SUA-GLOBAL-API-KEY"
```

### Mensagem Não Entregue

**Verificações:**

1. **Status da conexão:**
```bash
curl "http://localhost:4000/instance/status" \
  -H "apikey: token-secreto-da-instancia"
```
Status deve ser `open`.

2. **Formato do número:**
   - Apenas dígitos (sem espaços, hífens ou símbolos)
   - Incluir DDI e DDD
   - Validar se o número existe no WhatsApp

3. **Logs do servidor:**
```bash
docker-compose logs -f evolution-go
```

### Erro de Autenticação

Verifique se está enviando o header correto:
```bash
-H "apikey: sua-chave-aqui"
```

Confirme a chave configurada no `docker-compose.yml` ou `.env`.

---

## Próximos Passos

### Documentação da API

- **[API de Mensagens](../guias-api/api-messages.md)** - Todos os tipos de mensagens
- **[API de Grupos](../guias-api/api-groups.md)** - Gerenciamento de grupos
- **[API de Usuários](../guias-api/api-users.md)** - Perfis e contatos
- **[Referência Completa](../guias-api/api-overview.md)** - Todos os endpoints

### Recursos Avançados

- **[Sistema de Eventos](../recursos-avancados/events-system.md)** - Webhooks e filas
- **[Webhooks](../recursos-avancados/webhooks.md)** - Receber notificações
- **[RabbitMQ](../recursos-avancados/rabbitmq.md)** - Processamento assíncrono
- **[MinIO/S3](../recursos-avancados/minio.md)** - Armazenamento de mídia

### Deploy e Segurança

- **[Configuração](./configuration.md)** - Variáveis de ambiente
- **[Docker Deployment](../deploy-producao/docker-deployment.md)** - Deploy em produção
- **[Segurança](../deploy-producao/security.md)** - Boas práticas de segurança

---

**Documentação Evolution GO v1.0**
