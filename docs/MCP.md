# MCP — conectando Claude e ChatGPT ao Evolution GO

O Evolution GO expõe um servidor **MCP (Model Context Protocol)** que permite a
assistentes de IA enviar mensagens de WhatsApp e pesquisar o histórico de
conversas.

- **Endpoint:** `POST https://SEU-HOST/mcp`
- **Transporte:** Streamable HTTP (JSON-RPC 2.0)
- **Protocolo MCP:** `2024-11-05`

## Autenticação

A chave pode ser enviada de quatro formas — use a que o seu cliente suportar:

| Forma | Exemplo |
|---|---|
| Header `apikey` | `apikey: SUA_CHAVE` |
| Bearer token | `Authorization: Bearer SUA_CHAVE` |
| Token na URL | `POST /mcp/SUA_CHAVE` |
| Query string | `POST /mcp?apikey=SUA_CHAVE` |

Duas chaves são aceitas, e o **escopo muda conforme a chave**:

- **Token da instância** — o assistente fica restrito àquela instância. O
  argumento `instance` das ferramentas se torna desnecessário. **Recomendado.**
- **Chave global (admin)** — dá acesso a todas as instâncias. Nesse caso o
  argumento `instance` passa a ser obrigatório nas ferramentas.

> A chave dá poder de **enviar mensagens em seu nome**. Prefira o token da
> instância e trate-o como uma senha.

## Como conectar

### Claude (claude.ai / Desktop)

Configurações → Connectors → Add custom connector → URL:

```
https://SEU-HOST/mcp/SEU_TOKEN_DA_INSTANCIA
```

### Claude Code

```bash
claude mcp add evolution-go --transport http https://SEU-HOST/mcp \
  --header "apikey: SEU_TOKEN_DA_INSTANCIA"
```

### ChatGPT

Settings → Connectors → Create → MCP Server, e informe a mesma URL com o token
no caminho. As ferramentas `search` e `fetch` exigidas pelo modo *deep research*
já estão implementadas.

## Ferramentas disponíveis

### Leitura

| Ferramenta | O que faz |
|---|---|
| `list_instances` | Lista as instâncias e o status de conexão |
| `find_contact` | **Acha um contato ou grupo pelo nome** e devolve o número |
| `search_messages` | Busca mensagens por conteúdo, data, chat, remetente, direção ou tipo |
| `get_chat_history` | Lê as mensagens recentes de uma conversa |
| `list_chats` | Lista as conversas mais ativas com a última mensagem e o nome salvo |
| `fetch_message` | Busca uma mensagem específica pelo id |
| `search` / `fetch` | Aliases compatíveis com o *deep research* do ChatGPT |

#### Enviar pelo nome do contato

As ferramentas de envio só aceitam **número** — nome não. Para "envie oi para
Cibele Falcão", a IA faz dois passos:

1. `find_contact` com `query: "Cibele Falcão"` → devolve o `number`
2. `send_text_message` com esse `number`

**Se o cliente MCP bloquear o `find_contact`**, use o `send_message_to_contact`:

```
send_message_to_contact  name="Cibele Falcão"  text="oi"
```

Ele resolve o contato **dentro do servidor** e envia num passo só — o telefone
nunca volta para o modelo, então não há dado pessoal na conversa. Alguns
conectores (o do ChatGPT, por exemplo) interrompem a execução de ferramentas que
devolvem PII como número de telefone; este caminho evita isso.

A resposta traz só o nome e o número mascarado (`••••0031`). Se o nome casar com
mais de um contato, **nada é enviado**: volta um erro listando as opções
mascaradas, para a IA perguntar qual. Para desempatar entre homônimos existe o
`number_hint` (os últimos dígitos).

A busca ignora maiúsculas e acentos, então `falcao` acha `Falcão`. Ela procura no
nome salvo na sua agenda, no nome que a pessoa usa no WhatsApp (*push name*), no
nome de empresa e nos grupos de que a instância participa.

Se dois contatos empatarem (dois "Cibele", por exemplo), a resposta vem com
`ambiguous: true` — deliberadamente, para a IA **perguntar qual** em vez de
mandar mensagem para a pessoa errada.

> A agenda vem da sincronização do WhatsApp: uma instância que nunca conectou
> não tem contatos. Contatos `@lid` não expõem número — use o `jid` devolvido.

### Envio

