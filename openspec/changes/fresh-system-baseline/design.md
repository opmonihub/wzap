## Context

Ver `proposal.md` e os 11 deltas em `specs/`. O plano foi aprovado para instalações novas e reset local sem backup; a instrução atual limita esta entrega a artefatos OpenSpec. A cadeia embutida contém duas versões `00009`, reproduzidas como panic do Goose em teste de integração; o banco local está na versão 9 de mensagens temporárias, sem `resolved_at`.

## Goals / Non-Goals

**Goals:** estabelecer uma base inicial verificável e eliminar caminhos cuja única finalidade é adaptar instalações anteriores, mantendo transporte, serviços, sessões, eventos, mídia e storage nas suas fronteiras atuais.

**Non-Goals:** implementar ou resetar nesta entrega, migrar dados antigos, mudar o contrato de eventos v1, reescrever funcionalidades válidas, instalar dependências ou editar workflows CI. Futuras migrações do produto novo continuam permitidas.

## Decisions

### 1. Baseline único, sem transformação histórica

Na aplicação futura, substituir os SQLs existentes por `internal/storage/migrations/00001_init.sql`, criando diretamente: `users`, `instances`, `instance_connections`, `instance_webhooks`, `instance_chat_settings`, `message_queue`, `media`, `jid_cache`, `chatwoot_configs`, `chatwoot_messages`, `group_metadata`, `channel_metadata`, `idempotency_keys`, `event_outbox` e `webhook_dead_letters`. Goose e whatsmeow mantêm suas tabelas próprias; `remodel_report` desaparece.

Partir do catálogo final vigente, mantendo UUIDs/defaults, timestamps, enums, unicidades naturais, índices e FKs necessários ao uso atual, sem copiar scripts de backfill. `instances.owner_user_id` será NOT NULL com FK restritiva, e `instances.name` terá unicidade global case-sensitive. A validação atual de nome e os erros 422/409 continuam nos serviços e handlers, com conflitos concorrentes do banco mapeados para `instance_name_taken`.

Alternativa rejeitada: renumerar para 9/10/11, pois manteria upgrades e compatibilidade que o usuário decidiu eliminar. Não implementar testes de upgrade nem recuperação de dados antigos.

### 2. Fluxos atuais com entradas válidas

Seed cria somente o primeiro admin; remover `ownerBackfiller`, `BackfillOwner` e compensações exclusivas da adoção. Repositórios e fixtures não criam instâncias órfãs. Criação por key global continua escolhendo o admin mais antigo quando não há dono explícito; a ausência de dono válido impede persistência e emissão de key.

Remover exceções de nomes históricos, `ErrInstanceNameAmbiguous` e cenários de dados duplicados. Preservar UUID prioritário, nome exato, reserva de `stats`/UUID, renomeação, autorização e identidade canônica para idempotência. A validação genérica de entrada e ausência de credencial continua necessária; remover testes de compatibilidade não significa remover testes de segurança.

Alternativa rejeitada: remover testes apenas pelo nome ou trocar textos sem retirar ramificações de runtime, pois isso esconderia suporte histórico ainda executável.

### 3. Respostas e eventos sem conversores

Replay usa o status HTTP e corpo originais produzidos pelo contrato atual, após autorização, sem `convertReplayBody`, conversão de envelopes ou reconstrução do recurso. Preservar TTL, fingerprint, concorrência, headers, liberação de chave para 4xx/503 e proteção contra repetição de efeito. Integridade de registros atuais continua verificável; corrupção resulta em erro interno, nunca em reexecução para reconstruir uma resposta.

Remover `legacy_error` dos DTOs e fixtures; revisar todos os produtores de falhas de conexão, envio, outbox e dead letters para gravarem código, mensagem e instante reais, sem inventar timestamp. `LastError` textual utilizado por consumidores atuais não é removido indiscriminadamente: é a mensagem do erro estruturado vigente. Remover o campo `reason` da resposta de revogação, hoje reservado apenas para compatibilidade futura e sem produtor; sucesso continua `data.revoked=true`.

Na outbox pending-only, remover `DeletePublishedBefore`, scheduling e limpeza sem efeito e os respectivos fakes/testes. Confirmação de publicação continua removendo o pendente; broker indisponível mantém retries, envelope v1 e `event_id` estável.

Alternativas rejeitadas: converter envelopes em replay ou limpar todas as chaves, pois perpetuariam compatibilidade ou permitiriam segundo efeito.

### 4. Segredos e configuração com formato único

Chatwoot habilitado exige `WZAP_CHATWOOT_TOKEN_KEY` em base64 com exatamente 32 bytes. Tokens não vazios persistem apenas em AES-256-GCM autenticado no formato atual; leituras públicas nunca incluem tokens. A escrita recebe plaintext em memória e o cifra; leitura autentica/decifra, sem passthrough de texto puro nem `BackfillTokenSeal`. Configuração desligada sem token não exige chave; gravação de token não vazio sem chave válida é recusada. Testes e wiring de repositórios que usam tokens passam a fornecer chave válida de teste.

Remover o modo plaintext e warnings de backfill. Não acrescentar negociação de versões ou conversão de ciphertext. Preservar a importação operacional Chatwoot e as correlações pendentes até a confirmação do WA ID: são funcionalidades atuais, não migração histórica.

