# Visão Geral da API

Guia completo para usar a API REST do Evolution GO.

## Base URL

```
http://localhost:4000
```

## Autenticação

Todas as requisições precisam do header `apikey`:

```bash
curl -H "apikey: SUA-CHAVE-API" http://localhost:4000/endpoint
```

**Tipos de API Key**:
- **GLOBAL_API_KEY**: Configurada na variável de ambiente, usada para operações administrativas (criar/deletar instâncias)
- **Token da Instância**: Definido ao criar a instância, usado para operações da instância (enviar mensagens, conectar, etc)

## Formato das Requisições

### Content-Type

```
Content-Type: application/json
```

### Exemplo de Requisição

```bash
curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: GLOBAL_API_KEY" \
  -d '{
    "name": "teste",
    "token": "token-teste-123"
  }'
```

## Formato das Respostas

### Sucesso (HTTP 200/201)

```json
{
  "message": "success",
  "data": {
    "id": "uuid-abc-123",
    "name": "teste",
    "token": "token-teste-123",
    "connected": false
  }
}
```

### Erro (HTTP 4xx/5xx)

```json
{
  "error": "Instance not found",
  "message": "A instância especificada não existe"
}
```

## Códigos de Status HTTP

| Código | Significado |
|--------|-------------|
| 200 | OK - Requisição bem-sucedida |
| 201 | Created - Recurso criado com sucesso |
| 400 | Bad Request - Dados inválidos |
| 401 | Unauthorized - API key inválida |
| 404 | Not Found - Recurso não encontrado |
| 500 | Internal Server Error - Erro no servidor |

## Estrutura da API

A API está organizada em módulos:

### Instâncias
Gerenciar conexões WhatsApp
- `POST /instance/create` - Criar instância
- `POST /instance/connect` - Conectar via QR Code
- `GET /instance/status` - Ver status
- `DELETE /instance/delete/:id` - Deletar

### Mensagens
Enviar e gerenciar mensagens
- `POST /send/text` - Texto simples
- `POST /send/link` - Link com preview
- `POST /send/media` - Mídia (imagem, vídeo, áudio, documento)
- `POST /send/sticker` - Sticker/figurinha
- `POST /send/location` - Localização
- `POST /send/contact` - Contato
- `POST /send/poll` - Enquete
- `POST /send/button` - Botões interativos
- `POST /send/list` - Lista de opções

### Gerenciar Mensagens
Interagir com mensagens enviadas/recebidas
- `POST /message/react` - Reagir a mensagem
- `POST /message/markread` - Marcar como lida
- `POST /message/delete` - Deletar para todos
- `POST /message/edit` - Editar mensagem
- `POST /message/presence` - Definir presença (digitando, gravando)
- `POST /message/downloadmedia` - Baixar mídia
- `POST /message/status` - Ver status de entrega

### Usuários
Gerenciar perfil e contatos
- `POST /user/info` - Informações do usuário
- `POST /user/check` - Verificar se número existe
- `POST /user/avatar` - Obter avatar
- `GET /user/contacts` - Listar contatos
- `GET /user/privacy` - Ver configurações de privacidade
- `POST /user/privacy` - Configurar privacidade
- `POST /user/block` - Bloquear contato
- `POST /user/unblock` - Desbloquear contato
- `GET /user/blocklist` - Ver lista de bloqueados
- `POST /user/profilePicture` - Alterar foto de perfil
- `POST /user/profileName` - Alterar nome
- `POST /user/profileStatus` - Alterar status/recado

### Grupos
Gerenciar grupos WhatsApp
- `GET /group/list` - Listar todos os grupos
- `POST /group/create` - Criar grupo
- `POST /group/info` - Informações do grupo
- `POST /group/invitelink` - Obter link de convite
- `POST /group/photo` - Alterar foto do grupo
- `POST /group/name` - Alterar nome
- `POST /group/description` - Alterar descrição
- `POST /group/participant` - Gerenciar participantes (adicionar/remover/promover)
- `POST /group/join` - Entrar via link
- `POST /group/leave` - Sair do grupo

### Chats
Gerenciar conversas
- `POST /chat/pin` - Fixar conversa
- `POST /chat/unpin` - Desfixar conversa
- `POST /chat/archive` - Arquivar conversa
- `POST /chat/unarchive` - Desarquivar
- `POST /chat/mute` - Silenciar notificações
- `POST /chat/unmute` - Reativar notificações
- `POST /chat/history-sync` - Solicitar sincronização de histórico

### Labels (Etiquetas)
Organizar conversas com etiquetas
- `POST /label/chat` - Adicionar etiqueta a chat
- `POST /label/message` - Adicionar etiqueta a mensagem
- `POST /label/edit` - Criar/editar etiqueta
- `GET /label/list` - Listar todas as etiquetas
- `POST /unlabel/chat` - Remover etiqueta de chat
- `POST /unlabel/message` - Remover etiqueta de mensagem

