# Como Contribuir

Guia para contribuir com o Evolution GO.

## Índice

- [Como Contribuir](#como-contribuir)
- [Código de Conduta](#código-de-conduta)
- [Reportando Bugs](#reportando-bugs)
- [Sugerindo Features](#sugerindo-features)
- [Pull Requests](#pull-requests)
- [Padrões de Código](#padrões-de-código)
- [Processo de Review](#processo-de-review)

---

## Como Contribuir

Contribuições são bem-vindas! Você pode contribuir:

- 🐛 Reportando bugs
- 💡 Sugerindo novas features
- 📝 Melhorando documentação
- 🔧 Corrigindo bugs
- ✨ Implementando features

---

## Código de Conduta

- Seja respeitoso e profissional
- Aceite feedback construtivo
- Foque no que é melhor para a comunidade
- Mostre empatia com outros membros

---

## Reportando Bugs

### Antes de Reportar

1. Verifique se o bug já foi reportado nas [Issues](https://git.evoai.app/Evolution/evolution-go/issues)
2. Teste na versão mais recente
3. Colete informações: logs, versão do Go, sistema operacional

### Template de Bug Report

```markdown
**Descrição do Bug**
Descrição clara e concisa do bug.

**Como Reproduzir**
1. Faça X
2. Execute Y
3. Veja erro Z

**Comportamento Esperado**
O que deveria acontecer.

**Comportamento Atual**
O que está acontecendo.

**Screenshots/Logs**
Se aplicável, adicione screenshots ou logs.

**Ambiente**
- OS: [ex: Ubuntu 22.04]
- Go Version: [ex: 1.24.0]
- Evolution GO Version: [ex: v1.0.0]
- PostgreSQL Version: [ex: 15.2]

**Informações Adicionais**
Qualquer outro contexto sobre o problema.
```

---

## Sugerindo Features

### Template de Feature Request

```markdown
**Descrição da Feature**
Descrição clara da feature proposta.

**Problema que Resolve**
Qual problema esta feature resolve?

**Solução Proposta**
Como você imagina que esta feature funcionaria?

**Alternativas Consideradas**
Outras abordagens que você considerou?

**Informações Adicionais**
Mockups, exemplos, referências, etc.
```

---

## Pull Requests

### Processo

1. **Fork** o repositório
2. **Clone** seu fork: `git clone https://git.evochat.com/SEU-USUARIO/evolution-go.git`
3. **Crie branch**: `git checkout -b feature/minha-feature`
4. **Desenvolva** e **commit** suas mudanças
5. **Push**: `git push origin feature/minha-feature`
6. **Abra PR** no repositório original

### Checklist do PR

- [ ] Código segue os padrões do projeto
- [ ] Código foi formatado (`make fmt`)
- [ ] Lint passou sem erros (`make lint`)
- [ ] Documentação foi atualizada (se necessário)
- [ ] Swagger foi atualizado (`make swagger`) se alterou endpoints
- [ ] Commit messages são descritivas

### Padrão de Commit

Use [Conventional Commits](https://www.conventionalcommits.org/):

```bash
# Feature
git commit -m "feat: adiciona suporte para envio de áudio"

# Bug fix
git commit -m "fix: corrige erro ao deletar instância"

# Documentação
git commit -m "docs: atualiza guia de instalação"

# Refatoração
git commit -m "refactor: simplifica lógica de conexão"

# Chore (manutenção)
git commit -m "chore: atualiza dependências"

# Performance
git commit -m "perf: otimiza query de listagem de mensagens"
```

### Mensagens de Commit Detalhadas

```bash
git commit -m "feat: adiciona endpoint de busca de mensagens

- Implementa GET /message/search
- Adiciona filtros por data, remetente e texto
- Inclui paginação (limit/offset)
- Atualiza documentação Swagger

Closes #42"
```

---

## Padrões de Código

### Go Formatting

```bash
# Sempre formate antes de commit
make fmt

# Ou manualmente
go fmt ./...
goimports -w .
```

### Linting

```bash
# Execute lint
make lint

# Corrige automaticamente quando possível
golangci-lint run --fix
```

### Estrutura de Código

Siga o padrão **Handler → Service → Repository**:

```go
// pkg/mymodule/handler.go
package mymodule

type MyHandler struct {
    service MyService
}

func (h *MyHandler) Create(c *gin.Context) {
    // Valida input
    // Chama service
    // Retorna response
}

// pkg/mymodule/service.go
type MyService interface {
    Create(dto CreateDTO) (*Model, error)
}

type myService struct {
    repo MyRepository
}

func (s *myService) Create(dto CreateDTO) (*Model, error) {
    // Lógica de negócio
    // Chama repository
    return model, nil
}

// pkg/mymodule/repository.go
type MyRepository interface {
    Save(model *Model) error
}

type myRepository struct {
    db *gorm.DB
}

func (r *myRepository) Save(model *Model) error {
    return r.db.Create(model).Error
}
```

### Documentação Swagger

Sempre documente endpoints públicos:

```go
// GetInstance retorna uma instância por ID.
//
// @Summary Buscar instância
// @Description Retorna detalhes de uma instância WhatsApp
// @Tags instance
// @Accept json
// @Produce json
// @Param instanceName path string true "Nome da instância"
// @Success 200 {object} Instance
// @Failure 404 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /instance/{instanceName} [get]
func (h *InstanceHandler) GetInstance(c *gin.Context) {
    // ...
}
```

---

## Processo de Review

### O que os Reviewers Avaliam

- ✅ **Funcionalidade**: O código faz o que deveria?
- ✅ **Qualidade**: Código limpo, legível e manutenível?
- ✅ **Performance**: Há problemas de performance?
- ✅ **Segurança**: Há vulnerabilidades?
- ✅ **Testes**: Há cobertura adequada? (quando testes existirem)
- ✅ **Documentação**: Está atualizada?

### Timeline Esperado

- **Primeira revisão**: 1-3 dias úteis
- **Revisões subsequentes**: 1-2 dias úteis
- **Merge**: Após aprovação de 1+ mantainer

### Respondendo a Feedbacks

- Seja receptivo a sugestões
- Faça perguntas se não entender
- Atualize o código conforme feedback
- Force push (`git push -f`) é OK durante review

### Após o Merge

- Delete sua branch: `git branch -d feature/minha-feature`
- Atualize seu fork: `git pull upstream main`

---

## Recursos Úteis

- **Issues**: https://git.evoai.app/Evolution/evolution-go/issues
- **Pull Requests**: https://git.evoai.app/Evolution/evolution-go/merge_requests
- **Documentação**: https://git.evoai.app/Evolution/evolution-go/-/wikis
- **Go Style Guide**: https://google.github.io/styleguide/go/
- **Effective Go**: https://go.dev/doc/effective_go

---

**Obrigado por contribuir!** 🎉

**Mantido por**: Equipe EvoAI Services