Aceitar apenas `json` e `console` em `WZAP_LOG_FORMAT`; valores diferentes falham a configuração. Remover handler dedicado de `/api/v1`; paths não registrados seguem o fluxo genérico de routing/autorização. Remover referências a configurações anteriores sem introduzir aliases novos.

Alternativa rejeitada: manter plaintext opcional, que conservaria dois formatos de persistência e uma ramificação de compatibilidade.

### 5. Dois backends de mídia oficiais

Backend fixado no boot: endpoint S3 presente seleciona S3/MinIO; ausente seleciona disco. Compose mantém MinIO como padrão; se S3 configurado não consegue preparar o bucket, o boot falha. Falhas posteriores não trocam para disco. Não oferecer troca de backend com migração automática de registros.

Mídias novas guardam bucket explícito: bucket configurado no modo S3 e `local` no modo disco. Remover default de bucket vazio, `MigrateLocalFiles`, `SetBucket` usado apenas pelo migrador e o comando `media-migrate`. Leitura e exclusão S3 usam o bucket da row; esse suporte permanece válido para registros criados pelo produto atual. Cache de objetos, SHA-256, TTL, autenticação, retries de limpeza e compensação de criação falha continuam.

Alternativas rejeitadas: S3 obrigatório, porque o usuário escolheu disco quando não configurado; fallback em outage, porque o usuário escolheu reportar a falha sem alternar o backend.

### 6. Contrato primeiro e cleanup por comportamento

Na aplicação futura, criar `plan.md` com passos TDD por task ID dentro desta change; usar branch `codex/fresh-system-baseline` e worktree `.worktrees/fresh-system-baseline`. Tratar primeiro schema/contrato Go e depois Manager. Cada arquivo tem um único escritor; revisões usam evidências `file:line`.

Remover fixtures de remodelagem e testes cujo único objetivo é upgrade, conversão ou tolerância histórica. Preservar/adaptar testes de RBAC, validação, concorrência, replay, transferência de protocolo, mídia e correlação atuais. Revisar referências sem depender apenas de busca por `legacy`, pois código morto pode não ter esse nome e fixtures válidas podem ter nomes antigos.

Manager mantém comportamento e layout atuais, retirando hints de nomes históricos e tipos/conversões desnecessários. Atualizar README, AGENTS, exemplos vigentes e schemas Swagger. Não reescrever changes arquivadas; na sincronização futura, atualizar os Purpose das specs principais que ainda descrevem remodelagem histórica. Esta entrega altera apenas `openspec/changes/fresh-system-baseline/`.

## Risks / Trade-offs

- [Risk] Confundir exclusão de legado com exclusão de um fluxo ativo, como import Chatwoot ou `pending:{uuid}` -> Mitigation: inventário por comportamento, testes atuais e revisão do diff por subsistema.
- [Risk] Baseline omitir uma FK, índice ou tabela válida -> Mitigation: comparar catálogo esperado das 15 tabelas e executar repositórios reais sobre schemas isolados.
- [Risk] Exclusão de testes ocultar regressão de segurança -> Mitigation: manter cobertura de autenticação, privacidade, RBAC, concorrência e replay autorizado.
- [Risk] Tokens sem chave válida interromperem Chatwoot -> Mitigation: falhar explicitamente na configuração e testar conector ligado/desligado e tokens vazios/não vazios.
- [Risk] Remover volumes de outro projeto -> Mitigation: verificar mounts reais e usar exclusivamente os quatro nomes aprovados, sem prune global ou remoção de volumes de caches/dev.
- [Risk] Reset sem backup causar perda definitiva -> Mitigation: decisão expressa do usuário; executar somente após gates, sem prometer recuperação de dados, rollback ou migração de sessões.
- [Risk] Falta de número de teste impedir evidência de envio real -> Mitigation: registrar o cenário como não exercitado e não declarar verificação completa de WhatsApp com fakes.

## Migration Plan

Este é um plano de reinstalação futura, não upgrade. Nenhuma ação operacional é executada na proposta.

1. Implementar e revisar os subsistemas no worktree, preservando alterações locais alheias; passar gates Go, Swagger e Manager e integrações dedicadas.
2. Parar a única réplica local e confirmar mounts dos volumes `wzap_pgdata`, `wzap_natsdata`, `wzap_miniodata` e `wzap_media`.
3. Remover somente esses quatro volumes diretamente, sem backup; descartar dados e sessões anteriores sem conversão ou importação.
4. Reconstruir o código novo e iniciar uma única réplica no projeto Compose existente; recriar `wzap_test`. Não iniciar o worktree e o checkout principal simultaneamente contra o mesmo banco.
5. Validar readiness/liveness, seed, login, nova instância e mídia; novo pareamento, envio e webhook exigem número de teste disponível.
6. Registrar `verify.md` com os sete checks e evidências reais, revisão e `retrospective.md`; sincronizar specs e arquivar por último somente após concluir tarefas e resolver bloqueios. Sem backup, a recuperação de falha usa correção do código e nova instalação vazia, sem restauração de dados anteriores.
