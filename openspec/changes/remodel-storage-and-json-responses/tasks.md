## 1. Fechar condições de execução

- [x] 1.1 Fechar os cinco bloqueios do design, preenchendo tipos, nullabilidade, defaults, enums, índices, exclusões, rollback, vínculos legados, correlação pré-WA ID, catálogo de erros, matriz completa por rota e origem verificável da imagem exata; verificar que nenhuma decisão material permanece aberta.

  Registro (1.1): a tag MinIO aprovada foi substituída por `docker.io/cccs/minio:latest` por ruling do controller (quay 401 em todas as tags); o spec `wzap-media` ainda cita a tag quay e será reconciliado na sincronização de specs. Detalhes em design.md §"Decisões fechadas na task 1.1" e `.superpowers/sdd/plan/task-1.1-report.md`.
- [ ] 1.2 Coordenar a baseline e a migração 00007 com a change de origem, preparar worktree em `.worktrees/` e preflight com backups e fixtures; verificar que alterações locais alheias não entram no diff e que os bancos de teste são isolados.

## 2. Remodelar persistência

- [ ] 2.1 Implementar e ensaiar expansão, backfill e constraints das 14 tabelas; verificar upgrade e banco vazio em PostgreSQL com preservação de UUIDs, unicidade, timestamps, owners e dados recuperáveis.
- [ ] 2.2 Separar repositórios e comandos de identidade, conexão e webhook; verificar reconexão pelo dispositivo correto, exclusividade e edição concorrente sem regressão do estado.
- [ ] 2.3 Atualizar usuários, cache JID, metadados, configurações Chatwoot e dead letters para os nomes aprovados; verificar quotas, lookup global, unicidade por instância e limite de 500 falhas por instância.
- [ ] 2.4 Implementar FKs opcionais de mídia e correlação Chatwoot com isolamento por instância; verificar compartilhamento de mídia, múltiplos envios por cw_id e entradas sem fila artificial.

## 3. Migrar mídia para MinIO

- [ ] 3.1 Integrar cliente S3, ambiente e Compose com a imagem exata e rede existente; verificar inicialização e armazenamento contra a imagem comprovada sem credenciais fixas.
- [ ] 3.2 Adaptar upload, download, envio e mirror para objetos e temporários; verificar integridade, autorização e cleanup em sucesso, falha e cancelamento.
- [ ] 3.3 Implementar transferência verificável dos arquivos e expiração dos objetos; verificar preservação de IDs, SHA-256, tamanho, metadados e FKs, exclusão idempotente e rollback ensaiado.
- [ ] 3.4 Separar marcadores locais Chatwoot de wa_id reais conforme o relatório de migração; verificar que leitura, reply e revoke não usam marcadores sintéticos como IDs WhatsApp.

## 4. Preservar durabilidade e replay

- [ ] 4.1 Alterar o relay para remover somente eventos confirmados pelo JetStream; verificar indisponibilidade, reinício e falha de exclusão após PubAck com o mesmo UUID.
- [ ] 4.2 Capturar os erros estruturados nas quatro tabelas; verificar last_error público nulo ou estruturado, legado sem data inventada e fixtures de eventos v1 inalteradas.
- [ ] 4.3 Registrar os UUIDs de saídas Chatwoot conforme a política fechada em 1.1; verificar janela pré-WA ID, múltiplos anexos, falhas parciais e deduplicação de mirror/import.
- [ ] 4.4 Converter respostas legadas de idempotência sem repetir efeitos; verificar autorização, fingerprint, status HTTP, UUID, chave entregue uma vez e política segura para corpos não conversíveis.

## 5. Atualizar contrato HTTP e consumidores

- [ ] 5.1 Implementar DTOs e mapeadores de todas as operações da matriz fechada; verificar envelopes, coleções vazias, ocultação de campos, comandos e exceções com fixtures por rota.
- [ ] 5.2 Atualizar tipos e fluxos do Manager, incluindo instances_used e flags Chatwoot; verificar quotas, formulários sem apagar campos ocultos, opções de webhook e executar testes, typecheck e build.
- [ ] 5.3 Atualizar anotações dos handlers e regenerar Swagger; verificar cobertura de rotas, exemplos e segurança global sem campo apikey duplicado.

## 6. Verificar e concluir a implementação futura

- [ ] 6.1 Fazer revisão independente e executar os checks Go, Manager, PostgreSQL, NATS e MinIO previstos no plano; registrar os ambientes realmente executados e corrigir falhas antes do corte.
- [ ] 6.2 Ensaiar corte e recuperação com backups, produzir verify.md e retrospective.md e concluir o fluxo de integração; verificar evidências, marcar somente tarefas comprovadas e arquivar a change por último.
