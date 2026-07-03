 # Pareamento por Chave de Acesso (Passkey / WebAuthn)

Guia completo de como conectar uma instância WhatsApp no Evolution GO quando a
conta exige uma **chave de acesso (passkey)** — a nova etapa de segurança que a
Meta passou a exigir em algumas contas ao vincular um novo dispositivo.

---

## 1. Por que o passkey existe

O fluxo clássico multi-dispositivo era: mostra o QR rotativo → o celular escaneia
→ pareado. A Meta está lançando o **linking travado por passkey** (o protocolo
que os clients oficiais chamam de **Shortcake** no lado web / **CRSC** no lado
telefone): para **algumas contas**, o servidor exige uma **assertion de passkey
WebAuthn** antes de vincular um novo dispositivo.

**Quem é afetado:** é uma decisão **100% server-side, por conta** (um bucket A/B
do servidor + a conta ter um passkey + sinal de risco). O usuário não escolhe, e
o cliente não consegue disparar nem desviar. Você não controla se sua conta cai
nesse fluxo.

**Importante — nem sempre há QR.** Numa conta travada, o servidor responde ao
pareamento com `retry-with-method: shortcake-with-passkeys` e frequentemente
**omite o `qr-code`** — ou seja, **pode não existir QR para escanear**. O prólogo
de passkey também pode vir logo após um **código de pareamento**. Por isso o
Evolution GO trata o pedido de passkey de forma independente do QR.

O ponto difícil: a chamada `navigator.credentials.get()` (que dispara a
biometria/PIN) **só funciona numa página cujo domínio bata com o `rpId` da
credencial** (`whatsapp.com`) — ou seja, precisa rodar dentro de uma aba real
`web.whatsapp.com`. **Não existe bypass headless** (confirmado empiricamente por
pesquisa de engenharia reversa independente — ver §10): o servidor valida a
assinatura contra a chave pública **registrada** da conta, com challenge fresco;
uma assertion forjada é rejeitada. A cerimônia exige o autenticador real do dono
da conta (Touch ID / Windows Hello / passkey sincronizada).

> **É web-only.** Um companion android (`platform_type = 10`) é rejeitado pelo
> servidor com `463 account_reachout_restricted`. A cerimônia Shortcake só roda
> num companion web.

**Solução do Evolution GO:** uma pequena extensão de navegador
(`tools/passkey-helper`) roda **apenas** em `web.whatsapp.com`, executa a
cerimônia WebAuthn ali (onde o `rpId` é aceito), e devolve a assinatura para a
API do Evolution GO, que a repassa ao WhatsApp via whatsmeow.

> A extensão é o único componente que roda no domínio `web.whatsapp.com`. Ela
> não interage com o login do WhatsApp Web e não coleta dados — serve só como
> "palco" com o origin correto para a biometria.

---

## 2. Arquitetura

```
┌──────────────────────────── Evolution GO (backend) ────────────────────────────┐
│                                                                                 │
│  whatsmeow client ──emite──> events.PairPasskeyRequest / Confirmation / Error   │
│         │                            │                                          │
│         │                            ▼                                          │
│         │                    ceremony.Store (token efêmero + estágio, mutex)    │
│         │                            ▲                                          │
│         ▼                            │                                          │
│  SendPasskeyResponse   ◄──── API pública /passkey-ceremony/{token}              │
│  SendPasskeyConfirmation      (CORS libera https://web.whatsapp.com)            │
└─────────────────────────────────────────────────────────────────────────────────┘
              ▲                                          │
              │ POST assertion / confirm                 │ GET desafio
              │                                          ▼
   ┌──────────────────────── Extensão (content script) ─────────────────────────┐
   │  roda SÓ em https://web.whatsapp.com/*                                       │
   │  lê o token+baseUrl do hash da URL → GET desafio → navigator.credentials.get │
   │  → POST da assinatura de volta                                               │
   └──────────────────────────────────────────────────────────────────────────────┘
              ▲
              │ abre https://web.whatsapp.com/#wapk=<payload>
   ┌──────────────────────────────── Manager ───────────────────────────────────┐
   │  QRCodeModal detecta passkeyStage no poll do QR → botão "Abrir WhatsApp Web" │
   └──────────────────────────────────────────────────────────────────────────────┘
```

