# FAQ - Perguntas Frequentes

Respostas para questões comuns sobre o Evolution GO.

## Visão Geral

### O que é Evolution GO?

Gateway de API WhatsApp construído em Go que fornece interface RESTful para comunicação com o protocolo WhatsApp Web. Permite automação de mensagens, gerenciamento multi-instância e integração via APIs padronizadas.

### Diferença entre Evolution GO e Evolution API (Node.js)?

- **Evolution GO**: Implementação em Go, foco em performance e baixo consumo de recursos
- **Evolution API V2**: Implementação em Node.js/TypeScript, maior maturidade e conjunto de features

Ambos utilizam a biblioteca whatsmeow para conexão com WhatsApp.

### Suporte a múltiplas instâncias?

Sim. É possível gerenciar quantas instâncias forem necessárias, cada uma representando uma conta WhatsApp independente com sessão isolada.

---

## Instalação

### SQLite vs PostgreSQL

**PostgreSQL** é fortemente recomendado para produção devido a:
- Performance superior em ambientes multiusuário
- Suporte a conexões concorrentes
- Recursos empresariais (replicação, backup incremental)

SQLite é adequado apenas para desenvolvimento/testes.

### Geração de API Key

```bash
# UUID v4 (recomendado)
uuidgen

# OpenSSL
openssl rand -hex 32

# Python
python3 -c "import uuid; print(uuid.uuid4())"
```

---

## Operações

### Processo de Conexão

1. Criar instância: `POST /instance/create`
2. Iniciar conexão: `POST /instance/connect`
3. Obter QR Code: `GET /instance/qr?instanceName=NOME`
4. Escanear QR Code no WhatsApp (Aparelhos Conectados)
5. Aguardar estabelecimento da conexão

### Validade do QR Code

Aproximadamente 40 segundos. Após expiração, novo QR Code é gerado automaticamente até atingir `QRCODE_MAX_COUNT` (padrão: 5 tentativas).

### Envio para Múltiplos Destinatários

Realizar requisições individuais para cada destinatário. Para volume alto:
- Implementar fila de envio
- Adicionar delay entre requisições (2-5 segundos recomendado)
- Utilizar worker pool para paralelização controlada

### Limites de Envio

O WhatsApp não divulga limites oficiais, mas observa-se:
- Aproximadamente 1000 mensagens/dia para números não salvos
- Risco de bloqueio com padrões identificados como spam
- Recomendações:
  - Começar com volume baixo (~100/dia)
  - Escalar gradualmente
  - Personalizar mensagens
  - Evitar conteúdo idêntico em massa
  - Respeitar opt-out de usuários

### Recepção de Mensagens

Configure webhook ao criar instância ou via atualização:

```json
{
  "instanceName": "nome-instancia",
  "webhook": "https://seu-servidor.com/webhook"
}
```

Eventos serão enviados via POST HTTP para o endpoint configurado.

---

## Troubleshooting

### QR Code Não Exibido

**Diagnóstico**:
```bash
docker-compose logs -f evolution-go
```

**Soluções**:
- Verificar conectividade com banco de dados
- Deletar e recriar instância
- Verificar logs para erros específicos

### Falha no Envio de Mensagens

**Verificações**:
1. Status da conexão: `GET /instance/status?instanceName=NOME`
2. Formato do número: DDI + DDD + número (ex: `5511999999999`)
3. Validação do destinatário: `CHECK_USER_EXISTS=true`

### Desconexão Espontânea

**Causas comuns**:
- WhatsApp Web ativo em outro dispositivo
- Bloqueio preventivo do WhatsApp
- Instabilidade de rede
- Versão do protocolo desatualizada

**Resolução**: Reconectar via `POST /instance/reconnect`

### Webhook Não Recebe Eventos

**Verificações**:
- URL acessível publicamente (não localhost)
- Endpoint retorna status 200
- Firewall/proxy não bloqueando
- Logs: `docker-compose logs -f evolution-go | grep webhook`

---

## Performance

### Capacidade por Servidor

Estimativas baseadas em uso típico:

| RAM | CPU | Instâncias Estimadas |
|-----|-----|---------------------|
| 1GB | 1 core | 5-10 |
| 2GB | 2 cores | 20-30 |
| 4GB | 4 cores | 50-100 |
| 8GB | 8 cores | 200+ |

**Fatores de impacto**:
- Volume de mensagens
- Uso de webhook vs filas
- `DATABASE_SAVE_MESSAGES` habilitado
- Processamento de mídia

### Otimização

**Configuração**:
```env
# Desabilitar salvamento de mensagens
DATABASE_SAVE_MESSAGES=false

# Ignorar eventos desnecessários
EVENT_IGNORE_STATUS=true
EVENT_IGNORE_GROUP=false  # conforme necessidade

# Usar PostgreSQL
POSTGRES_AUTH_DB=postgresql://...
POSTGRES_USERS_DB=postgresql://...
```

**Infraestrutura**:
- Connection pooling adequado
- Índices de banco otimizados
- Escalabilidade horizontal (múltiplos servidores)
- Load balancer para distribuição

---

## Segurança

### Proteção de API Key

- Não versionar no controle de código
- Utilizar variáveis de ambiente
- Rotação periódica (recomendado: 90 dias)
- HTTPS obrigatório em produção
- Rate limiting por chave

### Armazenamento de Mensagens

Configurar `DATABASE_SAVE_MESSAGES=false` se histórico não for necessário:
- Reduz uso de storage
- Melhora privacidade
- Aumenta performance

### Backup Seguro

```bash
# Backup com criptografia
docker exec postgres pg_dumpall -U postgres | \
  gzip | \
  gpg --encrypt --recipient seu@email.com \
  > backup-$(date +%Y%m%d).sql.gz.gpg

# Restauração
gpg --decrypt backup-20250111.sql.gz.gpg | \
  gunzip | \
  docker exec -i postgres psql -U postgres
```

---

## Limitações do WhatsApp

### Envio para Números Não Salvos

WhatsApp limita envios para números não salvos na agenda. Boas práticas:
- Priorizar números salvos
- Aguardar resposta antes de enviar mais mensagens
- Evitar volume alto para números desconhecidos

### Prevenção de Banimento

**Práticas recomendadas**:
- Não enviar spam
- Respeitar opt-out de usuários
- Usar contas WhatsApp Business verificadas
- Adicionar delay entre mensagens
- Personalizar conteúdo

**Em caso de bloqueio**:
- Utilizar número diferente
- Considerar WhatsApp Business API oficial
- Revisar práticas de envio

---

## Desenvolvimento

### Contribuição

Consulte [Guia de Contribuição](../desenvolvimento/contributing.md) para processo de contribuição ao projeto.

### Reporte de Bugs

Utilize [Issues no GitLab](https://git.evoai.app/Evolution/evolution-go/issues) incluindo:
- Versão do Evolution GO
- Ambiente (Docker/local, SO)
- Steps to reproduce
- Logs relevantes
- Comportamento esperado vs observado

---

## Recursos

### Documentação
- [Guia de Instalação](../fundamentos/installation.md)
- [Configuração](../fundamentos/configuration.md)
- [Referência de API](./api-reference.md)
- [Swagger UI](http://localhost:4000/swagger/index.html)

### Suporte
- [Issues GitLab](https://git.evoai.app/Evolution/evolution-go/issues)
- [Documentação Completa](https://git.evoai.app/Evolution/evolution-go/-/wikis)

---

**Não encontrou resposta?** Abra uma [issue](https://git.evoai.app/Evolution/evolution-go/issues) com sua questão.

**Documentação Evolution GO v1.0**
