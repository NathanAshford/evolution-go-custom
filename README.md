<h1 align="center">Evolution GO Custom</h1>

<p align="center">
  <b>API de WhatsApp em Go, self-hosted</b> — com mensagens interativas que renderizam
  <b>em todo lugar</b>: botões, listas e carrosséis deslizáveis no <b>Android, iOS e WhatsApp Web/Desktop</b>.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" />
  <img alt="Docker" src="https://img.shields.io/badge/Docker-pronto-2496ED?logo=docker&logoColor=white" />
  <img alt="Ubuntu" src="https://img.shields.io/badge/Ubuntu-instala%C3%A7%C3%A3o%201--clique-E95420?logo=ubuntu&logoColor=white" />
  <img alt="Self-hosted" src="https://img.shields.io/badge/Self--hosted-sem%20PaaS-32c766" />
  <img alt="Licença" src="https://img.shields.io/badge/Licen%C3%A7a-Apache%202.0-blue" />
</p>

---

## ⚡ Instale em um comando

Em um **Ubuntu Server / VPS** novo (como root):

```bash
curl -fsSL https://raw.githubusercontent.com/NathanAshford/evolution-go-custom/main/install.sh | sudo bash
```

Só isso. O instalador configura o Docker, compila a aplicação a partir do código-fonte,
gera segredos fortes e sobe tudo — e no final imprime a **URL do Manager** e a sua **API key**.
Sem Coolify, sem painel de controle, sem contas externas. [Detalhes abaixo.](#-instalação)

---

## ✨ Destaques

- 🎛️ **Mensagens interativas que renderizam em todo lugar** — botões, listas e
  carrosséis aparecem corretamente no **Android, iOS e WhatsApp Web/Desktop**, não em só uma plataforma.
- 🔘 **Tipos de botão ricos** — `reply`, `url`, `call`, `copy` **e o botão nativo `pix`** de pagamento.
- 🎠 **Endpoint de carrossel** — envie cards deslizáveis (imagem/vídeo + texto + botões),
  com geração automática de thumbnail para carregamento instantâneo da imagem.
- 🩹 **Erro 463 do WhatsApp resolvido** — o tratamento dos tokens NativeFlow
  (`tctoken`/`cstoken`) que fazia botões/carrosséis falharem foi corrigido, então os
  envios interativos chegam de forma confiável.
- 🔑 **Pareamento por Passkey (WebAuthn)** — suporte ao novo fluxo de passkey do
  WhatsApp (quando o servidor exige passkey para vincular o aparelho): uma extensão
  de navegador roda a cerimônia WebAuthn no `web.whatsapp.com` e o instalador já
  configura a `PASSKEY_PUBLIC_URL` — funciona de fábrica.
- 🌐 **Proxy por instância** — roteie cada instância por seu próprio proxy `http`,
  `https` ou `socks5` (com ou sem autenticação), definido ou removido **em tempo real** —
  e que vale até no pareamento por QR Code.
- 🪝 **Múltiplos webhooks por instância** — distribua os eventos para vários endpoints de forma independente.
- 🖼️ **Mídia flexível** — envie por URL pública **ou** base64; imagens, vídeo, áudio,
  documentos, GIFs, figurinhas, localização, contatos e enquetes.
- 📣 **Status do WhatsApp** — publique texto/imagem/vídeo em `status@broadcast`.
- 🔌 **Eventos do seu jeito** — Webhook, WebSocket, RabbitMQ (AMQP) e NATS.
- 🔒 **Privado por padrão** — **sem telemetria**, segredos gerados automaticamente,
  banco de dados nunca exposto à internet, roda 100% offline (sem servidor de licença).
- 🚀 **Self-hosting em 1 clique** — um único script transforma uma VPS Ubuntu limpa numa API rodando.

---

## 🎛️ Mensagens interativas

É o que diferencia o Evolution GO Custom. Toda requisição é autenticada com o header
`apikey`. Conecte uma instância primeiro (`/instance/create` → `/instance/qr`) e depois:

### Botões — `POST /send/button`

**Botões de resposta** (até 3):

```jsonc
{
  "number": "5582988898565",
  "title": "Oferta especial",
  "description": "Confira as condições abaixo",
  "footer": "Sua Empresa",
  "buttons": [
    { "type": "reply", "displayText": "Quero saber mais", "id": "btn_info" },
    { "type": "reply", "displayText": "Falar com vendas", "id": "btn_vendas" }
  ]
}
```

**Botões de ação** (`url` / `call` / `copy`):

```jsonc
{
  "number": "5582988898565",
  "title": "Links úteis",
  "description": "Escolha uma ação",
  "footer": "Sua Empresa",
  "buttons": [
    { "type": "url",  "displayText": "Abrir site",   "url": "https://exemplo.com" },
    { "type": "call", "displayText": "Ligar agora",  "phoneNumber": "+5582988898565" },
    { "type": "copy", "displayText": "Copiar cupom", "copyCode": "PROMO2026" }
  ]
}
```

**Botão Pix** (pagamento instantâneo — paga em um toque):

```jsonc
{
  "number": "5582988898565",
  "title": "Pagamento",
  "description": "Pague com Pix em um toque",
  "footer": "Sua Loja",
  "buttons": [
    { "type": "pix", "currency": "BRL", "name": "Sua Loja", "keyType": "cpf", "key": "12345678900" }
  ]
}
```

> **Regras dos botões** (validadas pela API): até **3 botões `reply`**; `reply` não pode
> ser misturado com outros tipos; um botão `pix` deve ser o único da mensagem.
> No **WhatsApp Web**, evite misturar `reply` com botões de ação — envie só-reply *ou* só-ação.

### Menu de lista — `POST /send/list`

Um menu de seleção única agrupado em seções (usa o formato nativo `ListMessage` para
máxima compatibilidade):

```jsonc
{
  "number": "5582988898565",
  "title": "Nossos planos",
  "description": "Escolha o plano ideal para você",
  "buttonText": "Abrir menu",
  "footerText": "Sua Empresa",
  "sections": [
    {
      "title": "Planos",
      "rows": [
        { "title": "Básico", "description": "R$ 29,90/mês", "rowId": "plano_basico" },
        { "title": "Pro",    "description": "R$ 59,90/mês", "rowId": "plano_pro" }
      ]
    }
  ]
}
```

### Carrossel — `POST /send/carousel`

Cards deslizáveis, cada um com mídia, texto e botões próprios:

```jsonc
{
  "number": "5582988898565",
  "body": "Confira nossas novidades!",
  "footer": "Sua Empresa",
  "cards": [
    {
      "header": { "imageUrl": "https://picsum.photos/seed/card1/600/400", "title": "Oferta do dia" },
      "body":   { "text": "Card 1 — oferta especial" },
      "footer": "Por tempo limitado",
      "buttons": [
        { "type": "URL",   "displayText": "Comprar",   "id": "https://exemplo.com/produto1" },
        { "type": "REPLY", "displayText": "Mais infos", "id": "card1_info" }
      ]
    }
  ]
}
```

> Nos cards do carrossel, os botões `url`/`call` levam o destino no campo `id`
> (não há campos `url`/`phoneNumber` separados). Pix não está disponível dentro dos cards.

---

## 🌐 Proxy por instância

Dê a cada instância do WhatsApp seu próprio proxy de saída — perfeito para distribuir
números por IPs diferentes. O protocolo do proxy é detectado automaticamente pela porta,
ou você pode definir explicitamente (`http`, `https`, `socks5`), com ou sem autenticação.

**Definir / atualizar um proxy** (aplicado na hora; a instância reconecta por ele —
inclusive no pareamento por QR):

```jsonc
POST /instance/proxy/{instanceId}
{
  "protocol": "socks5",          // opcional: http | https | socks5 (inferido pela porta se omitido)
  "host": "proxy.exemplo.com",
  "port": "1080",
  "username": "usuario",         // opcional (proxies sem autenticação também funcionam)
  "password": "senha"            // opcional
}
```

**Remover o proxy:**

```
DELETE /instance/proxy/{instanceId}
```

Você também pode definir um **proxy padrão para todas as instâncias** pelas variáveis de
ambiente (`PROXY_PROTOCOL`, `PROXY_HOST`, `PROXY_PORT`, `PROXY_USERNAME`, `PROXY_PASSWORD`).

---

## 🚀 Instalação

### Um comando (recomendado)

```bash
curl -fsSL https://raw.githubusercontent.com/NathanAshford/evolution-go-custom/main/install.sh | sudo bash
```

O instalador é **100% autônomo** — você roda e já sai usando. Ele:

1. Cria **swap automaticamente** se a RAM for baixa (evita travar o build em VPS de 1 GB)
2. Instala **Docker Engine + Compose** se não existirem
3. Clona este repositório em `/opt/evolution-go`
4. Gera uma **API key** e uma **senha de banco** aleatórias e fortes
5. **Compila a imagem a partir do código-fonte** e sobe a stack completa
6. **Abre a porta da app (e o SSH)** no firewall, se houver um ativo
7. Imprime a **URL do Manager** e a sua **API key**, com um comando pronto pra criar sua 1ª instância

### Opções

Coloque variáveis de ambiente antes do comando para personalizar:

```bash
# Porta customizada + firewall automático (UFW: libera SSH e a porta da app)
curl -fsSL https://raw.githubusercontent.com/NathanAshford/evolution-go-custom/main/install.sh \
  | sudo APP_PORT=4000 SETUP_UFW=1 bash
```

| Variável | Função | Padrão |
|---|---|---|
| `APP_PORT` | Porta em que a API/Manager escuta | `8080` |
| `INSTALL_DIR` | Local de instalação | `/opt/evolution-go` |
| `EVO_BRANCH` | Branch a implantar | `main` |
| `SETUP_UFW` | Habilitar o UFW se estiver inativo (sempre libera a porta se já houver firewall ativo) | `0` |
| `CREATE_SWAP` | Criar swap automaticamente em VPS de pouca RAM (`0` para desligar) | `auto` |
| `SWAP_SIZE` | Tamanho do swap criado | `2G` |

### Instalação manual (inspecionar antes)

```bash
git clone https://github.com/NathanAshford/evolution-go-custom.git
cd evolution-go-custom
sudo ./install.sh
```

### Gerenciando a stack

```bash
cd /opt/evolution-go/deploy
docker compose ps            # status
docker compose logs -f       # acompanhar logs
docker compose restart       # reiniciar
docker compose down          # parar (mantém os dados)
docker compose up -d --build # atualizar após um git pull
```

> **Requisitos:** uma VPS Ubuntu 20.04+/Debian 11+ 64-bit. A primeira compilação
> (do binário Go) usa ~2 GB de RAM — em VPS de 1 GB o instalador **cria swap
> automaticamente** para não travar. O painel (Manager) já vem pré-compilado, então
> o build é mais leve. Depois, o consumo de memória em execução é baixo.

Quando estiver no ar, abra a **interface do Manager** em `http://SEU_IP:8080/manager` e a
**referência interativa da API (Swagger)** em `http://SEU_IP:8080/swagger/index.html`.

---

## ⚙️ Configuração

Os segredos ficam em `/opt/evolution-go/deploy/.env` (gerado na primeira execução, com
permissão `600`). O essencial:

| Variável | Descrição | Padrão |
|---|---|---|
| `SERVER_PORT` | Porta HTTP | `8080` |
| `GLOBAL_API_KEY` | API key mestra (enviada no header `apikey`) | **obrigatória** |
| `POSTGRES_AUTH_DB` / `POSTGRES_USERS_DB` | Strings de conexão do PostgreSQL | automático |
| `DATABASE_SAVE_MESSAGES` | Persistir mensagens no banco | `false` |
| `WEBHOOK_URL` | Endpoint de webhook padrão | vazio |
| `PROXY_PROTOCOL` / `PROXY_HOST` / `PROXY_PORT` | Proxy opcional (`http`/`https`/`socks5`) | vazio |
| `MINIO_ENABLED` | Armazenar mídia no MinIO/S3 | `false` |

Você também pode configurar webhooks **por instância** (vários endpoints) via
`POST /instance/webhooks/{instanceId}`.

---

## 📡 Visão geral da API

Todos os endpoints exigem o header `apikey`. Principais:

| Área | Endpoints |
|---|---|
| **Instâncias** | `POST /instance/create`, `GET /instance/qr`, `GET /instance/status`, `POST /instance/connect`, `DELETE /instance/logout` |
| **Envio — interativo** | `POST /send/button`, `POST /send/list`, `POST /send/carousel` |
| **Envio — mídia e texto** | `POST /send/text`, `/send/link`, `/send/media` (URL ou base64), `/send/poll`, `/send/sticker`, `/send/location`, `/send/contact` |
| **Status** | `POST /send/status/text`, `POST /send/status/media` |
| **Mensagens** | `POST /message/react`, `/message/edit`, `/message/delete`, `/message/markread`, `/message/downloadmedia` |
| **Usuários** | `POST /user/check`, `/user/info`, `/user/avatar`, `/user/block`, atualização de perfil |
| **Grupos / Comunidade / Newsletter / Etiquetas** | endpoints completos de gerenciamento |
| **Webhooks (por instância)** | `GET/POST/DELETE /instance/webhooks/{instanceId}` |
| **Proxy (por instância)** | `POST /instance/proxy/{instanceId}` |

A referência completa e sempre atualizada é o Swagger embutido em `/swagger/index.html`.

---

## 🔒 Segurança e privacidade

- **Sem telemetria.** A instância nunca "liga para casa" — nada sobre o seu tráfego sai do seu servidor.
- **Segredos gerados automaticamente.** A API key e a senha do banco são aleatórias por instalação e salvas com permissão `600`.
- **Banco de dados privado.** O PostgreSQL só é acessível pela aplicação na rede interna do Docker — nunca é publicado na internet.
- **Roda offline.** Não exige servidor de licença nem ativação externa.
- Coloque por trás de um reverse proxy (Nginx/Caddy/Traefik) para adicionar HTTPS na frente da API.

---

## 🧱 Stack

| Componente | Tecnologia |
|---|---|
| Linguagem | Go 1.25 |
| HTTP | Gin |
| Motor do WhatsApp | whatsmeow (vendorizado) |
| Banco de dados | PostgreSQL + GORM |
| Eventos | Webhook · WebSocket · RabbitMQ · NATS |
| Armazenamento de mídia | MinIO / S3 (opcional) |
| Interface do Manager | React (embutido na imagem) |
| Empacotamento | Docker + Docker Compose |

---

## 🙏 Agradecimentos

Construído sobre o excelente trabalho open-source do projeto oficial
**[Evolution](https://github.com/evolution-foundation)** e da biblioteca
**[whatsmeow](https://github.com/tulir/whatsmeow)**, de Tulir Asokan.
Muito obrigado a ambos — este fork está apoiado nos ombros deles.

---

## 📄 Licença

Licenciado sob a **Apache License 2.0** (com os avisos de proteção de marca do projeto
original preservados). Veja [LICENSE](./LICENSE) e [NOTICE](./NOTICE) para os detalhes completos.
