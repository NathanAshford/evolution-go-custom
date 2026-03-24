# API de Labels

Documentação completa dos endpoints para organizar chats e mensagens com etiquetas (labels).

## 📋 Índice

- [Adicionar Label em Chat](#adicionar-label-em-chat)
- [Adicionar Label em Mensagem](#adicionar-label-em-mensagem)
- [Editar Label](#editar-label)
- [Remover Label de Chat](#remover-label-de-chat)
- [Remover Label de Mensagem](#remover-label-de-mensagem)
- [Listar Todas as Labels](#listar-todas-as-labels)

---

## Adicionar Label em Chat

Adiciona uma etiqueta (label) a um chat.

**Endpoint**: `POST /label/chat`

**Headers**:
```
Content-Type: application/json
apikey: SUA-CHAVE-API
```

**Body**:
```json
{
  "jid": "5511999999999@s.whatsapp.net",
  "labelId": "1"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string | ✅ Sim | JID do chat (individual ou grupo) |
| `labelId` | string | ✅ Sim | ID da label |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/label/chat \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "5511999999999@s.whatsapp.net",
    "labelId": "1"
  }'
```

---

## Adicionar Label em Mensagem

Adiciona uma etiqueta a uma mensagem específica.

**Endpoint**: `POST /label/message`

**Body**:
```json
{
  "jid": "5511999999999@s.whatsapp.net",
  "labelId": "2",
  "messageId": "3EB0C5A277F7F9B6C599"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string | ✅ Sim | JID do chat |
| `labelId` | string | ✅ Sim | ID da label |
| `messageId` | string | ✅ Sim | ID da mensagem |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/label/message \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "5511999999999@s.whatsapp.net",
    "labelId": "2",
    "messageId": "3EB0C5A277F7F9B6C599"
  }'
```

---

## Editar Label

Edita nome, cor ou deleta uma label existente.

**Endpoint**: `POST /label/edit`

**Body**:
```json
{
  "labelId": "1",
  "name": "Clientes VIP",
  "color": 0,
  "deleted": false
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `labelId` | string | ✅ Sim | ID da label a editar |
| `name` | string | ✅ Sim | Novo nome da label |
| `color` | int | ❌ Não | Código de cor (0-19) |
| `deleted` | bool | ❌ Não | Se true, marca label como deletada |

**Cores Disponíveis** (0-19):

| Código | Cor |
|--------|-----|
| 0 | Azul |
| 1 | Rosa |
| 2 | Roxo |
| 3 | Verde |
| 4 | Laranja |
| 5 | Cinza |
| 6 | Vermelho |
| 7 | Marrom |
| 8 | Verde água |
| 9 | Amarelo |
| 10-19 | Variações |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
# Renomear label
curl -X POST http://localhost:4000/label/edit \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "labelId": "1",
    "name": "Clientes VIP",
    "color": 3
  }'

# Deletar label
curl -X POST http://localhost:4000/label/edit \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "labelId": "1",
    "name": "Label antiga",
    "deleted": true
  }'
```

---

## Remover Label de Chat

Remove uma etiqueta de um chat.

**Endpoint**: `POST /unlabel/chat`

**Body**:
```json
{
  "jid": "5511999999999@s.whatsapp.net",
  "labelId": "1"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string | ✅ Sim | JID do chat |
| `labelId` | string | ✅ Sim | ID da label a remover |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/unlabel/chat \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "5511999999999@s.whatsapp.net",
    "labelId": "1"
  }'
```

---

## Remover Label de Mensagem

Remove uma etiqueta de uma mensagem específica.

**Endpoint**: `POST /unlabel/message`

**Body**:
```json
{
  "jid": "5511999999999@s.whatsapp.net",
  "labelId": "2",
  "messageId": "3EB0C5A277F7F9B6C599"
}
```

**Parâmetros**:

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `jid` | string | ✅ Sim | JID do chat |
| `labelId` | string | ✅ Sim | ID da label |
| `messageId` | string | ✅ Sim | ID da mensagem |

**Resposta de Sucesso (200)**:
```json
{
  "message": "success"
}
```

**Exemplo cURL**:
```bash
curl -X POST http://localhost:4000/unlabel/message \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "5511999999999@s.whatsapp.net",
    "labelId": "2",
    "messageId": "3EB0C5A277F7F9B6C599"
  }'
```

---

## Listar Todas as Labels

Obtém todas as labels da instância.

**Endpoint**: `GET /label`

**Headers**:
```
apikey: SUA-CHAVE-API
```

**Resposta de Sucesso (200)**:
```json
[
  {
    "ID": 1,
    "LabelID": "1",
    "Name": "Clientes VIP",
    "Color": 3,
    "Deleted": false,
    "PredefinedID": 0,
    "InstanceID": "minha-instancia"
  },
  {
    "ID": 2,
    "LabelID": "2",
    "Name": "Suporte",
    "Color": 6,
    "Deleted": false,
    "PredefinedID": 0,
    "InstanceID": "minha-instancia"
  }
]
```

**Campos da Resposta**:
- `ID`: ID interno do banco de dados
- `LabelID`: ID da label no WhatsApp
- `Name`: Nome da label
- `Color`: Código da cor (0-19)
- `Deleted`: Se foi deletada
- `PredefinedID`: ID de label pré-definida (0 se customizada)
- `InstanceID`: ID da instância

**Exemplo cURL**:
```bash
curl -X GET http://localhost:4000/label \
  -H "apikey: SUA-CHAVE-API"
```

---

## Fluxos Completos de Uso

### 1. Sistema de Categorização de Clientes

```bash
# 1. Listar labels existentes
LABELS=$(curl -s -X GET http://localhost:4000/label \
  -H "apikey: SUA-CHAVE-API")

echo $LABELS | jq '.'

# 2. Criar nova label (via edição)
curl -X POST http://localhost:4000/label/edit \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "labelId": "10",
    "name": "Lead Qualificado",
    "color": 3,
    "deleted": false
  }'

# 3. Etiquetar chats de clientes
CLIENTES=("5511999999999@s.whatsapp.net" "5511888888888@s.whatsapp.net")

for cliente in "\${CLIENTES[@]}"; do
  curl -X POST http://localhost:4000/label/chat \
    -H "Content-Type: application/json" \
    -H "apikey: SUA-CHAVE-API" \
    -d "{
      "jid": "$cliente",
      "labelId": "10"
    }"
done
```

### 2. Marcar Mensagens Importantes

```bash
# Etiquetar mensagem específica como "Urgente"
curl -X POST http://localhost:4000/label/message \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "jid": "5511999999999@s.whatsapp.net",
    "labelId": "5",
    "messageId": "3EB0C5A277F7F9B6C599"
  }'
```

### 3. Gerenciar Labels de Suporte

```bash
# Criar labels de status de atendimento
LABELS=(
  "1:Aguardando:0"
  "2:Em Atendimento:3"
  "3:Resolvido:4"
  "4:Cancelado:6"
)

for label_data in "\${LABELS[@]}"; do
  IFS=':' read -r id name color <<< "$label_data"
  
  curl -X POST http://localhost:4000/label/edit \
    -H "Content-Type: application/json" \
    -H "apikey: SUA-CHAVE-API" \
    -d "{
      "labelId": "$id",
      "name": "$name",
      "color": $color,
      "deleted": false
    }"
done
```

### 4. Reorganizar Labels

```bash
# Renomear label existente
curl -X POST http://localhost:4000/label/edit \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "labelId": "1",
    "name": "Clientes Premium",
    "color": 1
  }'

# Deletar label obsoleta
curl -X POST http://localhost:4000/label/edit \
  -H "Content-Type: application/json" \
  -H "apikey: SUA-CHAVE-API" \
  -d '{
    "labelId": "99",
    "name": "Label Antiga",
    "deleted": true
  }'
```

---

## Casos de Uso Práticos

### CRM Simples
Use labels para categorizar clientes:
- **Label 1**: Leads
- **Label 2**: Clientes Ativos
- **Label 3**: Clientes Inativos
- **Label 4**: VIP

### Suporte ao Cliente
Organize tickets por status:
- **Label 1**: Aberto
- **Label 2**: Em Progresso
- **Label 3**: Aguardando Cliente
- **Label 4**: Resolvido

### E-commerce
Categorize conversas por estágio:
- **Label 1**: Interessado
- **Label 2**: Pedido Realizado
- **Label 3**: Enviado
- **Label 4**: Entregue

### Equipes
Atribua conversas para departamentos:
- **Label 1**: Vendas
- **Label 2**: Suporte
- **Label 3**: Financeiro
- **Label 4**: Diretoria

---

## Códigos de Erro Comuns

| Código | Erro | Solução |
|--------|------|---------|
| 400 | `jid is required` | Forneça o JID do chat |
| 400 | `label id is required` | Forneça o ID da label |
| 400 | `message id is required` | Forneça o ID da mensagem |
| 400 | `name is required` | Forneça o nome da label (ao editar) |
| 500 | `instance not found` | Instância não conectada |
| 500 | `error parse community jid` | JID inválido |

---

## Boas Práticas

### 1. Planeje suas Labels
Defina um sistema de categorização antes de começar. Exemplo:
- **Label 1**: "Novo Lead" (cor: azul/0)
- **Label 2**: "Qualificado" (cor: verde/3)
- **Label 3**: "Cliente" (cor: rosa/1)
- **Label 4**: "Inativo" (cor: marrom/5)

### 2. Use Cores Consistentes
Mantenha um padrão de cores:
- **Verde (3)**: Status positivo
- **Vermelho (6)**: Urgente/Problema
- **Azul (0)**: Novo/Neutro
- **Laranja (4)**: Em progresso

### 3. Documente IDs
Mantenha uma tabela de referência:
```
ID | Nome | Cor | Descrição
---|------|-----|----------
1  | VIP  | 1   | Clientes premium
2  | Lead | 0   | Novos contatos
3  | Urg  | 6   | Requer atenção
```

### 4. Não Abuse de Labels
Limite-se a **10-15 labels** no máximo. Muitas labels dificultam a organização.

### 5. Limpe Labels Antigas
Periodicamente, revise e delete labels não utilizadas:
```bash
# Listar todas
curl -s http://localhost:4000/label -H "apikey: KEY" | jq '.'

# Deletar obsoletas
curl -X POST http://localhost:4000/label/edit \
  -H "Content-Type: application/json" \
  -H "apikey: KEY" \
  -d '{"labelId": "99", "name": "Old", "deleted": true}'
```

### 6. Labels em Mensagens
Use labels em mensagens apenas para casos especiais:
- Mensagens com pedidos
- Informações importantes para referência
- Reclamações/Problemas

**Evite**: Etiquetar todas as mensagens (cria poluição visual).

---

## Limitações do WhatsApp

### Limites Conhecidos
- **Máximo de labels**: Não documentado oficialmente, mas recomenda-se não ultrapassar 100
- **Labels por chat**: Sem limite conhecido
- **Labels por mensagem**: 1 label por mensagem

### Sincronização
Labels são sincronizadas entre dispositivos via WhatsApp Multi-Device. Mudanças podem levar alguns segundos para aparecer em outros dispositivos.

### Visibilidade
Labels são **privadas** - apenas você vê suas labels. O destinatário não vê que você etiquetou o chat ou mensagem.

---

## Próximos Passos

- [API de Chats](./api-chats.md) - Gerenciar conversas (pin, archive, mute)
- [API de Mensagens](./api-messages.md) - Enviar e gerenciar mensagens
- [API de Usuários](./api-user.md) - Gerenciar perfil e contatos
- [Visão Geral da API](./api-overview.md)

---

**Documentação gerada para Evolution GO v1.0**
