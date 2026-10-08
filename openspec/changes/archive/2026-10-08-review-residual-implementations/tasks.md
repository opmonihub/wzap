## 1. API e autenticação

- [x] 1.1 Revalidar existência e role atuais de sessões com regressões de login, remoção, role alterada e falha de repositório em `go test ./internal/httpapi/... ./internal/auth/... -count=1`.
- [x] 1.2 Corrigir replay/422/409 em PUT/PATCH e isolamento de recursos concretos mantendo aliases equivalentes, verificado pelos testes HTTP de idempotência.
- [x] 1.3 Rejeitar status/envelopes de replay corrompidos com 500, zero efeitos e chave conservada, mantendo bytes válidos literais nos testes de core.
- [x] 1.4 Exigir EOF e limite após o primeiro JSON, verificando 400/413 e zero efeitos para conteúdo extra/padding sem recusar campos desconhecidos.
- [x] 1.5 Alinhar campos multipart consumidos ao fingerprint, verificando query, valores repetidos e destinatários/payloads divergentes em mensagens e status.
- [x] 1.6 Limitar buckets ativos do login sem reiniciar budgets existentes, verificado com mais de MaxLoginBuckets IPs e expiração determinística.

## 2. Núcleo e integrações

- [x] 2.1 Preservar campos omitidos de identidade/webhook em PATCH concorrente com regressões determinísticas e `go test -race ./internal/instance -count=1`.
- [x] 2.2 Remover JIDs internos de logs/erros de restauração e projeção pública, verificado com marcadores sintéticos nos testes de sessão/representação.
- [x] 2.3 Bloquear redirects em webhook e cliente Chatwoot autenticados, verificando que destinos secundários e HTTP não recebem chamada/credencial.
- [x] 2.4 Cifrar todo token Chatwoot não vazio incluindo prefixo do envelope, verificando round trip, ciphertext SQL e adulteração/chave incorreta.
- [x] 2.5 Bloquear exceção SSRF de hosts com DNS parcialmente sobreposto ao Chatwoot, verificando dial/redirect e host privado configurado legítimo.
- [x] 2.6 Isolar recibos por instance_id e wa_id, verificando duas instâncias com mesmo wa_id em Postgres isolado e projeção de eventos correspondente.
- [x] 2.7 Preservar chunks novos durante import e lote original na falha, verificando snapshot/import concorrente com barreiras e dedup existente.
- [x] 2.8 Manter coleta history-sync inerte sem URI e funcional quando habilitada, verificado em sessões/fakes e fluxo de conclusão.
- [x] 2.9 Persistir estado terminal e evento na mesma transação com event_id estável e retry sem novo envio WhatsApp, verificando sent/failed, rollback SQL e falhas de persistência.
- [x] 2.10 Filtrar contatos por variantes normalizadas exatas e identifier exato antes de escolher/fundir, verificando resultados contains alheios, BR e grupos.
- [x] 2.11 Iniciar sessão fresca no comando init e init:number e ocultar JID no status, verificado com fluxo real de lifecycle e confirmações sintéticas.

## 3. Manager

- [x] 3.1 Permitir alteração/limpeza apenas do recado e preservar aviso 501 de nome/foto com testes do fluxo de perfil.
- [x] 3.2 Manter detalhe e URL válidos após rename por alias usando UUID e preservando section, verificado por recarga/ação posterior.
- [x] 3.3 Recusar coordenadas vazias/espaços/ausentes sem chamar API e preservar zeros explícitos nos testes do composer.
- [x] 3.4 Renderizar status de mídia sem texto/legenda com identificação e ação Delete, verificado para mídia/texto e coleção vazia.
- [x] 3.5 Disponibilizar revogação a admin sem depender de cache local ou freshKey, verificando sucesso/falha, storage indisponível e usuário comum.

## 4. Revisão e validação

- [x] 4.1 Completar a segunda passagem read-only das áreas restantes, registrar confirmação/descarte dos candidatos e incorporar apenas achados demonstrados antes de editar.
- [x] 4.2 Concluir revisão cruzada, gates Go/manager, integração Postgres com schemas isolados e NATS temporário, Swagger freshness, OpenSpec strict, verify.md e retrospective.md, preservando as alterações locais anteriores.
- [x] 4.3 Incluir e preservar o placeholder de embed documentado após build Nuxt, verificando build/test Go sem assets e 503 da console não construída.
