# Conexão QR Code

Como funciona o processo de autenticação via QR Code no Evolution GO.

## 📋 Índice

- [Visão Geral](#visão-geral)
- [Fluxo de Conexão](#fluxo-de-conexão)
- [Geração do QR Code](#geração-do-qr-code)
- [Leitura do QR Code](#leitura-do-qr-code)
- [Pareamento por Código](#pareamento-por-código)
- [Estados da Conexão](#estados-da-conexão)
- [Reconexão Automática](#reconexão-automática)
- [Troubleshooting](#troubleshooting)
- [Boas Práticas](#boas-práticas)

---

## Visão Geral

O Evolution GO utiliza o protocolo Multi-Device do WhatsApp para autenticar instâncias. O processo é baseado em QR Code, similar ao WhatsApp Web.

### Como Funciona

1. **Gerar QR Code**: O Evolution GO cria um código único
2. **Escanear**: Você escaneia o código com o WhatsApp no celular
3. **Pareamento**: WhatsApp e Evolution GO trocam chaves de criptografia
4. **Sessão Ativa**: Conexão estabelecida e salva no banco de dados

### Por que QR Code?

**Vantagens**:
- ✅ Seguro: chaves criptográficas trocadas de forma protegida
- ✅ Não requer senha ou número de telefone
- ✅ Suporta múltiplos dispositivos conectados simultaneamente
- ✅ Processo simples e rápido

**Importante**: O WhatsApp não oferece login com usuário e senha para APIs. A autenticação é exclusivamente via QR Code ou código de pareamento.

---

## Fluxo de Conexão

### Diagrama Completo

```
┌──────────────┐                                        ┌──────────────┐
│ Evolution GO │                                        │  WhatsApp    │
│   Client     │                                        │   Servers    │
└──────┬───────┘                                        └──────┬───────┘
       │                                                       │
       │ 1. POST /instance/connect                            │
       │────────────────────────┐                             │
       │                        │                             │
       │ 2. StartClient()       │                             │
       │<───────────────────────┘                             │
       │                                                       │
       │ 3. Connect WebSocket                                 │
       │──────────────────────────────────────────────────────>│
       │                                                       │
       │ 4. Request QR Code                                   │
       │──────────────────────────────────────────────────────>│
       │                                                       │
       │ 5. QR Code Data (ref + public key)                   │
       │<──────────────────────────────────────────────────────│
       │                                                       │
       │ 6. Generate PNG QR Code                              │
       │────────────┐                                          │
       │            │                                          │
       │<───────────┘                                          │
       │                                                       │
       │ 7. Save QR to Database (base64|text)                 │
       │────────────┐                                          │
       │            │                                          │
       │<───────────┘                                          │
       │                                                       │
       │ 8. Emit QR_CODE Event                                │
       │────────────┐                                          │
       │            │ (Webhook/WebSocket)                      │
       │<───────────┘                                          │
       │                                                       │
┌──────▼───────┐                                              │
│    User      │                                              │
│ (Smartphone) │                                              │
└──────┬───────┘                                              │
       │                                                       │
       │ 9. Open WhatsApp App                                 │
       │    → Linked Devices                                  │
       │    → Scan QR Code                                    │
       │                                                       │
       │ 10. Send Pairing Data (encrypted)                    │
       │──────────────────────────────────────────────────────>│
       │                                                       │
       │ 11. Pairing Success                                  │
       │<──────────────────────────────────────────────────────│
       │                                                       │
┌──────▼───────┐                                              │
│ Evolution GO │                                              │
└──────┬───────┘                                              │
       │                                                       │
       │ 12. Receive Pairing Success Event                    │
       │<──────────────────────────────────────────────────────│
       │                                                       │
       │ 13. Generate Identity & Pre-Keys                     │
       │────────────┐                                          │
       │            │                                          │
       │<───────────┘                                          │
       │                                                       │
       │ 14. Save Session to Database                         │
       │────────────┐                                          │
       │            │ (JID, Keys, Device Info)                │
       │<───────────┘                                          │
       │                                                       │
       │ 15. Update Instance (connected=true, jid=...)        │
       │────────────┐                                          │
       │            │                                          │
       │<───────────┘                                          │
       │                                                       │
       │ 16. Emit CONNECTED Event                             │
       │────────────┐                                          │
       │            │                                          │
       │<───────────┘                                          │
       │                                                       │
       │ 17. Start Message Sync                               │
       │<─────────────────────────────────────────────────────>│
       │                    (bidirectional)                    │
       │                                                       │
```

### Passos Detalhados

#### 1. Conectar Instância

**API Call**:
```bash
POST /instance/connect
{
  "instanceName": "vendas"
}
```

**Ações**:
- Verifica se instância existe no banco
- Inicia goroutine com whatsmeow client
- Estabelece WebSocket com servidores WhatsApp

#### 2. Solicitação de QR Code

O Evolution GO solicita ao WhatsApp um código QR único, que contém:
- **Ref**: Referência única que identifica esta tentativa de conexão
- **Public Key**: Chave pública para iniciar o pareamento criptografado

#### 3. Geração da Imagem QR

**Formato do texto no QR Code**:
```
versão@ref,chave_pública,segredo,dados_adicionais
```

**Exemplo**:
```
2@ABC123DEF456,GHI789JKL012MNO345,PQR678STU901VWX234,YZA567
```

**Armazenamento**:
O QR Code é convertido em uma imagem PNG, codificada em base64 e salva no banco de dados junto com o texto original.

#### 4. Exibição do QR Code

**Via navegador**:
```
GET /instance/qr?instanceName=vendas
```

**Resposta HTML**:
```html
<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUg..." />
```

**Via API JSON**:
```bash
GET /instance/qr?instanceName=vendas
```

```json
{
  "qrcode": "iVBORw0KGgoAAAANSUhEUgAA...",
  "code": "2@ABC123,DEF456,GHI789,JKL012"
}
```

#### 5. Usuário Escaneia

**No smartphone**:
1. Abre WhatsApp
2. Menu → Aparelhos conectados
3. Conectar um aparelho
4. Escaneia QR Code

**WhatsApp envia**:
- Chaves públicas do device
- JID (WhatsApp ID) do número
- Device info (modelo, OS)

#### 6. Pareamento Concluído

Após o usuário escanear o QR Code:

1. **Evolution GO recebe confirmação** do WhatsApp
2. **Sessão é salva** no banco de dados com:
   - Chaves de identidade
   - Chaves pré-geradas
   - JID (identificador do WhatsApp)
   - Informações do dispositivo
3. **Instância marcada como conectada**
4. **Evento CONNECTED** é publicado

---

## Geração do QR Code

### Evento de QR Code

Quando um QR Code é gerado:

1. **Texto do QR Code** é recebido do WhatsApp
2. **Imagem PNG** é criada a partir do texto
3. **Salvo no banco de dados** em formato base64
4. **Evento QR_CODE** é publicado para webhooks/websockets configurados

### Formato do QR

**Estrutura**:
```
version@ref,pub_key,secret,adv
```

**Componentes**:
- `version`: Versão do protocolo (geralmente `2`)
- `ref`: Referência única (identifica esta tentativa de pareamento)
- `pub_key`: Chave pública do servidor
- `secret`: Segredo compartilhado
- `adv`: Dados de advertising (device info)

**Exemplo real**:
```
2@12345abcdef,67890ghijk,lmnop12345,qrstu67890
```

### Expiração

**Tempo de vida**: ~40 segundos

**Após expiração**:
- Whatsmeow emite novo evento QR
- Novo QR Code é gerado
- Repete até `max_attempts` (padrão: 5 tentativas)

**Evento de timeout**:
```go
case *events.QRTimeout:
    logger.Warn("QR Code expired, generating new one...")
```

---

## Leitura do QR Code

### Endpoint HTTP

**Obter QR Code via API**:
```bash
GET /instance/qr?instanceName=vendas
```

**Response (JSON)**:
```json
{
  "qrcode": "iVBORw0KGgoAAAANSUhEUgAABAA...",
  "code": "2@12345abcdef,67890ghijk,lmnop12345,qrstu67890"
}
```

**Response (HTML)** - Se aceitar `text/html`:
```html
<!DOCTYPE html>
<html>
<head><title>QR Code - vendas</title></head>
<body>
    <h1>QR Code - Instância: vendas</h1>
    <img src="data:image/png;base64,iVBORw0KG..." />
    <p>Escaneie com WhatsApp</p>
</body>
</html>
```

### Via WebSocket

**Conectar**:
```javascript
const ws = new WebSocket('ws://localhost:4000/ws?token=TOKEN&instanceId=vendas');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    
    if (data.queue === 'qrcode') {
        const payload = JSON.parse(data.payload);
        const qrBase64 = payload.qrcode;
        
        // Exibir QR Code
        document.getElementById('qr-img').src = 
            `data:image/png;base64,${qrBase64}`;
    }
};
```

### Via Webhook

**Configurar webhook**:
```bash
POST /instance/connect
{
  "instanceName": "vendas",
  "webhookUrl": "https://meu-servidor.com/webhook",
  "subscribe": ["QRCODE"]
}
```

**Evento recebido**:
```json
{
  "event": "QRCODE",
  "instance": "vendas",
  "data": {
    "qrcode": "iVBORw0KGgoAAAANSUhEUgA...",
    "code": "2@12345abcdef,67890ghijk,lmnop12345,qrstu67890"
  }
}
```

---

## Pareamento por Código

### Alternativa ao QR Code

Além de QR Code, é possível parear via **código de 8 dígitos**.

**Endpoint**:
```bash
POST /instance/pair
{
  "instanceName": "vendas",
  "phone": "5511999999999"
}
```

**Response**:
```json
{
  "pairingCode": "ABCD-1234"
}
```

### Como Usar

**No smartphone**:
1. Abre WhatsApp
2. Menu → Aparelhos conectados
3. Conectar um aparelho
4. Link com código de aparelho
5. Digita `ABCD-1234`

**Vantagens**:
- ✅ Não precisa de camera/QR scanner
- ✅ Mais fácil para compartilhar remotamente

**Desvantagens**:
- ❌ Requer digitação manual
- ❌ Menos visual que QR Code

### Como Funciona

O Evolution GO gera um código de 8 dígitos único e o retorna na API. Esse código é válido por tempo limitado e pode ser usado uma única vez.

---

## Estados da Conexão

### Máquina de Estados

```
┌─────────────┐
│ CREATED     │ Instância criada, nunca conectada
└──────┬──────┘
       │ POST /instance/connect
       ▼
┌─────────────┐
│ CONNECTING  │ WebSocket iniciando
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ WAITING_QR  │ Aguardando usuário escanear
└──────┬──────┘
       │ Usuário escaneia QR
       ▼
┌─────────────┐
│ PAIRING     │ Trocando chaves criptográficas
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ OPEN        │ Conectado e autenticado
└──────┬──────┘
       │
       │ Erro de conexão ou logout
       ▼
┌─────────────┐
│ DISCONNECTED│
└─────────────┘
```

### Eventos Associados

#### QRCODE
```json
{
  "event": "QRCODE",
  "data": {
    "qrcode": "base64...",
    "code": "2@..."
  }
}
```

#### QR_TIMEOUT
```json
{
  "event": "QR_TIMEOUT",
  "data": {
    "attempts": 1,
    "maxAttempts": 5
  }
}
```

#### QR_SUCCESS
```json
{
  "event": "QR_SUCCESS",
  "data": {
    "jid": "5511999999999@s.whatsapp.net"
  }
}
```

#### CONNECTED
```json
{
  "event": "CONNECTED",
  "data": {
    "jid": "5511999999999@s.whatsapp.net",
    "name": "João Silva",
    "timestamp": "2025-11-11T10:30:00Z"
  }
}
```

---

## Reconexão Automática

### Após Primeiro Pareamento

Depois do primeiro pareamento bem-sucedido via QR Code:

1. **Sessão é salva** no banco de dados PostgreSQL
2. **Próxima inicialização**: Sessão é carregada automaticamente
3. **Conecta diretamente** sem necessidade de novo QR Code

O Evolution GO mantém várias tabelas para armazenar as informações de sessão e chaves criptográficas necessárias para reconexão.

### Tentativas de Reconexão

Se uma conexão cair, o Evolution GO tenta reconectar automaticamente usando estratégia de intervalo crescente:

- Tentativa 1: 2 segundos
- Tentativa 2: 4 segundos
- Tentativa 3: 8 segundos
- Tentativa 4: 16 segundos
- Tentativa 5: 32 segundos

### Quando é Necessário Novo QR Code

Em alguns cenários, é necessário gerar um novo QR Code:

- ❌ Após logout explícito via API
- ❌ Dispositivo removido manualmente no app WhatsApp
- ❌ Sessão expirada (mais de 14 dias offline)
- ❌ Conta temporariamente ou permanentemente bloqueada

---

## Troubleshooting

### QR Code não aparece

**Diagnóstico**:
```bash
GET /instance/logs/:instanceId?level=ERROR
```

**Causas comuns**:
1. Cliente não conectado ao WebSocket
2. Firewall bloqueando conexão com WhatsApp servers
3. Instância já conectada (não precisa de QR)

**Solução**:
```bash
# Forçar reconexão
POST /instance/reconnect
{
  "instanceName": "vendas"
}
```

### QR Code expira muito rápido

**Causa**: Delay entre geração e exibição.

**Solução**: Use WebSocket para receber QR em tempo real.

```javascript
const ws = new WebSocket('ws://localhost:4000/ws?token=TOKEN&instanceId=vendas');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.queue === 'qrcode') {
        // Exibir imediatamente
        showQRCode(data.payload.qrcode);
    }
};
```

### Escaneou QR mas não conectou

**Diagnóstico**:
```bash
GET /instance/status?instanceName=vendas
```

**Resposta esperada após scan**:
```json
{
  "connected": true,
  "loggedIn": true,
  "jid": "5511999999999@s.whatsapp.net"
}
```

**Se continuar `connected: false`**:
1. Verificar logs de erro
2. Tentar logout e novo QR
3. Verificar se WhatsApp está atualizado

### Erro: "too many attempts"

**Causa**: QR Code expirou 5 vezes.

**Solução**:
```bash
# Desconectar
POST /instance/disconnect

# Aguardar 30 segundos

# Reconectar
POST /instance/connect
```

---

## Boas Práticas

### 1. Exibir QR em Tempo Real

Use WebSocket em vez de polling:

**❌ Ruim** (polling):
```javascript
setInterval(() => {
    fetch('/instance/qr?instanceName=vendas')
        .then(res => res.json())
        .then(data => showQR(data.qrcode));
}, 5000);  // A cada 5 segundos
```

**✅ Bom** (WebSocket):
```javascript
const ws = new WebSocket('ws://localhost:4000/ws?token=TOKEN&instanceId=vendas');
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.queue === 'qrcode') showQR(data.payload.qrcode);
};
```

### 2. Feedback Visual

Mostre estado da conexão para o usuário:

```javascript
ws.onmessage = (event) => {
    const { queue } = JSON.parse(event.data);
    
    switch(queue) {
        case 'qrcode':
            showStatus('Escaneie o QR Code');
            break;
        case 'qr_success':
            showStatus('QR Code escaneado! Conectando...');
            break;
        case 'connected':
            showStatus('Conectado com sucesso!');
            break;
    }
};
```

### 3. Timeout de Inatividade

Desconecte se QR não for escaneado em tempo razoável:

```javascript
let qrTimeout;

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    
    if (data.queue === 'qrcode') {
        // Cancela timeout anterior
        clearTimeout(qrTimeout);
        
        // Timeout de 2 minutos
        qrTimeout = setTimeout(() => {
            alert('QR Code expirou. Por favor, tente novamente.');
            ws.close();
        }, 120000);
    }
    
    if (data.queue === 'connected') {
        clearTimeout(qrTimeout);
    }
};
```

### 4. Validar Sessão Antes de Solicitar QR

```bash
# Verificar status primeiro
GET /instance/status?instanceName=vendas

# Se já conectado, não precisa de QR
# Se desconectado mas tem sessão, tentar reconnect
# Só gerar QR se não tem sessão
```

---

## Próximos Passos

- [Multi-Dispositivo](./multi-device.md) - Entender protocolo Multi-Device
- [Instâncias WhatsApp](../conceitos-core/instances.md) - Ciclo de vida completo
- [Sistema de Eventos](./events-system.md) - Receber eventos de conexão
- [API de Instâncias](../guias-api/api-instances.md) - Todos os endpoints

---

**Documentação gerada para Evolution GO v1.0**
