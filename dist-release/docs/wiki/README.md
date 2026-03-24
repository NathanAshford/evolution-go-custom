<div align="center">

# 📚 Documentação Evolution GO

**Gateway de API WhatsApp de alta performance desenvolvido em Go**

[![Documentação: 100%](https://img.shields.io/badge/Documenta%C3%A7%C3%A3o-100%25-success?style=flat-square)]()
[![Endpoints: 79](https://img.shields.io/badge/Endpoints-79-blue?style=flat-square)]()
[![Guias: 30](https://img.shields.io/badge/Guias-30-informational?style=flat-square)]()

</div>

---

Bem-vindo à documentação oficial do Evolution GO! Aqui você encontrará guias completos, referências de API e melhores práticas para integrar o WhatsApp às suas aplicações de forma profissional e escalável.

## 🚀 Começo Rápido

**Primeira vez aqui?** Siga este caminho:

1. 📖 [**Introdução**](./fundamentos/introduction.md) - Entenda o que é o Evolution GO
2. 🔧 [**Instalação**](./fundamentos/installation.md) - Configure em 5 minutos
3. ⚡ [**Quickstart**](./fundamentos/quickstart.md) - Envie sua primeira mensagem
4. 📡 [**Visão Geral da API**](./guias-api/api-overview.md) - Conheça os endpoints disponíveis

## 📖 O que você quer fazer?

### Enviar Mensagens
- [Enviar texto, imagens e documentos](./guias-api/api-messages.md)
- [Criar enquetes interativas](./guias-api/api-messages.md#enviar-enquete)
- [Enviar localizações e contatos](./guias-api/api-messages.md#enviar-localização)

### Gerenciar Instâncias
- [Criar e conectar instâncias](./guias-api/api-instances.md)
- [Autenticação via QR Code](./recursos-avancados/qrcode-connection.md)
- [Gerenciar múltiplas contas](./conceitos-core/instances.md)

### Automatizar Workflows
- [Receber eventos via Webhook](./recursos-avancados/events-system.md)
- [Integrar com RabbitMQ/NATS](./recursos-avancados/events-system.md)
- [Armazenar mídias automaticamente](./recursos-avancados/media-storage.md)

### Deploy em Produção
- [Deploy com Docker](./deploy-producao/docker-deployment.md)
- [Segurança e compliance](./deploy-producao/security.md)
- [Escalabilidade](./conceitos-core/architecture.md)

---

## 📚 Documentação Completa

### 🎯 Fundamentos

Documentação essencial para começar a usar o Evolution GO.

| Documento | Descrição |
|-----------|-----------|
| [**Introdução**](./fundamentos/introduction.md) | O que é o Evolution GO, recursos e casos de uso |
| [**Instalação**](./fundamentos/installation.md) | Guia completo de instalação (Docker, Local, Swarm) |
| [**Configuração**](./fundamentos/configuration.md) | Variáveis de ambiente e configurações |
| [**Quickstart**](./fundamentos/quickstart.md) | Tutorial prático: primeira instância em minutos |

### 🏗️ Conceitos Core

Entenda como o Evolution GO funciona por dentro.

| Documento | Descrição |
|-----------|-----------|
| [**Arquitetura**](./conceitos-core/architecture.md) | Arquitetura em camadas, componentes e fluxos |
| [**Instâncias**](./conceitos-core/instances.md) | Como funcionam as instâncias WhatsApp |
| [**Autenticação**](./conceitos-core/authentication.md) | API Keys, tokens e segurança |
| [**Banco de Dados**](./conceitos-core/database.md) | Estrutura dual: evogo_auth + evogo_users |

### 📡 API Reference

Referência completa de todos os endpoints disponíveis.

| API | Endpoints | Descrição |
|-----|-----------|-----------|
| [**Overview**](./guias-api/api-overview.md) | - | Visão geral da API REST |
| [**Instâncias**](./guias-api/api-instances.md) | 16 | Criar, conectar, gerenciar instâncias |
| [**Mensagens**](./guias-api/api-messages.md) | 16 | Enviar texto, mídia, enquetes, stickers |
| [**Usuários**](./guias-api/api-user.md) | 13 | Perfil, contatos, privacidade, bloqueio |
| [**Grupos**](./guias-api/api-groups.md) | 11 | Criar e administrar grupos |
| [**Chats**](./guias-api/api-chats.md) | 7 | Pin, archive, mute, histórico |
| [**Labels**](./guias-api/api-labels.md) | 6 | Etiquetar chats e mensagens |
| [**Chamadas**](./guias-api/api-call.md) | 1 | Rejeitar chamadas recebidas |
| [**Comunidades**](./guias-api/api-community.md) | 3 | Criar e gerenciar comunidades |
| [**Newsletters**](./guias-api/api-newsletter.md) | 6 | Canais do WhatsApp |
| [**Webhooks**](./guias-api/api-webhooks.md) | - | Configurar eventos em tempo real |

### 🚀 Recursos Avançados

Funcionalidades avançadas para casos de uso complexos.

| Documento | Descrição |
|-----------|-----------|
| [**Sistema de Eventos**](./recursos-avancados/events-system.md) | Webhooks, RabbitMQ, NATS, WebSocket |
| [**Armazenamento de Mídia**](./recursos-avancados/media-storage.md) | MinIO/S3, presigned URLs |
| [**Conexão QR Code**](./recursos-avancados/qrcode-connection.md) | Fluxo de autenticação detalhado |
| [**Multi-Device**](./recursos-avancados/multi-device.md) | Protocolo Multi-Device do WhatsApp |

### 🐳 Deploy e Produção

Guias para colocar o Evolution GO em produção.

| Documento | Descrição |
|-----------|-----------|
| [**Deploy com Docker**](./deploy-producao/docker-deployment.md) | Docker Compose, Swarm, Kubernetes |
| [**Segurança**](./deploy-producao/security.md) | SSL/TLS, API Keys, Firewall, Hardening |

### 💻 Desenvolvimento

Para desenvolvedores que querem contribuir ou personalizar.

| Documento | Descrição |
|-----------|-----------|
| [**Guia de Desenvolvimento**](./desenvolvimento/development-guide.md) | Setup do ambiente de desenvolvimento |
| [**Como Contribuir**](./desenvolvimento/contributing.md) | Padrões de código e processo de PR |
| [**Debugging**](./desenvolvimento/debugging.md) | Troubleshooting e profiling |

### 📖 Referência Técnica

Consulta rápida para desenvolvedores.

| Documento | Descrição |
|-----------|-----------|
| [**API Reference**](./referencia/api-reference.md) | Todos os 79 endpoints em uma página |
| [**Environment Variables**](./referencia/environment-variables.md) | Lista completa de variáveis de ambiente |
| [**Error Codes**](./referencia/error-codes.md) | Códigos HTTP e erros da aplicação |
| [**FAQ**](./referencia/faq.md) | Perguntas frequentes e soluções |

---

## 💡 Precisa de Ajuda?

### 📖 Recursos de Aprendizado
- **Novo no Evolution GO?** Comece pelo [Quickstart](./fundamentos/quickstart.md)
- **Dúvidas frequentes?** Confira o [FAQ](./referencia/faq.md)
- **Problemas técnicos?** Veja o guia de [Debugging](./desenvolvimento/debugging.md)

### 🔗 Links Importantes
- 🌐 [Repositório Oficial](https://git.evoai.app/Evolution/evolution-go)
- 📚 [Biblioteca Whatsmeow](https://github.com/tulir/whatsmeow)
- 🔧 [API Swagger](http://localhost:4000/swagger/index.html) (quando servidor estiver rodando)
- 🐛 [Reportar Issues](https://git.evoai.app/Evolution/evolution-go/issues)

### 💬 Suporte
- **Issues técnicos**: Abra uma issue no repositório
- **Dúvidas sobre API**: Consulte a [Referência da API](./referencia/api-reference.md)
- **Erros e códigos**: Veja [Códigos de Erro](./referencia/error-codes.md)

## 🎯 Recursos Principais

- ✅ **79 endpoints** REST documentados
- ✅ **Multi-instância** - Gerencie múltiplas contas WhatsApp
- ✅ **Multi-device** - Suporte WhatsApp Multi-Device nativo
- ✅ **Eventos em tempo real** - Webhooks, RabbitMQ, NATS, WebSocket
- ✅ **Armazenamento de mídia** - MinIO/S3 integrado
- ✅ **Alta performance** - Desenvolvido em Go
- ✅ **Docker ready** - Deploy fácil em produção

---

<div align="center">

**Mantido por EvoAI Services** • **Versão da Documentação: 1.0.0**

[⬆️ Voltar ao topo](#-documentação-evolution-go)

</div>
