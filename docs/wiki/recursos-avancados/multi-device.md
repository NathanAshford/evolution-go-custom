# WhatsApp Multi-Device

Entenda como funciona o protocolo Multi-Device do WhatsApp e como o Evolution GO o utiliza.

## 📋 Índice

- [Visão Geral](#visão-geral)
- [Arquitetura](#arquitetura)
- [Protocolo Signal](#protocolo-signal)
- [Sincronização de Dados](#sincronização-de-dados)
- [Criptografia](#criptografia)
- [Limitações](#limitações)
- [Vantagens](#vantagens)
- [Comparação com Versão Antiga](#comparação-com-versão-antiga)

---

## Visão Geral

WhatsApp **Multi-Device** é o protocolo que permite conectar até 4 dispositivos simultaneamente, sem necessidade do celular estar online após o pareamento inicial.

### Antes vs Depois

**Legacy (Antigo - Web WhatsApp)**:
```
┌─────────────┐
│  Celular    │ ◄── SEMPRE precisa estar online
│  (Primary)  │
└──────┬──────┘
       │
       │ Relay
       │
       ▼
┌──────────────┐
│ WhatsApp Web │ ◄── Depende 100% do celular
└──────────────┘
```

**Multi-Device (Atual)**:
```
┌─────────────┐
│  Celular    │ ◄── Dispositivo principal
│  (Primary)  │
└──────┬──────┘
       │
       │ Peer-to-Peer Sync
       │
       ├────────────┬────────────┬────────────┐
       ▼            ▼            ▼            ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│WhatsApp  │ │Evolution │ │WhatsApp  │ │WhatsApp  │
│Desktop   │ │   GO     │ │Web       │ │Business  │
└──────────┘ └──────────┘ └──────────┘ └──────────┘

Todos funcionam INDEPENDENTEMENTE!
```

---

## Arquitetura

### Componentes

1. **Dispositivo Principal (Primary Device)**
   - Celular com WhatsApp instalado
   - Única fonte de verdade para contatos e configurações
   - Pode funcionar offline após sincronização inicial

2. **Dispositivos Companion (Companion Devices)**
   - WhatsApp Web, Desktop, Business API, Evolution GO
   - Até 4 dispositivos simultâneos
   - Funcionam independentemente após pareamento

3. **Servidores WhatsApp**
   - Facilitam comunicação entre devices
   - Armazenam mensagens criptografadas temporariamente
   - Sincronizam estado entre dispositivos

### Diagrama de Comunicação

```
┌──────────────────────────────────────────────────────────┐
│                   WhatsApp Servers                       │
│  - Message Routing                                       │
│  - State Synchronization                                 │
│  - End-to-End Encryption Relay                           │
└───────────────────┬──────────────────────────────────────┘
                    │
         ┌──────────┼──────────┬──────────┬──────────┐
         │          │          │          │          │
         ▼          ▼          ▼          ▼          ▼
    ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
    │Primary │ │Device 1│ │Device 2│ │Device 3│ │Device 4│
    │(Phone) │ │Desktop │ │Evolution│ │   Web  │ │Business│
    └────────┘ └────────┘ └────────┘ └────────┘ └────────┘
         │          │          │          │          │
         └──────────┴──────────┴──────────┴──────────┘
                    │
            Direct Peer Sync
         (quando ambos online)
```

### Funcionamento

**Enviar Mensagem**:
1. Device 1 (Evolution GO) envia mensagem criptografada para WhatsApp servers
2. Servers encaminham para destinatário
3. Servers **também** sincronizam com outros devices do remetente
4. Primary device e outros companion devices recebem cópia

**Receber Mensagem**:
1. WhatsApp servers recebem mensagem destinada ao seu número
2. Servers replicam para **todos os devices conectados** simultaneamente
3. Cada device descriptografa independentemente

---

## Protocolo Signal

### O que é Signal Protocol?

**Signal Protocol** é o protocolo de criptografia ponta-a-ponta (E2EE) usado pelo WhatsApp Multi-Device.

**Criado por**: Open Whisper Systems (agora Signal Foundation)

**Usado em**:
- WhatsApp
- Signal Messenger
- Facebook Messenger (Secret Conversations)
- Google Messages (RCS)

### Componentes do Signal Protocol

#### 1. Identity Keys (Chaves de Identidade)

Cada dispositivo tem um par de chaves únicas:
- **Chave Pública**: Compartilhada com outros dispositivos
- **Chave Privada**: Nunca sai do dispositivo, usada para descriptografia

#### 2. Pre-Keys (Chaves Pré-geradas)

Um conjunto de 100 chaves geradas automaticamente quando você conecta um dispositivo pela primeira vez. Essas chaves permitem que outros dispositivos iniciem conversas criptografadas com você, mesmo quando você está offline.

**Renovação**: Novas pre-keys são geradas automaticamente conforme necessário.

#### 3. Signed Pre-Keys (Chaves Assinadas)

Uma pre-key especial assinada pela sua chave de identidade, provando autenticidade.

**Renovação**: A cada 7 dias.

#### 4. Session Keys (Chaves de Sessão)

Chaves únicas para cada conversa que mudam automaticamente a cada mensagem enviada (conceito chamado "ratcheting").

**Segurança**: Se uma chave for comprometida, apenas aquela mensagem específica é afetada - mensagens anteriores e futuras permanecem seguras (Forward Secrecy).

---

## Sincronização de Dados

### Tipos de Sincronização

#### 1. Mensagens

**Histórico Inicial**:
- Últimos **3 meses** de conversas
- Mídia: Apenas thumbnails (mídia completa baixada on-demand)

**Novas Mensagens**:
- Sincronizadas em tempo real
- Todos os dispositivos recebem simultaneamente

#### 2. Contatos

**Sincronização**:
- Lista completa de contatos do celular
- Atualizada automaticamente quando um contato muda nome ou número

#### 3. Grupos

**Informações sincronizadas**:
- Lista de todos os grupos que você participa
- Metadados (nome, descrição, foto, lista de participantes)
- Configurações (silenciado, fixado)

#### 4. Chats

**Estados sincronizados**:
- Chats fixados (pinned)
- Chats arquivados (archived)
- Chats silenciados (muted)
- Mensagens lidas/não lidas

#### 5. Configurações

**Sincronizadas**:
- Foto de perfil
- Nome de exibição
- Recado (status text)
- Configurações de privacidade

**NÃO sincronizadas**:
- Notificações (específico por device)
- Tema/aparência (específico por device)

### History Sync Request

**Solicitação de histórico** (implementado via API):

```bash
POST /chat/history-sync-request
{
  "messageInfo": {
    "Chat": "5511999999999@s.whatsapp.net",
    "IsFromMe": false,
    "IsGroup": false,
    "ID": "3EB0C5A277F7F9B6C599",
    "Timestamp": "2025-11-11T10:00:00Z"
  },
  "count": 50
}
```

**Parâmetros**:
- `messageInfo`: Mensagem de referência (ponto de partida)
- `count`: Número de mensagens para buscar (máx 100)

**Uso**: Carregar mensagens antigas de uma conversa.

---

## Criptografia

### End-to-End Encryption (Criptografia Ponta-a-Ponta)

Todas as mensagens são criptografadas no dispositivo do remetente e só podem ser descriptografadas no dispositivo do destinatário.

**Importante**: Os servidores do WhatsApp não conseguem ler o conteúdo das mensagens - eles apenas facilitam a entrega dos dados criptografados.

### Fluxo de Criptografia

```
┌────────────┐                                  ┌────────────┐
│  Remetente │                                  │Destinatário│
│  (Device)  │                                  │  (Device)  │
└─────┬──────┘                                  └──────┬─────┘
      │                                                │
      │ 1. Mensagem: "Olá!"                            │
      │──────────┐                                     │
      │          │                                     │
      │ 2. Criptografa com Session Key                │
      │<─────────┘                                     │
      │                                                │
      │ 3. Envia encrypted blob                       │
      │───────────────────────────────────────────────>│
      │        (via WhatsApp Servers)                  │
      │                                                │
      │                                                │ 4. Descriptografa
      │                                                │────────────┐
      │                                                │            │
      │                                                │ 5. "Olá!"  │
      │                                                │<───────────┘
      │                                                │
```

### Verificação de Segurança

**Código de Segurança (Safety Number)**:
- É um código de 60 dígitos único para cada conversa
- Compara as chaves de identidade dos participantes
- Muda se um dos participantes trocar de dispositivo

**Como verificar no WhatsApp**:
1. Abrir a conversa
2. Tocar no nome do contato
3. Selecionar "Criptografia"
4. Comparar o código com o contato pessoalmente ou por outro canal seguro

---

## Limitações

### Limite de Devices

**Máximo**: 4 companion devices + 1 primary device (celular).

**Exemplo**:
- ✅ Celular (Primary)
- ✅ WhatsApp Web (Device 1)
- ✅ WhatsApp Desktop (Device 2)
- ✅ Evolution GO (Device 3)
- ✅ WhatsApp Business API (Device 4)
- ❌ Outro device → Erro: "Máximo de devices atingido"

**Solução**: Desconectar um device antes de conectar novo.

### Histórico Limitado

**Sincronização inicial**: Apenas últimos **3 meses**.

**Mensagens mais antigas**:
- Não são sincronizadas automaticamente
- Podem ser buscadas via History Sync Request (se disponíveis)
- Mídia completa não é transferida (apenas thumbnails)

### Chamadas

**Limitação atual**: Evolution GO **não suporta atender** chamadas de voz/vídeo.

**Suportado**:
- ✅ Receber notificação de chamada (evento `CALL`)
- ✅ Rejeitar chamada automaticamente

**NÃO suportado**:
- ❌ Atender chamada
- ❌ Áudio/vídeo em tempo real

### Sessão Expira

**Inatividade**: Se device ficar offline por **>14 dias**, sessão expira.

**Solução**: Novo pareamento via QR Code necessário.

---

## Vantagens

### 1. Independência do Celular

**Evolution GO funciona mesmo com celular offline** (após sincronização inicial).

**Casos de uso**:
- Celular sem bateria
- Celular sem internet
- Viagem internacional (celular desligado)

### 2. Sincronização em Tempo Real

**Todas as mensagens** aparecem em todos os devices simultaneamente.

**Exemplo**:
- Enviar mensagem no Evolution GO
- Aparece instantaneamente no WhatsApp Web
- Aparece no celular
- Aparece no Desktop

### 3. Múltiplas Contas Simultâneas

**Com containers Docker**, você pode ter **N instâncias** do Evolution GO, cada uma conectada a um número WhatsApp diferente:

```bash
docker run -d --name evo-vendas evolution-go
docker run -d --name evo-suporte evolution-go
docker run -d --name evo-marketing evolution-go
```

Cada container = 1 número WhatsApp separado.

### 4. Alta Disponibilidade

**Redundância**: Se um device falha, outros continuam funcionando.

**Load Balancing**: Distribua carga entre múltiplos Evolution GO instances.

---

## Comparação com Versão Antiga

| Aspecto | Legacy (Web WhatsApp) | Multi-Device (Atual) |
|---------|----------------------|----------------------|
| **Celular precisa online?** | ✅ Sim, sempre | ❌ Não (após pareamento) |
| **Criptografia** | E2EE | E2EE (melhorada) |
| **Sincronização** | Relay via celular | Peer-to-peer + servers |
| **Histórico** | Depende do celular | 3 meses sincronizados |
| **Latência** | Alta (2 hops) | Baixa (direto) |
| **Devices simultâneos** | 1 (Web) | 4 companions |
| **Chamadas** | Não suportado | Não suportado* |
| **Mídia** | Relay via celular | Direct download |

*Chamadas não suportadas em companion devices (limitação WhatsApp).


---

## Próximos Passos

- [Conexão QR Code](./qrcode-connection.md) - Processo de pareamento
- [Sistema de Eventos](./events-system.md) - Receber eventos Multi-Device
- [Instâncias WhatsApp](../conceitos-core/instances.md) - Gerenciamento de devices
- [Banco de Dados](../conceitos-core/database.md) - Armazenamento de sessões

---

**Documentação gerada para Evolution GO v1.0**
