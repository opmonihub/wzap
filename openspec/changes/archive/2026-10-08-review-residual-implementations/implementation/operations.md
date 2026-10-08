# O1 — Placeholder do manager

Baseline: fresh worktree não tinha manager/.output/public; go:embed falhou antes dos testes. O comentário de manager.go já documentava um .gitkeep force-added, ausente do índice. Nuxt generate também remove o diretório e o placeholder anterior.

Correção: incluir somente manager/.output/public/.gitkeep, vazio, e executar scripts/preserve-embed-placeholder.mjs após nuxt generate. O helper usa writeFile de Node/fs existente, sem dependências. Nenhum asset Nuxt é rastreado.

Verificação do build completo: `pnpm --config.verify-deps-before-run=false --dir manager build`, exit 0; index.html e .gitkeep presentes ao terminar. Log /tmp/wzap-review-manager-placeholder-build.log. O flag CLI evita o auto-install do pnpm em node_modules symlinkado do worktree; nenhuma instalação.

Verificação sem assets: copiar manager.go e manager_test.go reais para /tmp/wzap-unbuilt-manager-probe, com apenas .gitkeep no embed e módulo Go 1.26. `go build ./...`, `go test ./manager -count=1 -v` e probe do Handler retornaram exit 0. Probe confirmou Built=false e GET /manager/ 503; TestEmbeddedDist foi skipped explicitamente nesse cenário sem bundle. Logs /tmp/wzap-review-unbuilt-manager-{test,check}.log. Os gates completos da base com console construída serão registrados em verify.md.

Sem novo teste permanente que apenas replique criação de arquivo; aproveitados testes existentes de serving e probe do pacote real. Sem commit/deploy/reinício.