### Chamadas
Gerenciar chamadas
- `POST /call/reject` - Rejeitar chamada

### Newsletter (Canais)
Gerenciar newsletters/canais
- `POST /newsletter/create` - Criar newsletter
- `GET /newsletter/list` - Listar newsletters
- `POST /newsletter/info` - Informações da newsletter
- `POST /newsletter/link` - Obter link de convite
- `POST /newsletter/subscribe` - Inscrever-se
- `POST /newsletter/messages` - Ver mensagens

### Community (Comunidades)
Gerenciar comunidades
- `POST /community/create` - Criar comunidade
- `POST /community/add` - Adicionar grupo à comunidade
- `POST /community/remove` - Remover grupo da comunidade

## Parâmetros Comuns

### number
Número do WhatsApp (DDI + DDD + Número)
- Formato: Apenas dígitos
- Exemplo: `"5511999999999"` (Brasil)
- Exemplo: `"1234567890"` (EUA)

## Formato de Números

### Número Individual
```json
{
  "number": "5511999999999"
}
```

### Grupo
```json
{
  "number": "120363XXXXXXXXXX@g.us"
}
```

O Evolution GO formata automaticamente números quando necessário.

## Paginação

Endpoints que retornam listas suportam paginação:

```bash
GET /endpoint?page=1&limit=50
```

## WebSocket

Para receber eventos em tempo real via WebSocket:

```
ws://localhost:4000/ws?token=SUA-CHAVE-API&instanceId=minha-instancia
```

## Webhooks

Configure webhooks via variável de ambiente:

```env
WEBHOOK_URL=https://seu-servidor.com/webhook
```

Eventos serão enviados via POST para a URL configurada.

## Rate Limiting

Atualmente não há rate limiting configurado, mas é recomendado:
- Máximo 50 requisições por segundo por instância
- Aguarde resposta antes de enviar próxima mensagem

## Swagger/OpenAPI

Documentação interativa disponível em:

```
http://localhost:4000/swagger/index.html
```

Recursos do Swagger:
- Testar endpoints diretamente
- Ver todos os parâmetros
- Exemplos de request/response
- Autenticação integrada

## Exemplos Práticos

### Criar e Conectar Instância

```bash
# 1. Criar instância (usa GLOBAL_API_KEY)
curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: GLOBAL_API_KEY" \
  -d '{
    "name": "vendas",
    "token": "token-vendas-123"
  }'

# 2. Conectar (usa token da instância)
curl -X POST http://localhost:4000/instance/connect \
  -H "Content-Type: application/json" \
  -H "apikey: token-vendas-123" \
  -d '{
    "webhookUrl": "https://meu-servidor.com/webhook"
  }'

# 3. Ver QR Code (usa token da instância)
curl -X GET http://localhost:4000/instance/qr \
  -H "apikey: token-vendas-123"
```

### Enviar Mensagem com Mídia

```bash
curl -X POST http://localhost:4000/send/media \
  -H "Content-Type: application/json" \
  -H "apikey: token-vendas-123" \
  -d '{
    "number": "5511999999999",
    "url": "https://exemplo.com/produto.jpg",
    "type": "image",
    "caption": "Confira nosso produto!"
  }'
```

### Criar Grupo

```bash
curl -X POST http://localhost:4000/group/create \
  -H "Content-Type: application/json" \
  -H "apikey: token-vendas-123" \
  -d '{
    "name": "Equipe de Vendas",
    "participants": ["5511999999999", "5511888888888"]
  }'
```

## Boas Práticas

### 1. Validar Números
Use `/user/check` para verificar se número existe no WhatsApp

### 2. Gerenciar Erros
Sempre trate erros HTTP 4xx e 5xx

### 3. Usar Webhooks
Para receber mensagens, configure webhooks em vez de polling

### 4. Múltiplas Instâncias
Use uma instância por número WhatsApp

### 5. Logs
Configure logs adequadamente para debug

## Troubleshooting

### Erro 401 Unauthorized
Verifique se a `apikey` está correta

### Erro 404 Instance not found
Certifique-se de que criou a instância primeiro

### Mensagem não enviada
1. Verifique status da conexão
2. Confirme formato do número
3. Verifique se número existe no WhatsApp

## Próximos Passos

- [API de Instâncias](./api-instances.md) - Gerenciar instâncias
- [API de Mensagens](./api-messages.md) - Enviar mensagens
- [API de Grupos](./api-groups.md) - Gerenciar grupos
- [Sistema de Eventos](../recursos-avancados/events-system.md) - Webhooks e eventos

---

**Dica**: Use o Swagger para explorar todos os endpoints disponíveis!