| Ferramenta | O que faz |
|---|---|
| `send_text_message` | Texto simples |
| `send_message_to_contact` | Texto **pelo nome** do contato — resolve no servidor, sem expor o telefone |
| `send_media_message` | Imagem, vídeo, áudio ou documento — **por URL ou base64** |
| `send_link_message` | Texto com preview de link |
| `send_location_message` | Localização (lat/long) |
| `send_contact_message` | Cartão de contato (vCard) |
| `send_sticker_message` | Figurinha |
| `send_poll_message` | Enquete |
| `send_buttons_message` | Botões interativos |
| `send_list_message` | Menu de seleção |
| `send_carousel_message` | Carrossel de cards |
| `send_status` | Publica no status (stories) |
| `delete_message` | **Apaga para todos** (revoke) uma mensagem já enviada |
| `place_call` / `end_call` | Liga para um número e desliga (**sem áudio** — ver `docs/CALLS.md`) |

#### Mídia por URL ou base64

`send_media_message` aceita as duas formas — passe **exatamente uma**:

| Campo | Quando usar |
|---|---|
| `url` | O arquivo está acessível numa URL `http(s)` pública. O servidor baixa e envia. |
| `base64` | Você já tem os bytes. Aceita base64 puro **ou** data URI (`data:image/png;base64,...`), com ou sem padding, alfabeto padrão ou URL-safe. Quebras de linha são ignoradas. |

Prefira `url` sempre que der: base64 infla muito o payload, e num cliente MCP
isso vira token gasto pela IA.

Com `base64`, informe também o `filename` — os bytes não carregam nome, e sem ele
o destinatário recebe um documento sem nome.

> **Paridade com o REST:** o `POST /send/media` aceita base64 no próprio campo
> `url` quando o valor não começa com `http://` ou `https://`. O MCP mantém esse
> comportamento, então quem já usa esse formato não quebra — `base64` é só a
> forma explícita da mesma coisa.

`delete_message` recebe `number` + `message_id` e usa o mesmo caminho do
`POST /message/delete`. Duas limitações vêm do próprio WhatsApp: só dá para
revogar mensagens que **esta instância enviou**, e mensagens antigas demais são
recusadas. A ação é irreversível.

Todas aceitam `reply_to_message_id` (citar uma mensagem) e `delay_ms`.

### Parâmetros de botões

`send_buttons_message` — cada tipo usa campos diferentes:

| Tipo | Campos |
|---|---|
| `reply` | `display_text` + `id` (payload de callback) |
| `url` | `display_text` + `url` |
| `call` | `display_text` + `phone_number` (E.164) |
| `copy` | `display_text` + `copy_code` |
| `pix` | `currency` + `name` + `key_type` + `key` |

Regras validadas antes do envio, com erro explicando o motivo:
- no máximo **3** botões `reply`;
- `reply` **não pode** ser misturado com `url`/`call`/`copy`;
- `pix` tem que ser o **único** botão.

> ⚠️ `send_carousel_message` é **diferente**: os botões do carrossel não têm campos
> `url`/`phone_number`. Para URL e CALL, o valor vai no **`id`**. Pix não é
> suportado em carrossel.

### Filtros de `search_messages`

| Argumento | Descrição |
|---|---|
| `query` | Texto dentro da mensagem (case-insensitive, busca parcial) |
| `chat` | Número (`5511999999999`), JID completo, ou parte dele |
| `sender` | Número ou JID de quem enviou |
| `from_me` | `true` = enviadas por você, `false` = recebidas |
| `is_group` | `true` = apenas grupos, `false` = apenas conversas diretas |
| `message_type` | `text`, `image`, `video`, `audio`, `document`, `sticker`, `location`, `poll`… |
| `start_date` / `end_date` | `YYYY-MM-DD` ou RFC3339. Uma data pura cobre o dia inteiro |
| `limit` / `offset` | Paginação (máx. 500 por página) |

Exemplos de perguntas que passam a funcionar:

- *"Procure mensagens sobre boleto na instância 9761"*
- *"O que esse cliente mandou entre 1 e 5 de agosto?"*
- *"Liste as conversas mais recentes e me diga quais estão sem resposta"*
- *"Mande uma mensagem para 5511999999999 confirmando a reunião"*

## Arquivo de mensagens

A busca é servida pela tabela `archived_messages`, alimentada automaticamente:

- **Recebidas** — gravadas no handler de eventos do WhatsApp
- **Enviadas** — gravadas no envio pela API (inclusive botões)

Pontos importantes:

- **Só é possível buscar mensagens trocadas depois desta atualização.** O
  histórico anterior não é importado retroativamente.
- É armazenado apenas o **texto** (e legendas de mídia) — nunca os arquivos.
- O conteúdo é truncado em 16.000 caracteres por mensagem.
- Mensagens de protocolo/sistema são ignoradas.

Para limpar o arquivo:

```sql
DELETE FROM archived_messages WHERE timestamp < now() - interval '90 days';
```

## Teste rápido

```bash
curl -X POST https://SEU-HOST/mcp \
  -H "apikey: SEU_TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```
