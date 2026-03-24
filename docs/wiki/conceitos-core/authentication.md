# Autenticação

Sistema de autenticação do Evolution GO usando chaves de acesso (API Keys).

## 📋 Índice

- [Visão Geral](#visão-geral)
- [Dois Tipos de Chave](#dois-tipos-de-chave)
- [API Key Global](#api-key-global)
- [Token de Instância](#token-de-instância)
- [Como Usar](#como-usar)
- [Fluxos de Autenticação](#fluxos-de-autenticação)
- [Segurança](#segurança)

---

## Visão Geral

O Evolution GO usa **chaves de acesso** (API Keys) para proteger sua API. Pense nisso como senhas especiais que você precisa enviar em cada requisição.

### Como Funciona

Imagine um prédio com dois tipos de chave:
- **Chave Master (Admin)**: Abre todas as portas, permite criar/deletar salas
- **Chave Individual (Instância)**: Abre apenas uma sala específica

É assim que funciona a autenticação no Evolution GO!

**Importante**: Não usamos login/senha tradicional, JWT ou cookies. Apenas chaves simples!

---

## Dois Tipos de Chave

### 1. API Key Global (Admin)

**O que é**: A chave mestre do sistema. Quem tem essa chave controla tudo.

**Para que serve**:
- Criar novas instâncias do WhatsApp
- Deletar instâncias existentes
- Ver todas as instâncias do sistema
- Gerenciar configurações globais

**Como usar**:
```
Envie no header da requisição:
apikey: sua-chave-global-aqui
```

**Exemplo prático**:
```bash
# Criar uma instância nova
POST /instance/create
Header: apikey: minha-chave-master-123
```

### 2. Token de Instância

**O que é**: A chave individual de cada WhatsApp conectado.

**Para que serve**:
- Enviar mensagens
- Criar grupos
- Gerenciar contatos
- Todas as operações do WhatsApp dessa instância

**Como usar**:
```
Envie no header da requisição:
apikey: token-da-sua-instancia
```

**Exemplo prático**:
```bash
# Enviar uma mensagem
POST /send/text
Header: apikey: token-vendas-123
```

---

## API Key Global

### Configurando

A API Key Global é definida como uma variável de ambiente:

```env
GLOBAL_API_KEY=minha-chave-super-secreta
```

### Gerando uma Chave Segura

**Recomendado** (Linux/Mac):
```bash
# Gera uma chave aleatória forte
openssl rand -base64 32
```

Resultado exemplo:
```
dGhpc2lzYXNlY3VyZWtleXRoYXRpc3Zlcnlsb25nYW5kc2VjdXJl
```

**Não use chaves óbvias**:
- ❌ `123456`
- ❌ `admin`
- ❌ `minha-senha`

### Onde é Usado

**Endpoints que precisam da API Key Global**:
- Criar instância: `POST /instance/create`
- Deletar instância: `DELETE /instance/delete/:id`
- Listar todas: `GET /instance/all`
- Ver informações: `GET /instance/info/:id`
- Configurar proxy: `POST /instance/proxy/:id`
- Ver logs: `GET /instance/logs/:id`

---

## Token de Instância

### O que é

Cada instância do WhatsApp tem seu próprio token único. É como o CPF da instância - identifica ela no sistema.

### Como Obter

**Opção 1: Sistema gera automaticamente**
```json
POST /instance/create
{
  "name": "vendas"
}

Resposta:
{
  "id": "abc-123",
    "name": "vendas",
  "token": "token-gerado-automaticamente-xyz789"
}
```

**Opção 2: Você define o token**
```json
POST /instance/create
{
  "name": "vendas",
  "token": "meu-token-customizado-vendas"
}
```

⚠️ **Atenção**: O token deve ser único no sistema!

### Onde é Usado

**Endpoints que precisam do Token de Instância**:

**Mensagens**:
- Enviar texto: `POST /send/text`
- Enviar mídia: `POST /send/media`
- Enviar áudio: `POST /send/audio`
- Reagir: `POST /message/react`
- Marcar como lida: `POST /message/markread`

**Grupos**:
- Listar grupos: `GET /group/list`
- Criar grupo: `POST /group/create`
- Adicionar participante: `POST /group/participant`
- Sair do grupo: `POST /group/leave`

**Usuários**:
- Ver informações: `POST /user/info`
- Bloquear: `POST /user/block`
- Desbloquear: `POST /user/unblock`
- Ver contatos: `GET /user/contacts`

---

## Como Usar

### Formato do Header HTTP

Todas as requisições devem incluir o header `apikey`:

```
apikey: sua-chave-aqui
```

**NÃO use**:
```
❌ Authorization: Bearer sua-chave
❌ apikey: Bearer sua-chave
```

### Exemplos Práticos

**1. Criando uma instância (usa API Key Global)**:
```bash
curl -X POST http://localhost:4000/instance/create \
  -H "Content-Type: application/json" \
  -H "apikey: minha-chave-global" \
  -d '{"name": "vendas", "token": "token-vendas"}'
```

**2. Enviando mensagem (usa Token da Instância)**:
```bash
curl -X POST http://localhost:4000/send/text \
  -H "Content-Type: application/json" \
  -H "apikey: token-vendas" \
  -d '{
    "number": "5511999999999",
    "text": "Olá!"
  }'
```

**3. Com JavaScript**:
```javascript
fetch('http://localhost:4000/send/text', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'apikey': 'token-vendas'
  },
  body: JSON.stringify({
    number: '5511999999999',
    text: 'Olá!'
  })
});
```

---

## Fluxos de Autenticação

### Fluxo 1: Criar Instância

```
1. Você ─────> API
   POST /instance/create
   apikey: CHAVE-GLOBAL

2. API verifica ─────> ✓ Chave Global OK

3. API cria instância ─────> Banco de Dados

4. API retorna ─────> Você
   {
     "name": "vendas",
     "token": "token-vendas-123"
   }
```

### Fluxo 2: Enviar Mensagem

```
1. Você ─────> API
   POST /send/text
   apikey: token-vendas-123

2. API busca ─────> Banco de Dados
   "Qual instância tem esse token?"

3. Banco responde ─────> API
   "É a instância 'vendas'"

4. API envia mensagem ─────> WhatsApp

5. API retorna ─────> Você
   {"status": "success"}
```

### Fluxo 3: Chave Inválida

```
1. Você ─────> API
   POST /send/text
   apikey: token-errado

2. API busca ─────> Banco de Dados
   "Qual instância tem esse token?"

3. Banco responde ─────> API
   "Token não encontrado!"

4. API retorna ERRO ─────> Você
   401 Unauthorized
   {"error": "not authorized"}
```

---

## Segurança

### 1. Protegendo a API Key Global

**❌ NUNCA faça isso**:
- Compartilhar no Slack, email ou WhatsApp
- Salvar em arquivos públicos no GitHub
- Usar valores óbvios como "admin" ou "123456"
- Colocar no código-fonte

**✅ SEMPRE faça isso**:
- Salvar em arquivo `.env` (e adicionar ao `.gitignore`)
- Usar gerenciadores de secrets (Vault, AWS Secrets)
- Gerar chaves fortes (32+ caracteres aleatórios)
- Rotacionar periodicamente (trocar a chave)

### 2. Protegendo Tokens de Instância

**❌ NUNCA faça isso**:
- Colocar tokens na URL (`?token=...`)
- Salvar tokens em logs
- Expor tokens em páginas públicas

**✅ SEMPRE faça isso**:
- Enviar apenas via header HTTP
- Usar HTTPS em produção
- Guardar em local seguro (variáveis de ambiente)

### 3. Use HTTPS em Produção

⚠️ **CRÍTICO**: Em produção, sempre use HTTPS!

```
✅ https://api.suaempresa.com/send/text
❌ http://api.suaempresa.com/send/text
```

Sem HTTPS, suas chaves trafegam em **texto puro** pela internet e podem ser interceptadas.

### 4. Rotação de Chaves

Se uma chave foi comprometida:

**Para API Key Global**:
1. Gere uma nova chave forte
2. Atualize a variável `GLOBAL_API_KEY` no servidor
3. Reinicie a aplicação
4. Atualize todos os clientes que usam a API

**Para Token de Instância**:
1. Crie uma nova instância com novo token
2. Migre seus dados
3. Delete a instância antiga

💡 Não há como "trocar" um token existente - você precisa criar uma nova instância.

### 5. Monitorando Acessos

**Sinais de problema**:
- Muitas tentativas com chaves inválidas
- Acessos de IPs desconhecidos
- Horários estranhos de acesso

**Recomendação**: Configure logs e alertas para tentativas de acesso não autorizado.

---

## Exemplos do Dia a Dia

### Cenário 1: Primeiro Uso

```bash
# 1. Configure a chave global no servidor
echo "GLOBAL_API_KEY=$(openssl rand -base64 32)" >> .env

# 2. Inicie o servidor
docker-compose up -d

# 3. Crie sua primeira instância
curl -X POST http://localhost:4000/instance/create \
  -H "apikey: $(grep GLOBAL_API_KEY .env | cut -d '=' -f2)" \
  -H "Content-Type: application/json" \
  -d '{"name": "vendas"}'

# Guarde o token que foi retornado!
```

### Cenário 2: Múltiplas Instâncias

```bash
# Vendas
curl -X POST http://localhost:4000/send/text \
  -H "apikey: token-vendas" \
  -d '{"number": "5511111111", "text": "Equipe vendas"}'

# Suporte
curl -X POST http://localhost:4000/send/text \
  -H "apikey: token-suporte" \
  -d '{"number": "5522222222", "text": "Equipe suporte"}'
```

Cada instância é independente e usa seu próprio token!

---

## Troubleshooting

### Erro: "not authorized"

**Possíveis causas**:
1. Header `apikey` está faltando
2. Valor da chave está errado
3. Usando chave global onde precisa token de instância (ou vice-versa)

**Como resolver**:
```bash
# Verifique se o header está sendo enviado
curl -v http://localhost:4000/send/text

# Confirme o valor no .env
cat .env | grep GLOBAL_API_KEY

# Liste suas instâncias para ver os tokens
curl -H "apikey: sua-chave-global" http://localhost:4000/instance/all
```

### Token não funciona após reiniciar

**Causa**: Tokens ficam salvos no banco e não mudam.

**Solução**: Use o mesmo token que foi fornecido na criação da instância.

### Esqueci minha API Key Global

**Solução**: 
1. Acesse o servidor
2. Veja o arquivo `.env`
3. Procure por `GLOBAL_API_KEY`

---

## Resumo Rápido

| Tipo | Quando Usar | Exemplo |
|------|-------------|---------|
| **API Key Global** | Criar/deletar instâncias | `apikey: minha-chave-master` |
| **Token Instância** | Enviar mensagens, criar grupos | `apikey: token-vendas-123` |

**Lembre-se**:
- 🔑 API Key Global = Chave mestre (admin)
- 🎫 Token Instância = Chave de uma sala específica
- 🔒 Sempre use HTTPS em produção
- 📝 Guarde suas chaves em local seguro
- 🚫 Nunca compartilhe chaves publicamente

---

**Documentação Evolution GO v1.0**