**Papéis:**

1. **whatsmeow** — emite os eventos de passkey e expõe `SendPasskeyResponse` /
   `SendPasskeyConfirmation`.
2. **Backend Evolution GO** — guarda o estado da cerimônia (por token efêmero) e
   expõe a API pública com CORS que o `web.whatsapp.com` chama.
3. **Extensão** — a única peça no origin `web.whatsapp.com`; executa a cerimônia
   WebAuthn.
4. **Manager** — abre a URL da cerimônia e faz polling até conectar.

---

## 3. Sequência ponta a ponta

1. Você cria a instância e clica em conectar.
   - **Conta normal:** o QR aparece, você escaneia, conecta — e o passkey **nunca
     é acionado**. Este guia não se aplica.
   - **Conta travada por passkey:** pode aparecer um QR primeiro (e o passkey vir
     depois do escaneio) **ou não haver QR algum** — o servidor decide. De
     qualquer forma, o próximo passo é o passkey.
2. Quando o servidor pede o passkey, o whatsmeow emite `PairPasskeyRequest`
   (independente de ter havido QR). O backend:
   - gera um **token de cerimônia** efêmero (32 bytes, TTL 5 min);
   - guarda o desafio WebAuthn (`publicKey`) associado a esse token;
   - loga a URL `#wapk` pronta e a expõe no poll do QR.
3. O manager detecta `passkeyStage` e mostra o painel de passkey, com o botão
   **"Abrir WhatsApp Web"** (aponta para
   `https://web.whatsapp.com/#wapk=<payload>`).
4. A extensão (na aba `web.whatsapp.com`) lê o payload, busca o desafio na API,
   executa `navigator.credentials.get()` (biometria/PIN) e **POSTa a
   assinatura** para `/passkey-ceremony/{token}/response`.
5. O backend chama `SendPasskeyResponse`. O servidor valida a assertion contra a
   chave pública registrada e responde com um **código de confirmação** →
   `PairPasskeyConfirmation` → o backend guarda o código no estado da cerimônia.
6. A extensão mostra o código; o usuário confere e clica em confirmar → a
   extensão chama `/passkey-ceremony/{token}/confirm` → o backend chama
   `SendPasskeyConfirmation`.
7. O pareamento se completa: `PairSuccess` / `Connected`. O backend limpa a
   cerimônia. O manager, ainda em polling, detecta `connected: true`.

