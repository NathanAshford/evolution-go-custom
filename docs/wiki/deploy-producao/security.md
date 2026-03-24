# Segurança e Hardening

Guia de práticas de segurança para ambientes de produção do Evolution GO.

## Índice

- [Princípios de Segurança](#princípios-de-segurança)
- [Autenticação e API Keys](#autenticação-e-api-keys)
- [Gestão de Secrets](#gestão-de-secrets)
- [Segurança de Rede](#segurança-de-rede)
- [Hardening de Containers](#hardening-de-containers)
- [SSL/TLS](#ssltls)
- [Segurança do Banco de Dados](#segurança-do-banco-de-dados)
- [Backup Seguro](#backup-seguro)
- [Proteção contra Ataques](#proteção-contra-ataques)
- [Auditoria e Compliance](#auditoria-e-compliance)
- [Resposta a Incidentes](#resposta-a-incidentes)

---

## Princípios de Segurança

### Defense in Depth

Implementar múltiplas camadas de proteção:
- Autenticação forte
- Firewall e isolamento de rede
- Criptografia em trânsito e repouso
- Monitoramento e auditoria
- Backup e disaster recovery

### Least Privilege

Conceder apenas permissões mínimas necessárias:
- Containers não executam como root
- Credenciais com escopo limitado
- Acesso à rede restrito
- Segregação de ambientes

### Security by Default

- Configurações seguras desde a instalação
- HTTPS obrigatório em produção
- Logs de auditoria habilitados
- Credenciais fortes exigidas

### Requisitos por Ambiente

| Ambiente | Segurança Mínima |
|----------|------------------|
| **Desenvolvimento** | API Key forte, senhas não-padrão |
| **Staging** | + Firewall, backup, logs |
| **Produção** | + HTTPS, monitoramento, auditoria, secrets management |

---

## Autenticação e API Keys

### Geração de API Keys

Utilize chaves criptograficamente seguras:

```bash
# UUID v4 (recomendado)
uuidgen

# 64 caracteres hexadecimais
openssl rand -hex 32

# URL-safe base64
python3 -c "import secrets; print(secrets.token_urlsafe(32))"
```

**Exemplo de chave forte:**
```
df16caad-d0d2-41b2-bec5-75b90048a0db
```

**Evitar:**
- Valores padrão ou previsíveis
- Senhas comuns ou sequências
- Reutilização de chaves entre ambientes

### Rotação de Chaves

**Política recomendada:**
- Rotação periódica (90 dias)
- Rotação imediata em caso de comprometimento
- Manter período de transição para atualização de clientes

**Processo de rotação:**

1. Gerar nova chave
2. Atualizar configuração do servidor
3. Notificar clientes integradores
4. Estabelecer deadline para migração
5. Desabilitar chave antiga

### Rate Limiting

Implementar limitação de requisições por API Key via NGINX:

```nginx
http {
    map $http_apikey $limit_key {
        default $http_apikey;
        "" $binary_remote_addr;
    }

    limit_req_zone $limit_key zone=api_limit:10m rate=100r/s;

    server {
        location / {
            limit_req zone=api_limit burst=200 nodelay;
            limit_req_status 429;
            proxy_pass http://evolution-go:4000;
        }
    }
}
```

---

## Gestão de Secrets

### Docker Secrets (Swarm)

```bash
# Criar secrets
echo "senha_postgres" | docker secret create postgres_password -
echo "$(uuidgen)" | docker secret create evolution_api_key -

# docker-compose.swarm.yml
services:
  evolution-go:
    secrets:
      - evolution_api_key
      - postgres_password
    environment:
      GLOBAL_API_KEY_FILE: /run/secrets/evolution_api_key
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password

secrets:
  evolution_api_key:
    external: true
  postgres_password:
    external: true
```

### Kubernetes Secrets

```bash
# Criar secrets
kubectl create secret generic evolution-secrets \
  --from-literal=GLOBAL_API_KEY=$(uuidgen) \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -base64 32) \
  --namespace=evolution-go

# Habilitar encryption at rest
# https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/
```

```yaml
# deployment.yaml
        env:
        - name: GLOBAL_API_KEY
          valueFrom:
            secretKeyRef:
              name: evolution-secrets
              key: GLOBAL_API_KEY
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: evolution-secrets
              key: POSTGRES_PASSWORD
```

### HashiCorp Vault

```yaml
# vault-agent-config.hcl
vault {
  address = "https://vault.seudominio.com:8200"
}

auto_auth {
  method {
    type = "kubernetes"
    config = {
      role = "evolution-go"
    }
  }
}

template {
  source = "/vault/configs/.env.tpl"
  destination = "/app/.env"
}
```

---

## Segurança de Rede

### Firewall (iptables)

```bash
#!/bin/bash
# firewall-setup.sh

# Políticas padrão
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT ACCEPT

# Loopback
iptables -A INPUT -i lo -j ACCEPT

# Conexões estabelecidas
iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# SSH com rate limiting
iptables -A INPUT -p tcp --dport 22 -m conntrack --ctstate NEW -m recent --set
iptables -A INPUT -p tcp --dport 22 -m conntrack --ctstate NEW -m recent --update --seconds 60 --hitcount 4 -j DROP
iptables -A INPUT -p tcp --dport 22 -j ACCEPT

# HTTP/HTTPS restrito à rede interna
iptables -A INPUT -p tcp --dport 80 -s 10.0.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -s 10.0.1.0/24 -j ACCEPT

# Docker network
iptables -A INPUT -s 172.16.0.0/12 -j ACCEPT

# Drop inválidos
iptables -A INPUT -m conntrack --ctstate INVALID -j DROP

# Salvar regras
iptables-save > /etc/iptables/rules.v4
```

### UFW (Alternativa Simplificada)

```bash
sudo apt-get install ufw

# Configurar padrões
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Permitir serviços
sudo ufw allow 22/tcp   # SSH
sudo ufw allow 80/tcp   # HTTP
sudo ufw allow 443/tcp  # HTTPS

# Ativar
sudo ufw enable

# Status
sudo ufw status
```

### Isolamento de Redes Docker

```yaml
networks:
  frontend:
    driver: overlay
    driver_opts:
      encrypted: "true"
  
  backend:
    driver: overlay
    driver_opts:
      encrypted: "true"
    internal: true  # Sem acesso externo

services:
  evolution-go:
    networks:
      - frontend
      - backend

  postgres:
    networks:
      - backend  # Apenas rede interna
```

---

## Hardening de Containers

### Não Executar como Root

```dockerfile
FROM alpine:3.19.1

RUN addgroup -g 1000 evolution && \
    adduser -D -u 1000 -G evolution evolution

WORKDIR /app
COPY --chown=evolution:evolution server .

USER evolution

ENTRYPOINT ["/app/server"]
```

### Filesystem Read-Only

```yaml
services:
  evolution-go:
    read_only: true
    tmpfs:
      - /tmp
    volumes:
      - evolution_data:/app/dbdata
      - evolution_logs:/app/logs
```

### Security Options

```yaml
services:
  evolution-go:
    security_opt:
      - no-new-privileges:true
      - apparmor:docker-default
      - seccomp:default.json
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE  # Apenas se necessário
```

### Limites de Recursos

```yaml
services:
  evolution-go:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
          pids: 100
        reservations:
          cpus: '1.0'
          memory: 1G
    ulimits:
      nofile:
        soft: 1024
        hard: 2048
      nproc: 64
```

### Scan de Vulnerabilidades

```bash
# Trivy
trivy image evoapicloud/evolution-go:latest

# Apenas críticas
trivy image --severity CRITICAL evoapicloud/evolution-go:latest

# Docker Scout
docker scout cves evoapicloud/evolution-go:latest
```

---

## SSL/TLS

### Obtenção de Certificados (Let's Encrypt)

```bash
# Instalar Certbot
sudo apt-get install certbot

# Gerar certificado
sudo certbot certonly --standalone -d evolution.seudominio.com

# Certificados em:
# /etc/letsencrypt/live/evolution.seudominio.com/
```

### Configuração NGINX

```nginx
# Redirecionar HTTP → HTTPS
server {
    listen 80;
    server_name evolution.seudominio.com;
    return 301 https://$server_name$request_uri;
}

# HTTPS
server {
    listen 443 ssl http2;
    server_name evolution.seudominio.com;

    # Certificados
    ssl_certificate /etc/letsencrypt/live/evolution.seudominio.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/evolution.seudominio.com/privkey.pem;

    # Protocolos e ciphers
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers off;

    # HSTS
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

    # OCSP Stapling
    ssl_stapling on;
    ssl_stapling_verify on;
    ssl_trusted_certificate /etc/letsencrypt/live/evolution.seudominio.com/chain.pem;

    # Security Headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;

    # Ocultar server tokens
    server_tokens off;

    location / {
        proxy_pass http://evolution-go:4000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_hide_header X-Powered-By;
    }
}
```

### Renovação Automática

```bash
# Testar renovação
sudo certbot renew --dry-run

# Agendar via cron
sudo crontab -e

# Adicionar (renovação diária às 3h)
0 3 * * * certbot renew --quiet && systemctl reload nginx
```

---

## Segurança do Banco de Dados

### PostgreSQL Hardening

```ini
# postgresql.conf

# SSL obrigatório
ssl = on
ssl_cert_file = '/etc/ssl/certs/server.crt'
ssl_key_file = '/etc/ssl/private/server.key'

# Auditoria
log_connections = on
log_disconnections = on
log_duration = on
log_statement = 'mod'  # INSERT, UPDATE, DELETE

# Senha forte obrigatória
password_encryption = scram-sha-256
```

```ini
# pg_hba.conf

# Apenas SSL
hostssl evogo_auth evolution 10.0.0.0/8 scram-sha-256
hostssl evogo_users evolution 10.0.0.0/8 scram-sha-256

# Rejeitar sem SSL
hostnossl all all 0.0.0.0/0 reject
```

### Backup Criptografado

```bash
#!/bin/bash
# backup-encrypted.sh

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="postgres_${TIMESTAMP}.sql.gz.gpg"

# Dump e criptografar
docker exec postgres pg_dumpall -U postgres | \
  gzip | \
  gpg --symmetric --cipher-algo AES256 --output "/backups/${BACKUP_FILE}"

# Upload S3 com SSE
aws s3 cp "/backups/${BACKUP_FILE}" \
  "s3://bucket-backup/postgres/${BACKUP_FILE}" \
  --server-side-encryption AES256
```

---

## Backup Seguro

### Estratégia 3-2-1

- 3 cópias dos dados
- 2 mídias diferentes
- 1 cópia offsite

```bash
#!/bin/bash
# backup-secure.sh

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups"
S3_BUCKET="s3://backup-bucket"
GPG_KEY="backup@seudominio.com"

# PostgreSQL
docker exec postgres pg_dumpall -U postgres | \
  gzip | \
  gpg --encrypt --recipient $GPG_KEY --output "${BACKUP_DIR}/postgres_${TIMESTAMP}.sql.gz.gpg"

# Volumes
docker run --rm \
  -v evolution_data:/data \
  -v ${BACKUP_DIR}:/backup \
  alpine tar czf - -C /data . | \
  gpg --encrypt --recipient $GPG_KEY --output "${BACKUP_DIR}/evolution_data_${TIMESTAMP}.tar.gz.gpg"

# Upload S3 com KMS
aws s3 cp "${BACKUP_DIR}/postgres_${TIMESTAMP}.sql.gz.gpg" \
  "${S3_BUCKET}/postgres/" \
  --server-side-encryption aws:kms \
  --ssekms-key-id alias/backup-key

# Limpeza (30 dias)
find ${BACKUP_DIR} -name "*.gpg" -mtime +30 -delete
```

---

## Proteção contra Ataques

### DDoS Protection

**NGINX Rate Limiting:**

```nginx
http {
    limit_conn_zone $binary_remote_addr zone=conn_limit:10m;
    limit_conn conn_limit 10;

    limit_req_zone $binary_remote_addr zone=req_limit:10m rate=10r/s;

    server {
        location / {
            limit_req zone=req_limit burst=20 nodelay;
            proxy_pass http://evolution-go:4000;
        }
    }
}
```

**Cloudflare:** Habilitar proteção DDoS no painel.

### SQL Injection

Evolution GO usa GORM (ORM) que previne SQL injection por padrão através de prepared statements.

### Brute-Force Protection (Fail2ban)

```bash
sudo apt-get install fail2ban
```

```ini
# /etc/fail2ban/jail.local
[nginx-req-limit]
enabled = true
filter = nginx-req-limit
logpath = /var/log/nginx/error.log
maxretry = 5
findtime = 600
bantime = 3600
```

```ini
# /etc/fail2ban/filter.d/nginx-req-limit.conf
[Definition]
failregex = limiting requests, excess:.* by zone.*client: <HOST>
```

---

## Auditoria e Compliance

### LGPD/GDPR

**Direito ao esquecimento:**

```sql
CREATE PROCEDURE delete_user_data(user_phone VARCHAR)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM messages WHERE phone = user_phone;
    DELETE FROM contacts WHERE phone = user_phone;
    DELETE FROM instances WHERE owner_phone = user_phone;
    
    INSERT INTO audit_log (action, details, timestamp)
    VALUES ('user_data_deleted', user_phone, NOW());
    
    COMMIT;
END;
$$;
```

**Retenção de dados:**

```sql
-- Deletar mensagens antigas (30 dias)
DELETE FROM messages WHERE timestamp < NOW() - INTERVAL '30 days';
```

### Logging de Auditoria

```yaml
services:
  evolution-go:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "5"
        labels: "service,environment"
```

---

## Resposta a Incidentes

### Plano de Resposta

**1. Detecção**
- Alertas automatizados
- Monitoramento de logs
- Relatórios de usuários

**2. Contenção**
- Isolar sistemas afetados
- Revogar credenciais comprometidas
- Bloquear IPs maliciosos
- Capturar evidências

**3. Erradicação**
- Aplicar patches
- Atualizar imagens
- Revisar logs de auditoria
- Remover backdoors

**4. Recuperação**
- Restaurar de backup
- Validar integridade
- Monitorar por 48h

**5. Pós-Incidente**
- Documentar lições aprendidas
- Atualizar runbooks
- Treinar equipe

### Forensics

```bash
# Capturar estado do container
docker commit container_suspeito forensics-image
docker save forensics-image > forensics-image.tar

# Coletar logs
docker logs container_suspeito > logs_suspeito.txt

# Inspecionar
docker inspect container_suspeito > inspect_suspeito.json

# Exportar filesystem
docker export container_suspeito > filesystem_suspeito.tar
```

---

## Checklist de Segurança

### Crítico
- [ ] API Keys fortes (UUID)
- [ ] HTTPS habilitado
- [ ] Secrets em vault/secrets manager
- [ ] Firewall configurado
- [ ] Backups automáticos funcionando

### Importante
- [ ] Containers não executam como root
- [ ] Isolamento de redes Docker
- [ ] Limites de recursos definidos
- [ ] Logs de auditoria ativos
- [ ] Monitoramento configurado

### Recomendado
- [ ] Scan de vulnerabilidades regular
- [ ] Plano de resposta a incidentes
- [ ] Testes de restauração de backup
- [ ] Documentação de segurança
- [ ] Treinamento da equipe

---

## Recursos Adicionais

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

---

**Documentação Evolution GO v1.0**
