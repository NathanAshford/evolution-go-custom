# whatsmeow — como manter atualizado

## O que mudou

O projeto **não usa mais um fork vendorizado** do whatsmeow. A biblioteca é uma
dependência normal do Go:

```
go.mau.fi/whatsmeow v0.0.0-20260806224404-e277b766ab33
```

O diretório `whatsmeow-lib/` (5,4 MB) e o `replace` no `go.mod` foram removidos.

## Por que isso importa

O fork estava travado na versão de cliente `2.3000.1035920091`. O WhatsApp
avançou e passou a responder **405 "client outdated"** — a conexão morria antes
de emitir QR, então **nenhuma instância nova conseguia parear**.

Só subir o número da versão **não resolve**: o upstream publica os bumps de
versão **junto com mudanças no schema protobuf**. A prova: `2.3000.1044440921`
falhava no fork e funcionava num build do upstream puro, na mesma máquina.

O próprio whatsmeow avisa isso no `SetWAVersion`:

> *"you should keep the library up-to-date instead of using this, as there may be
> code changes that are necessary too (like protobuf schema changes)"*

## Como atualizar agora

```bash
go get -u go.mau.fi/whatsmeow
go mod tidy
go build ./... && go test ./...
```

Se o QR parar de funcionar de novo com 405, é isso: rode o comando acima.

## Por que deu para abandonar o fork

Tudo que tínhamos patcheado no fork **já está no upstream**:

| Customização | Situação |
|---|---|
| Pareamento por passkey (WebAuthn) | ✅ upstream (`pair-passkey.go`) |
| `cstoken` / derivação do NCT salt | ✅ upstream (`cstoken.go`) |
| tctoken com chaveamento por LID (erro 463) | ✅ upstream |
| Migração `14-nct-salt.sql` | ✅ upstream (idêntica à nossa) |
| Índice `IndexNCTSaltSync` no app-state | ✅ upstream |
| Consultas MEX de limites da conta | ❌ **só nosso** → `pkg/walimits` |

### `pkg/walimits`

As únicas features que não existem no upstream são as consultas de diagnóstico
que usamos para o erro 463:

- `GetAccountReachoutTimelock` — se a conta está bloqueada para iniciar conversas
- `GetNewChatMessageCappingInfo` — quota de conversas novas no ciclo

Elas viviam dentro da biblioteca porque usavam o `sendMexIQ`, que é privado. Como
o upstream expõe `DangerousInternals().SendMexIQ`, elas passaram para o nosso
código sem precisar de fork.

## Nota sobre bancos existentes

Bancos criados com o fork têm a migração 14 aplicada com uma diferença cosmética:
a nossa versão declarava `PRIMARY KEY (our_jid)` sem a foreign key para
`whatsmeow_device`. As colunas são as mesmas e o upstream não reaplica a 14, então
funciona normalmente — a única perda é o `ON DELETE CASCADE` nessa tabela em
bancos antigos.