> **Nota sobre o código de confirmação (handoff).** Em alguns casos o telefone
> pula a tela de conferência de código (quando um "handoff proof" válido é
> enviado). O Evolution GO **sempre** exige a confirmação manual
> (`skipHandoffUX=false`) — confirmar automaticamente pode dessincronizar o
> pareamento e disparar o anti-abuso do WhatsApp ("Não foi possível conectar o
> dispositivo").

---

## 4. Configuração obrigatória

### `PASSKEY_PUBLIC_URL`

A extensão roda no navegador e precisa chamar a **API do Evolution GO**. A URL
base dessa API precisa ser **acessível pelo navegador** onde o WhatsApp Web
abre. Defina no `.env`:

```env
# URL pública da API que o navegador (aba web.whatsapp.com) consegue alcançar.
# Em produção: o domínio público da API.
# Em desenvolvimento: um túnel (ngrok/cloudflared) OU o IP de LAN da máquina.
#   NÃO use localhost se o navegador estiver em outra máquina.
PASSKEY_PUBLIC_URL=https://sua-api.exemplo.com
```

> Se `PASSKEY_PUBLIC_URL` não estiver setado, a URL `#wapk` sai com o
> placeholder `<SET_PASSKEY_PUBLIC_URL>` e o manager exibe um aviso, com o botão
> "Abrir WhatsApp Web" desabilitado.

### Instalar a extensão (uma vez por navegador)

A extensão fica na pasta **`passkey-helper/`** do repositório público (Manifest
V3, Chromium — Chrome/Edge). No repositório de desenvolvimento ela está em
`tools/passkey-helper/`.

**Chrome:** `chrome://extensions` → ativar **Modo do desenvolvedor** → **Carregar
sem compactação** → selecionar a pasta `passkey-helper`.

**Edge:** `edge://extensions` → **Modo de desenvolvedor** → **Carregar sem
pacote** → selecionar a pasta `passkey-helper`.

> Use o navegador logado na mesma conta (Google/iCloud) onde a passkey do
> WhatsApp está salva, ou tenha o celular por perto para a biometria.

### CORS

O backend já libera a origem `https://web.whatsapp.com` (o CORS atual usa `*`,
que cobre chamadas não-credenciadas da extensão). As rotas `/passkey-ceremony/*`
também estão isentas do gate de licença.

---

## 5. Guia rápido para testar (comunidade)

Recurso **novo (0.7.2)** e ainda **não validado ponta a ponta** — poucas contas
foram travadas por passkey até agora. Se a sua conta pede passkey, siga o
passo-a-passo abaixo e reporte o resultado (funcionou / em qual etapa parou +
logs) numa issue do repositório. Isso ajuda muito.

**Como saber se preciso disso:** você tenta conectar a instância e (a) o QR não
completa o pareamento mesmo escaneado, ou (b) nem aparece QR, ou (c) o app do
WhatsApp reclama de "chave de acesso" / "não foi possível conectar o
dispositivo". Nesses casos a conta provavelmente está travada por passkey.

**Pré-requisitos:**

1. Evolution GO **0.7.2+** rodando.
2. `PASSKEY_PUBLIC_URL` no `.env` apontando para uma URL **pública** da API,
   alcançável pelo navegador (ver §4). Em teste local use um túnel (ngrok /
   cloudflared) — **não** `localhost` se o navegador estiver em outra máquina.
3. A **extensão** `tools/passkey-helper` instalada no Chrome/Edge (ver §4).
4. O navegador logado na mesma conta (Google/iCloud) onde a passkey do WhatsApp
   está salva, **ou** o celular por perto para a biometria.

**Passo a passo:**

1. No manager, crie a instância e clique em conectar.
2. Se aparecer QR, escaneie. Se a conta for travada, o painel muda para
   **"Chave de acesso (passkey)"** (ou não haverá QR, indo direto para esse
   painel).
3. Clique em **"Abrir WhatsApp Web"**. Abre uma aba `web.whatsapp.com` com o
   painel da extensão no canto.
4. Na aba, clique em **"Autenticar com chave de acesso"** e confirme com
   biometria/PIN.
5. Se aparecer um **código de confirmação**, confira que ele bate com o do
   celular e confirme.
6. Volte ao manager e aguarde a instância ficar **conectada**.

**Sem o manager (teste manual via API):** o backend loga a URL `#wapk` pronta
quando o passkey é pedido (procure por `Passkey required. Open this URL...` nos
logs). Copie essa URL, abra no navegador com a extensão, e complete a cerimônia.
Os endpoints estão na §6.

---

## 6. Referência das requisições

Todas as rotas de cerimônia são **públicas** (sem `apikey`): a segurança está no
token de cerimônia — opaco, imprevisível, de uso único e com expiração. Sem um
token válido não há nada a fazer, e concluir a cerimônia ainda exige o
autenticador real do usuário.

`{base}` = `PASSKEY_PUBLIC_URL`. `{token}` = token de cerimônia.

### 5.1 Consultar o estado da cerimônia

```
GET {base}/passkey-ceremony/{token}
```

Resposta (200):

```json
{
  "stage": "challenge",
  "publicKey": {
    "challenge": "<base64url>",
    "timeout": 60000,
    "rpId": "whatsapp.com",
    "allowCredentials": [
      { "type": "public-key", "id": "<base64url>", "transports": ["internal"] }
    ],
    "userVerification": "required"
  },
  "skipHandoffUX": false,
  "code": "",
  "error": ""
}
```

- `404` se o token for desconhecido ou expirado.

**Estágios (`stage`):**

| stage                   | significado                                                        |
| ----------------------- | ------------------------------------------------------------------ |
| `challenge`             | desafio disponível; a extensão executa `navigator.credentials.get` |
| `awaiting_confirmation` | assinatura enviada; aguardando o código do servidor                |
| `confirmation`          | código disponível (`code`); usuário deve conferir e confirmar      |
| `confirmed`             | confirmação enviada; pareamento concluindo                         |
| `error`                 | falhou (ver `error`)                                               |

### 5.2 Enviar a assinatura WebAuthn

```
POST {base}/passkey-ceremony/{token}/response
Content-Type: application/json
```

Corpo (produzido por `navigator.credentials.get()`, tudo em base64url **sem
padding**):

```json
{
  "id": "<credential id>",
  "rawId": "<base64url>",
  "type": "public-key",
  "response": {
    "clientDataJSON": "<base64url>",
    "authenticatorData": "<base64url>",
    "signature": "<base64url>",
    "userHandle": "<base64url|opcional>"
  }
}
```

Resposta: `200 {"ok": true}`. Em erro: `4xx/5xx {"error": "..."}`.

Internamente chama `client.SendPasskeyResponse(ctx, *types.WebAuthnResponse)`. O
servidor então emite, de forma assíncrona, `PairPasskeyConfirmation` (código) ou
`PairPasskeyError`.

### 5.3 Confirmar o código

```
POST {base}/passkey-ceremony/{token}/confirm
```

Resposta: `200 {"ok": true}`. Chama `client.SendPasskeyConfirmation(ctx)`; o
pareamento então completa com `PairSuccess` / `Connected`.

---

## 7. Como o manager expõe isso

O manager **não** tem um endpoint dedicado — ele reaproveita o poll do QR que já
existe. O endpoint autenticado do QR (`GET /instance/qr`, header `apikey`)
retorna, quando há uma cerimônia ativa:

```json
{
  "message": "success",
  "data": {
    "qrcode": "",
    "code": "",
    "passkeyStage": "challenge",
    "passkeyOpenUrl": "https://web.whatsapp.com/#wapk=...",
    "passkeyCode": ""
  }
}
```

O `QRCodeModal` detecta `passkeyStage` e mostra o painel de passkey (botão
"Abrir WhatsApp Web" → `passkeyOpenUrl`, e o `passkeyCode` quando disponível).

> **Importante:** a **extensão** conduz a cerimônia inteira (desafio +
> confirmação). O manager só abre a URL e faz polling. O código de confirmação é
> exibido tanto na extensão quanto (opcionalmente) no manager para conferência.

---

## 8. Notas de implementação (para quem for mexer no backend)

Regras que **não** podem ser violadas (extraídas do comportamento do whatsmeow):

- **Não use `GetQRChannel` durante o passkey.** No whatsmeow instalado, o handler
  do `qrChannel` (a) auto-confirma a `PairPasskeyConfirmation` quando
  `SkipHandoffUX` está setado — correndo com o fluxo manual — e (b) desconecta o
  socket quando os códigos de QR acabam. Por isso o Evolution GO conecta com
  `client.Connect()` direto e consome `events.QR` no event handler
  (`handleQRCodes`), que o `pair.go` despacha para todos os handlers de qualquer
  forma.
- **Nunca auto-confirme com base em `SkipHandoffUX`.** O backend força
  `skipHandoffUX=false`, então a confirmação é sempre manual (o usuário confere o
  código). Auto-confirmar dessincroniza o pareamento e dispara o anti-abuso do
  WhatsApp.
- **`EnableAutoReconnect = false`** já está setado; a reconexão é centralizada
  pela aplicação.
- **A rotação de QR pausa durante uma cerimônia de passkey.** O `handleQRCodes`
  consulta `ceremony.Store.HasActiveByInstance()` antes de qualquer teardown, para
  não derrubar o socket enquanto o usuário completa a cerimônia (que é conduzida
  por um humano no navegador e pode demorar mais que a janela de rotação do QR).
- **`PairPasskeyRequest` NÃO chama `SendPasskeyResponse`.** No momento do request
  só existe o desafio; a assinatura só existe depois que a extensão roda
  `navigator.credentials.get()`. O `SendPasskeyResponse` é chamado apenas no
  handler HTTP `/response`.

Arquivos principais:

| Arquivo                                        | Responsabilidade                                   |
| ---------------------------------------------- | -------------------------------------------------- |
| `pkg/passkey/ceremony/store.go`                | estado da cerimônia (token → estágio), mutex, TTL  |
| `pkg/passkey/handler/passkey_handler.go`       | os 3 endpoints públicos                            |
| `pkg/whatsmeow/service/whatsmeow.go`           | eventos `PairPasskey*`, `handleQRCodes`, bridges   |
| `pkg/instance/service/instance_service.go`     | `GetQr` expõe o estado da cerimônia ao manager     |
| `tools/passkey-helper/`                        | extensão de navegador (content script + manifest)  |

---

## 9. Solução de problemas

| Sintoma                                              | Causa provável / correção                                                                 |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Botão "Abrir WhatsApp Web" desabilitado             | `PASSKEY_PUBLIC_URL` não configurado. Defina no `.env` e reinicie.                          |
| A aba `web.whatsapp.com` não mostra o painel        | Extensão não instalada / não é a aba de `web.whatsapp.com`. Instale e recarregue a aba.    |
| `ceremony not found or expired` (404)               | Token expirou (TTL 5 min). Gere um novo QR no manager para reiniciar a cerimônia.          |
| A extensão falha ao buscar o desafio (CORS/rede)    | `PASSKEY_PUBLIC_URL` não é alcançável pelo navegador. Use um túnel ou o IP de LAN correto. |
| "Não foi possível conectar o dispositivo"           | Confirmação automática indevida. O backend força `skipHandoffUX=false` — confirme manual.  |
| A assinatura é rejeitada pelo WhatsApp              | Formato base64url com padding. A extensão emite base64url **sem** padding (correto).       |

---

## 10. Segurança

- O token de cerimônia é opaco, de uso único e expira em ~5 minutos.
- As rotas de cerimônia são públicas por design: sem um token válido não há nada
  a fazer, e concluir a cerimônia exige o autenticador real do usuário.
- A extensão não coleta nem envia dados a terceiros; apenas executa a cerimônia
  WebAuthn no origin correto.
- O whatsmeow é licenciado sob MIT; os eventos e métodos de passkey fazem parte
  da biblioteca oficial (`go.mau.fi/whatsmeow`).

---

## 11. Referências e limites conhecidos

- **Não existe bypass headless.** Numa conta travada, vincular exige uma assertion
  do **próprio authenticator do dono** — obtida no navegador/dispositivo dele e
  repassada ao fluxo. Isso é *usar* o passkey real, não burlá-lo. O servidor
  valida a assinatura contra a chave pública registrada da conta (challenge
  fresco, sem replay); assertions forjadas são rejeitadas silenciosamente.
- **Web-only.** A cerimônia Shortcake só roda num companion web. Companion android
  é rejeitado (`463 account_reachout_restricted`).
- **A decisão é do servidor.** Nenhuma versão de client, flag ou capability
  desvia do passkey — é um bucket A/B por conta, server-side.
- **Engenharia reversa do protocolo:** o fluxo Shortcake/CRSC (as stanzas,
  a criptografia ECDH commit-reveal, a captura de wire correlacionada) foi
  documentado de forma independente pelo projeto **zapo** em
  <https://zapo.to/pt-br/reverse-engineering/passkey-linking> — leitura
  recomendada para quem quiser entender o protocolo por baixo. O `zapo` é um
  projeto independente, não afiliado ao WhatsApp/Meta; o Evolution GO apenas
  consome o suporte a passkey já implementado no whatsmeow oficial.
