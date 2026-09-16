# Chamadas (VoIP) — Evolution GO

Stack de chamadas de voz do WhatsApp, portado de
[WaCalls](https://github.com/JotaDev66/WaCalls) para dentro do nosso fork.

Tudo em Go puro (sem cgo): codec MLow, RTP/SRTP, STUN, relay SCTP/WebRTC e a
sinalização `<call>`, integrados ao whatsmeow vendorizado.

## Endpoints

Todos exigem o **token da instância** no header `apikey`.

| Método | Rota | O que faz |
|---|---|---|
| POST | `/call/offer` | Liga para um número — o telefone dele toca |
| POST | `/call/accept` | Atende uma chamada recebida |
| POST | `/call/terminate` | Desliga uma chamada (tocando ou ativa) |
| POST | `/call/hangup` | Recusa uma chamada recebida (por `callId`) |
| POST | `/call/reject` | Recusa usando `callCreator`/`callId` crus do webhook |
| GET | `/call/list` | Chamadas ativas agora |
| GET | `/call/history` | Últimas 50 chamadas encerradas (em memória) |
| GET | `/call/status/{callId}` | Uma chamada, ativa ou encerrada |

### Exemplo

```bash
# Ligar
curl -X POST https://SEU-HOST/call/offer \
  -H "apikey: TOKEN_DA_INSTANCIA" \
  -H "Content-Type: application/json" \
  -d '{"number":"5511999999999"}'
# → {"data":{"callId":"A1B2C3...","state":"ringing",...}}

# Desligar
curl -X POST https://SEU-HOST/call/terminate \
  -H "apikey: TOKEN_DA_INSTANCIA" \
  -d '{"callId":"A1B2C3..."}'
```

Tudo também está no **Swagger** (`/swagger/index.html`) e no **API Tester** do
manager, que lê o Swagger dinamicamente.

No manager, o botão de testes (🧪) de cada instância tem o cenário
**"Ligacao de voz"**: liga, deixa tocar ~6s e desliga sozinho.

## Áudio: o que funciona e o que não

| Recurso | Status |
|---|---|
| Tocar no telefone do destinatário | ✅ |
| Atender / recusar / desligar | ✅ |
| Estado e duração da chamada | ✅ |
| Chamada chegar a `active` com o par | ✅ |
| **Áudio bidirecional** | ✅ via WebSocket (abaixo) |

Chamadas de **vídeo** e de **grupo** não são suportadas — nem aqui nem no
WaCalls original, que também é só voz. A stack de mídia é a MLow, um codec de
voz; não há caminho de vídeo.

## Áudio bidirecional

```
GET /call/audio/{callId}     (WebSocket, header apikey)
```

Carrega o áudio da chamada nos **dois sentidos**, em quadros binários de
**PCM 16 kHz mono, signed 16-bit little-endian** — o mesmo formato que o
WaCalls trafega pelo data channel do WebRTC.

| Sentido | Significado |
|---|---|
| Cliente → servidor | Injetado na chamada como se viesse do microfone |
| Servidor → cliente | Áudio do interlocutor |

```js
const ws = new WebSocket(`wss://SEU_HOST/call/audio/${callId}?apikey=TOKEN`);
ws.binaryType = 'arraybuffer';
ws.onmessage = e => tocar(new Int16Array(e.data));   // voz do outro lado
ws.send(pcmInt16.buffer);                            // seu microfone
```

Detalhes que importam na prática:

- **Um cliente por chamada.** O `CallManager` tem um único sink de áudio; uma
  segunda conexão receberia `409` em vez de roubar o stream da primeira em
  silêncio.
- **Cliente lento perde áudio, a chamada não trava.** A fila de saída é
  limitada e descarta quadros em vez de bloquear a goroutine de mídia.
- Ping a cada 25s; sem pong em 60s a conexão cai.

> Com **proxy HTTP** a mídia não trafega (veja abaixo) — a chamada toca e
> conecta, mas o áudio não passa. Para áudio via proxy, use **SOCKS5**.

## Proxy

Este é o ponto sutil. Uma chamada tem dois caminhos:

| Caminho | Transporte | Passa por proxy HTTP? |
|---|---|---|
| **Sinalização** (`<call>`, tocar, atender, desligar) | Websocket do WhatsApp | ✅ **Sim** — já usa o proxy da instância |
| **Mídia** (áudio SRTP) | UDP para os relays | ❌ Não |

Ou seja: **com proxy HTTP a ligação toca, é atendida e desliga normalmente.** Só
o áudio não trafega, porque HTTP CONNECT é um túnel TCP e não carrega UDP.

Para o áudio também sair pelo proxy, use um **SOCKS5**: o `UDP ASSOCIATE`
(RFC 1928) está implementado em `pkg/voip/transport/socks5udp.go` e é plugado no
pion via `SettingEngine.SetNet`. Basta configurar o proxy da instância com
protocolo `socks5` — o roteamento é automático.

Se a associação SOCKS5 falhar, o sistema registra um aviso e cai para UDP
direto, em vez de derrubar a chamada.

## Layout do código

| Caminho | Responsabilidade |
|---|---|
| `pkg/voip/core` | Tipos de domínio, constantes, interface `VoipSocket` |
| `pkg/voip/wanode` | Helpers de nó/JID |
| `pkg/voip/media` | Codec MLow (Go puro), RTP, SRTP, SSRC, PCM |
| `pkg/voip/transport` | Relay SCTP, STUN, **proxy SOCKS5/UDP** |
| `pkg/voip/signaling` | Build/parse de `<call>`, cripto da call key |
| `pkg/voip/call` | `CallManager` — conduz uma chamada de ponta a ponta |
| `pkg/voip/wa` | `Socket` — adaptador sobre o whatsmeow |
| `pkg/voip/registry` | Uma `CallManager` por chamada, por instância + histórico |
| `pkg/call` | Serviço e handlers HTTP |

Um `CallManager` conduz **uma** chamada; o registry mantém até **5 simultâneas
por instância** (ofertas além disso são recusadas automaticamente).

Os testes do stack vieram junto: `go test ./pkg/voip/...`.
