# Retrospectiva — review-residual-implementations

## 0. Evidências

Pedido de revisão global com subagentes por direção; base f9d1300. review.md registra 24 causas, cobertura, provas e candidatos sem confirmação. tasks.md contém 25/25 concluídas. Relatórios implementation/{api,core,manager,durability,operations}.md registram RED/GREEN e limites; verify.md contém sete checks e comandos. Suite final: 3.044 casos/subcasos Go PASS, 1.231 testes de topo PASS, zero FAIL, um S3 skipped; Postgres isolado e NATS descartável habilitados. Manager 48 Vitest + 7 Node PASS. Vet/lint/build/Swagger passaram. Revisões independentes aprovaram todas as áreas; main recebeu patch idêntico de 86 arquivos e passou smoke/build/tests. Manifest de hashes do estado anterior preservado.

## 1. Resultado

Corrigidas autorização obsoleta de JWT, idempotência/integridade de corpos e capacidade do login; redirects/SSRF/token/JID; PATCH parcial concorrente, receipts cruzados, perda de chunks de import, coleta sem URI, contatos contains alheios e init sem sessão; cinco fluxos do manager e embed ausente. Persistência terminal agora confirma estado+evento atomicamente, conserva event_id/resultado em retries e evita novo Send na execução. Entrega local revisável, sem mudança de contrato público, dependência, migração, commit ou deploy.

## 2. O que funcionou

Direções explícitas API/núcleo/manager e segunda onda do núcleo fecharam lacunas. Probes read-only viraram regressões observáveis antes de cada correção; aliases, campos omitidos, DNS misto, IDs iguais entre instâncias e failure injection SQL demonstraram efeitos concretos. Ownership por arquivo e lease do wiring evitaram dupla escrita. Worktree e manifesto de hashes permitiram integrar somente o patch autorizado. Revisão posterior encontrou complemento A4 que as primeiras regressões não cobriam, corrigido antes da entrega.

## 3. Dificuldades e ajustes

Fresh clone não compilava o embed e Nuxt removia o placeholder; arquivo vazio rastreável e passo Node/fs após build resolveram ambos. Go não estava no PATH inicial; comandos pinados à ferramenta instalada. pnpm tentou auto-install em symlink e recusou; flag local evitou instalação/configuração permanente. Novos testes UI compilavam SFC a cada mount e um caso excedeu timeout: cache exclusivo de compilação reduziu custo sem partilhar estado ou relaxar limites.

Revalidação de JWT expôs 22 fixtures que mintavam tokens sem conta; corrigidas contas reais na fixture, sem bypass do lookup. Interfaces em edição entre writers provocaram falhas transitórias de compilação em focused/race; fontes foram congeladas/sincronizadas antes dos gates finais. Reconnect teve fixture que deixava retry concluir antes da asserção: RED sob race count100, barreira por canais e GREEN100, sem mudar produção/timeouts.

## 4. Decisões e limites

Conta persistida continua fonte da role; falha de lookup fecha acesso, API keys independentes. Replay valida envelope mínimo e conserva bytes; fingerprint distingue recurso/corpo efetivo e normaliza só instância. Clientes com credenciais não seguem redirects; host privado configurado continua suportado. Import mantém feed em memória e reconhece lote processado, sem nova tabela.

Estado terminal e evento compartilham transação existente; notifier dispensa segunda escrita e recovery protege lote ativo. Retry retém lock/resultado e aplica backpressure. Crash antes do commit, webhooks best-effort e estados terminais legados sem evento continuam limites explícitos. Sem WhatsApp/Chatwoot/S3 real; não se alega integração externa completa nem exactly-once. Quatro observações sem prova suficiente não viraram correções especulativas.

## 5. Aprendizados

Baseline verde deixou passar cenários de mudança de identidade/autorização, canonicalização incompleta e efeitos depois de falha de persistência. Regressões úteis verificam chamadas/efeitos/rollback, não só código retornado. Barreira determinística revela concorrência sem depender de scheduler. Testes com SFC e composables reais cobrem os payloads/ações com as dependências existentes; revisão deve conservar explícito o limite sem navegador. Contratos internos compartilhados precisam compilar em conjunto antes de iniciar race/global.

## 6. Próximos passos

Nenhuma correção obrigatória do escopo permanece aberta. Change ativa, delta specs e worktree ficam disponíveis para revisão e decisão de commit/arquivamento posteriores. As quatro observações em review.md precisam de reprodução e requisito antes de eventual nova change. Artefatos e logs permitem reexecutar a validação sem afetar schema/stream compartilhados. Esta retrospectiva é mantida como registro; fatos posteriores receberão adendos.
