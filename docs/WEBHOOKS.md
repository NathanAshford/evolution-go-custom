# Webhooks

Cada instância pode enviar seus eventos para uma ou mais URLs. Este documento
cobre como configurar, o que esperar da entrega e como diagnosticar quando algo
não chega.

## Configuração

Uma instância tem **duas** origens de webhook, e ambas recebem os mesmos eventos:

| Origem | Como define | Quantidade |
|---|---|---|
| Webhook da criação | campo `webhook` ao criar/conectar a instância | 1 |
| Lista de webhooks | `POST /instance/webhooks/{instanceId}` | várias |

```bash
# adicionar
curl -X POST "$HOST/instance/webhooks/$ID" \
  -H "apikey: $GLOBAL_API_KEY" -H "Content-Type: application/json" \
  -d '{"url":"https://meu-servidor.com/webhook"}'

# listar (inclui o webhook da criação)
curl "$HOST/instance/webhooks/$ID" -H "apikey: $GLOBAL_API_KEY"

# remover
curl -X DELETE "$HOST/instance/webhooks/$ID" \
  -H "apikey: $GLOBAL_API_KEY" -H "Content-Type: application/json" \
  -d '{"url":"https://meu-servidor.com/webhook"}'
```

Há ainda o **webhook global** (`WEBHOOK_URL` no ambiente), que recebe os eventos
de todas as instâncias. Se a mesma URL estiver como global e como webhook da
instância, ela recebe o evento **uma vez só** — a duplicata é removida.

> As alterações valem **na hora**. Não é preciso reconectar a instância.

## Quais eventos chegam

O campo `events` da instância controla a assinatura. `ALL` entrega tudo; caso
contrário, só os tipos listados (`MESSAGE`, `SEND_MESSAGE`, `RECEIPT`, `GROUP`,
`NEWSLETTER`, `CONNECTION`, ...).

Duas exceções úteis: mesmo **sem** `MESSAGE`, mensagens de grupo chegam se você
assinar `GROUP`, e as de canal se assinar `NEWSLETTER`.

## Entrega

A entrega é **assíncrona**: a resposta da API não espera o webhook. O resultado
de cada tentativa vai para o log da instância (`GET /instance/logs/{id}`).

| Regra | Valor |
|---|---|
| Timeout por tentativa | 30s |
| Tentativas | 5 (1 inicial + 4 reenvios) |
| Espera entre tentativas | 2s, dobrando até no máximo 60s |
| Entregas simultâneas | 64 no total |

### O que é reenviado e o que não é

| Resposta do seu servidor | Reenvia? | Por quê |
|---|---|---|
| `2xx` | — | Entregue |
| `5xx` | ✅ | Falha temporária do servidor |
| `408`, `429` | ✅ | Timeout e rate limit se resolvem sozinhos |
| Outros `4xx` | ❌ | O servidor entendeu e recusou; reenviar só martela |
| Erro de rede / DNS / TLS | ✅ | Pode ser transitório |

Responda **2xx rápido** e processe depois. Se o seu handler demora mais de 30s,
a entrega é contada como falha e reenviada — você recebe o evento duplicado.

## Formato

`POST` com `Content-Type: application/json`, header
`User-Agent: EvolutionGO-Webhook/1.0`. O corpo tem sempre:

```json
{
  "event": "Message",
  "instanceId": "...",
  "instanceName": "...",
  "instanceToken": "...",
  "data": { }
}
```

## Diagnóstico

Quando um evento não chega, olhe nesta ordem:

1. **A URL está na lista?** `GET /instance/webhooks/{id}`.
2. **O evento está assinado?** Confira o campo `events` da instância.
3. **O log diz o quê?** `GET /instance/logs/{id}` registra cada tentativa:
   - `webhook delivered` — entregue, com status e resposta
   - `webhook rejected, not retrying` — seu servidor devolveu 4xx
   - `webhook failed, retrying in ...` — falha temporária, vai reenviar
   - `webhook failed after N attempts` — desistiu
   - `webhook dropped, N deliveries already in flight` — seus endpoints estão
     lentos demais e a fila encheu; o gargalo é o tempo de resposta deles
4. **O corpo chega vazio?** Verifique se o seu servidor lê o body antes de
   responder.

> Um erro no RabbitMQ, NATS ou WebSocket **não** impede os webhooks — os
> transportes são independentes.
