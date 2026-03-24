# Instâncias WhatsApp

Como funcionam as conexões do WhatsApp no Evolution GO - explicado de forma simples.

## 📋 Índice

- [O que é uma Instância](#o-que-é-uma-instância)
- [Informações de uma Instância](#informações-de-uma-instância)
- [Estados da Instância](#estados-da-instância)
- [Ciclo de Vida](#ciclo-de-vida)
- [Configurações Avançadas](#configurações-avançadas)
- [Sistema de Eventos](#sistema-de-eventos)
- [Proxy](#proxy)
- [Boas Práticas](#boas-práticas)

---

## O que é uma Instância

Uma **instância** é uma conexão independente com o WhatsApp. Pense assim:

### Analogia do Celular

Imagine que cada instância é como um celular separado:
- 📱 Celular 1 = Instância "vendas"
- 📱 Celular 2 = Instância "suporte"  
- 📱 Celular 3 = Instância "marketing"

Cada um:
- Tem seu próprio número do WhatsApp
- Envia e recebe mensagens independentemente
- Não interfere nos outros
- Tem suas próprias configurações

### Características

**Isolamento**: Cada instância é completamente separada
- Não compartilham mensagens
- Não compartilham contatos
- Não compartilham configurações

**Persistência**: A conexão fica salva
- Se o servidor reiniciar, reconecta automaticamente
- Não precisa escanear QR Code novamente
- Sessão guardada com segurança

**Multi-Device**: Usa o protocolo do WhatsApp Web
- Funciona como WhatsApp no computador
- Não desconecta o celular
- Até 4 dispositivos por número

---

## Informações de uma Instância

### Dados Básicos

Toda instância tem essas informações:

**id** - Identificador único (como um CPF)
```
Exemplo: abc-123-def-456
```

**name** - Nome amigável
```
Exemplo: "vendas", "suporte", "atendimento-24h"
Deve ser único no sistema
```

**token** - Chave de acesso
```
Exemplo: "token-vendas-secreto-123"
Usado para autenticar requisições
Deve ser único no sistema
```

**jid** - ID do WhatsApp (seu número)
```
Formato: 5511999999999@s.whatsapp.net
Só aparece depois de conectar
```

**connected** - Está conectado?
```
true = Conectado e funcionando
false = Desconectado
```

**qrcode** - QR Code para conectar
```
Formato: imagem_base64|codigo_texto
Expira em ~40 segundos
```

### Dados de Integração

**webhook** - URL para receber notificações
```
Exemplo: https://meusite.com/webhook
Recebe eventos em tempo real
```

**events** - Quais eventos quer receber
```
Exemplo: "MESSAGE,GROUP_UPDATE,CALL"
Pode ser "ALL" para receber tudo
```

**rabbitmqEnable** - Usar RabbitMQ?
```
"true" = Sim, enviar eventos
"false" = Não usar
```

**websocketEnable** - Usar WebSocket?
```
"true" = Sim, eventos em tempo real
"false" = Não usar
```

---

## Estados da Instância

### Diagrama de Estados

```
┌──────────────┐
│   CRIADA     │ ← Acabou de criar
└──────┬───────┘
       │ (aguardando conexão)
       ▼
┌──────────────┐
│ DESCONECTADA │ ← Sem conexão WhatsApp
└──────┬───────┘
       │ (conectar)
       ▼
┌──────────────┐
│  CONECTANDO  │ ← Iniciando conexão
└──────┬───────┘
       │ (gera QR Code)
       ▼
┌──────────────┐
│AGUARDANDO QR │ ← Esperando escanear
└──────┬───────┘
       │ (escaneia no celular)
       ▼
┌──────────────┐
│    ABERTA    │ ← Conectado! ✓
└──────┬───────┘
       │ (pode enviar/receber)
       │
       │ (se desconectar)
       ▼
┌──────────────┐
│ DESCONECTADA │
└──────────────┘
```

### Verificando o Estado

**Via API**:
```bash
GET /instance/status?instanceName=vendas

Resposta:
{
  "connected": true,
  "loggedIn": true,
  "jid": "5511999999999@s.whatsapp.net",
  "name": "João Silva"
}
```

---

## Ciclo de Vida

### 1. Criar Instância

**Endpoint**: `POST /instance/create`

**O que acontece**:
1. Sistema verifica se o nome já existe
2. Gera um ID único automaticamente
3. Salva no banco de dados
4. Retorna informações da instância
5. **Não conecta ainda** - apenas cria o registro

**Exemplo**:
```json
POST /instance/create
{
  "name": "vendas",
  "token": "meu-token-vendas"
}

Resposta:
{
  "id": "abc-123",
  "name": "vendas",
  "token": "meu-token-vendas",
  "connected": false
}
```

### 2. Conectar Instância

**Endpoint**: `POST /instance/connect`

**O que acontece**:
1. Sistema inicia cliente WhatsApp
2. Gera QR Code
3. Salva QR Code no banco
4. Aguarda você escanear

**Exemplo**:
```json
POST /instance/connect
{
  "instanceName": "vendas",
  "webhookUrl": "https://meusite.com/webhook",
  "subscribe": ["MESSAGE", "GROUP_UPDATE"]
}
```

**Importante**: A conexão é assíncrona! O QR Code aparece depois de alguns segundos.

### 3. Obter QR Code

**Endpoint**: `GET /instance/qr?instanceName=vendas`

**O que acontece**:
1. Sistema verifica se já está conectado
2. Se não estiver, retorna o QR Code
3. QR Code tem duas partes separadas por `|`

**Formato do QR Code**:
```
parte1|parte2

parte1 = imagem PNG em base64
parte2 = código texto do QR
```

**Uso prático**:
```html
<!-- Mostrar QR Code na página -->
<img src="parte1" />

<!-- Ou usar a biblioteca JS -->
<qrcode value="parte2"></qrcode>
```

### 4. Parear com Código

**Endpoint**: `POST /instance/pair`

**Alternativa ao QR Code**: Você recebe um código de 8 dígitos para digitar no celular.

**Exemplo**:
```json
POST /instance/pair
{
  "instanceName": "vendas",
  "phone": "5511999999999"
}

Resposta:
{
  "pairingCode": "1234-5678"
}
```

**Como usar**:
1. Abra WhatsApp no celular
2. Vá em Dispositivos Conectados
3. Conectar Dispositivo
4. Digite o código: 1234-5678

### 5. Reconectar

**Endpoint**: `POST /instance/reconnect`

**Diferença importante**:
- **Connect**: Primeira vez, precisa de QR Code
- **Reconnect**: Já conectou antes, reutiliza sessão

**Quando usar**:
- Após reiniciar o servidor
- Quando perder conexão temporária
- Para forçar reconexão

**Não precisa de QR Code!** A sessão já está salva.

### 6. Desconectar

**Endpoint**: `POST /instance/disconnect`

**O que acontece**:
1. Fecha a conexão com WhatsApp
2. Para de receber mensagens
3. **Mantém a sessão salva**
4. Pode reconectar depois sem QR Code

**Quando usar**:
- Manutenção temporária
- Pausar operação
- Trocar configurações

### 7. Logout

**Endpoint**: `POST /instance/logout`

**Diferença de Disconnect**:
- **Disconnect**: Desliga mas mantém sessão
- **Logout**: Remove a sessão completamente

**O que acontece**:
1. Remove sessão do WhatsApp
2. Desconecta completamente
3. **Precisa de novo QR Code** para conectar de novo
4. Como se nunca tivesse conectado

**Quando usar**:
- Trocar de número
- Limpar conexão completamente
- Resetar instância

### 8. Deletar

**Endpoint**: `DELETE /instance/delete/:id`

**O que acontece**:
1. Faz logout se estiver conectado
2. Remove todas as mensagens dessa instância
3. Remove todas as labels dessa instância
4. **Deleta tudo do banco de dados**
5. Operação irreversível!

**⚠️ ATENÇÃO**: Não tem como desfazer!

---

## Configurações Avançadas

### O que São

Configurações extras para mudar o comportamento da instância.

### Opções Disponíveis

**alwaysOnline** - Aparecer sempre online
```
true = Sempre mostra como "online"
false = Normal (online quando está usando)
```

**rejectCall** - Rejeitar chamadas automaticamente
```
true = Rejeita chamadas de voz/vídeo
false = Aceita normalmente
```

**msgRejectCall** - Mensagem ao rejeitar
```
Exemplo: "Desculpe, não atendo chamadas. Envie mensagem de texto."
Enviada automaticamente quando rejeitar
```

**readMessages** - Marcar como lidas automaticamente
```
true = Marca todas as mensagens recebidas como lidas
false = Normal (você marca manualmente)
```

**ignoreGroups** - Ignorar mensagens de grupo
```
true = Não processa eventos de grupos
false = Normal (recebe tudo)
```

**ignoreStatus** - Ignorar status/stories
```
true = Não processa atualizações de status
false = Normal (recebe tudo)
```

### Como Configurar

**Endpoint**: `POST /instance/:id/advanced-settings`

**Exemplo**:
```json
{
  "alwaysOnline": true,
  "rejectCall": true,
  "msgRejectCall": "Não atendo chamadas, envie mensagem!",
  "readMessages": false,
  "ignoreGroups": false,
  "ignoreStatus": true
}
```

**Efeito**: Aplica instantaneamente sem precisar reconectar!

---

## Sistema de Eventos

### O que São Eventos

Notificações que o Evolution GO envia quando algo acontece.

**Analogia**: Como notificações do celular
- "Nova mensagem recebida!"
- "Fulano entrou no grupo!"
- "Chamada recebida!"

### Tipos de Eventos

**Mensagens**:
- `MESSAGE` - Mensagem recebida
- `MESSAGE_SENT` - Mensagem enviada
- `MESSAGE_UPDATE` - Mensagem editada/deletada
- `MESSAGE_REACTION` - Reação em mensagem

**Grupos**:
- `GROUP_UPDATE` - Grupo atualizado
- `GROUP_PARTICIPANT_UPDATE` - Alguém entrou/saiu

**Conexão**:
- `CONNECTION_UPDATE` - Mudou status de conexão
- `QR_CODE` - QR Code gerado
- `QRCODE_UPDATED` - QR Code atualizado

**Outros**:
- `PRESENCE_UPDATE` - Alguém ficou online/offline
- `CALL` - Chamada recebida
- `CONTACT_UPDATE` - Contato atualizado
- `LABEL` - Label criada/atualizada

### Como Subscrever

**Ao conectar, escolha quais eventos quer**:

```json
POST /instance/connect
{
  "instanceName": "vendas",
  "subscribe": ["MESSAGE", "GROUP_UPDATE", "CALL"]
}
```

**Opções**:
- Lista específica: `["MESSAGE", "CALL"]`
- Todos os eventos: `["ALL"]`
- Apenas mensagens (padrão): `["MESSAGE"]`

### Destinos dos Eventos

Pode enviar para vários lugares ao mesmo tempo:

**Webhook** (HTTP POST):
```json
{
  "webhookUrl": "https://meusite.com/webhook"
}
```

**RabbitMQ** (Fila de mensagens):
```json
{
  "rabbitmqEnable": "true"
}
```

**WebSocket** (Tempo real):
```json
{
  "websocketEnable": "true"
}
```

**NATS** (Sistema de eventos):
```json
{
  "natsEnable": "true"
}
```

**Pode ativar todos juntos!** Cada evento será enviado para todos os destinos configurados.

---

## Proxy

### O que é Proxy

Um servidor intermediário entre você e o WhatsApp.

**Analogia**: Como usar uma empresa de correio
- Você → Correio → Destinatário
- Seu IP fica escondido
- WhatsApp vê o IP do proxy

### Quando Usar

**Recomendado se**:
- Tem muitas instâncias (>10)
- Quer esconder seu IP real
- Precisa de múltiplos IPs
- WhatsApp bloqueou seu IP

### Como Configurar

**Endpoint**: `POST /instance/proxy/:id`

**Exemplo**:
```json
  {
  "host": "proxy.exemplo.com",
  "port": "8080",
  "username": "usuario",      // Opcional
  "password": "senha"          // Opcional
}
```

**Importante**: Precisa reconectar para aplicar!

### Como Remover

**Endpoint**: `DELETE /instance/proxy/:id`

Remove o proxy e volta a conexão direta.

---

## Boas Práticas

### 1. Nomeação

Use nomes descritivos:

**✅ Bom**:
- `vendas-regiao-sul`
- `suporte-nivel-1`
- `marketing-campanhas`
- `atendimento-24h`

**❌ Ruim**:
- `teste`
- `instance1`
- `minha-instancia`
- `zap`

### 2. Gerenciar Eventos

**Subscreva apenas o necessário**:

```json
// ✅ Bom - Apenas o que precisa
{"subscribe": ["MESSAGE", "GROUP_UPDATE"]}

// ❌ Ruim - Muito tráfego desnecessário
{"subscribe": ["ALL"]}
```

Menos eventos = menos processamento = mais rápido

### 3. Configurações Avançadas

**Otimize para seu caso**:

```json
// Bot de atendimento
{
  "ignoreGroups": true,    // Não precisa de grupos
  "ignoreStatus": true,    // Não precisa de status
  "readMessages": true     // Marca como lido automaticamente
}

// Monitoramento de grupos
{
  "ignoreGroups": false,   // Precisa de grupos
  "ignoreStatus": true,    // Não precisa de status
  "readMessages": false    // Não marca como lido
}
```

### 4. Use Proxy

**Se tiver muitas instâncias** (>10):

- Evita bloqueio de IP
- Distribui a carga
- Mais estável

### 5. Monitore Logs

**Acompanhe sua instância**:

```bash
GET /instance/logs/vendas?level=ERROR&limit=50
```

Veja erros antes que causem problemas!

### 6. Limpeza Regular

**Delete instâncias não usadas**:

```bash
DELETE /instance/delete/teste-antigo
```

Libera recursos e organiza o sistema.

### 7. Reconexão vs Logout

**Entenda a diferença**:

| Situação | Use |
|----------|-----|
| Reiniciou servidor | Reconnect |
| Perdeu conexão | Reconnect |
| Trocar de número | Logout |
| Limpar tudo | Logout + Delete |

---

## Limitações

### Limites do WhatsApp

**Dispositivos**:
- Máximo 4 dispositivos por número
- Evolution GO conta como 1 dispositivo

**QR Code**:
- Expira em ~40 segundos
- Máximo de tentativas por hora

**Reconexão**:
- Não reconecte mais de 1x por minuto
- Pode ser bloqueado temporariamente

### Limites do Evolution GO

**Por Servidor**:
- Depende da RAM disponível
- Cada instância usa ~50-100MB
- Exemplo: 16GB RAM = ~100 instâncias

**Banco de Dados**:
- PostgreSQL: 100 conexões simultâneas (padrão)
- Pode configurar mais se necessário

---

## Troubleshooting

### "no active session found"

**Problema**: Cliente não existe.

**Causa**: Não conectou ainda ou sessão expirou.

**Solução**:
1. Conecte via `POST /instance/connect`
2. Escaneie QR Code
3. Aguarde conectar

### "client disconnected"

**Problema**: Perdeu conexão com WhatsApp.

**Causa**: Internet caiu ou WhatsApp instável.

**Solução**:
```bash
POST /instance/reconnect
```

### QR Code não aparece

**Problema**: QR Code vazio ou nulo.

**Causa**: Cliente ainda inicializando.

**Solução**:
1. Aguarde 2-3 segundos após `/connect`
2. Tente `/instance/qr` novamente
3. Se persistir, tente reconectar

### Não reconecta após reiniciar

**Problema**: Precisa escanear QR Code de novo.

**Causa**: Sessão corrompida ou expirada.

**Solução**:
```bash
# 1. Logout completo
POST /instance/logout

# 2. Conectar de novo
POST /instance/connect

# 3. Escanear novo QR Code
```

### Mensagens não chegam

**Problema**: Eventos não são recebidos.

**Causa**: Configuração de eventos errada.

**Solução**:
1. Verifique se inscreveu no evento `MESSAGE`
2. Teste o webhook/RabbitMQ
3. Veja os logs da instância

---

## Exemplo Completo

### Do Zero até Enviar Mensagem

```bash
# 1. Criar instância
POST /instance/create
{
  "name": "vendas",
  "token": "token-vendas-123"
}

# 2. Conectar
POST /instance/connect
{
  "instanceName": "vendas",
  "webhookUrl": "https://meusite.com/webhook",
  "subscribe": ["MESSAGE"]
}

# 3. Aguardar 2 segundos...

# 4. Pegar QR Code
GET /instance/qr?instanceName=vendas

# 5. Escanear QR Code no celular

# 6. Aguardar conectar (~5 segundos)

# 7. Enviar mensagem! 🎉
POST /send/text
Headers: apikey: token-vendas-123
{
  "number": "5511999999999",
  "text": "Olá! Primeira mensagem!"
}
```

---

## Resumo Rápido

| Conceito | Explicação |
|----------|------------|
| **Instância** | Uma conexão WhatsApp independente |
| **QR Code** | Código para parear com celular |
| **Connect** | Primeira conexão (precisa QR) |
| **Reconnect** | Reconexão (não precisa QR) |
| **Logout** | Remove sessão completamente |
| **Delete** | Apaga tudo (irreversível) |
| **Eventos** | Notificações quando algo acontece |
| **Proxy** | Servidor intermediário |

**Lembre-se**:
- 📱 Cada instância = 1 WhatsApp
- 🔄 Reconnect não precisa de QR Code
- 🚪 Logout remove tudo
- 📊 Configure apenas eventos necessários
- 🔒 Use proxy para muitas instâncias
- 🗑️ Delete instâncias não usadas

---

**Documentação Evolution GO v1.0**
