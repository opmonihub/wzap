## 1. Cobertura e contratos Swagger

- [x] 1.1 Adicionar regressão de cobertura por método/path e schemas do documento servido; verificar falha antes das correções e sucesso com `go test ./internal/httpapi -run TestSwagger -count=1` depois.
- [x] 1.2 Documentar operações Chatwoot e entradas públicas do Manager, incluindo parâmetros, acesso e respostas; verificar inventário servido sem rotas explícitas ausentes.
- [x] 1.3 Corrigir envelopes e exceções nas anotações existentes e regenerar docs; verificar schemas de coleções, binário, 204, readiness 503 e webhook, e uma segunda geração sem diferenças.

## 2. Verificação e entrega

- [x] 2.1 Executar gofmt, go vet, golangci-lint, go test e go build, revisar o delta próprio e registrar evidências em verify.md e retrospective.md.
- [x] 2.2 Aplicar somente o delta próprio à árvore original e verificar novamente Swagger e ausência de conflitos com o snapshot inicial.
