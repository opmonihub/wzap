## Context

Ver proposal.md. A listagem usa limite e cursor no HTTP, serviço e Postgres. Stats, restauração de sessões, scheduler Chatwoot e três telas do Manager acumulam essas páginas. O Swagger combina @Security com parâmetros de header duplicados.

## Goals / Non-Goals

Retirar a paginação de instâncias em todos esses consumidores sem alterar as demais coleções ou a autenticação. Não transformar a resposta em array raiz: data.items preserva o acesso atual dos clientes.

## Decisions

1. List(ctx) retorna ([]model.Instance, error), e o SELECT mantém ORDER BY created_at DESC, id DESC sem LIMIT ou cursor. Eliminar o contrato paginado evita manter uma função interna obsoleta. Não adicionar limite escondido.
2. GET /instances mantém data.items, retira next_cursor e ignora parâmetros antigos: queries desconhecidas já são toleradas no transporte, evitando validação artificial de parâmetros removidos.
3. Stats e consumidores internos consultam a coleção uma vez; preservar o processamento, filtros e tratamento de erros existentes. O Manager mantém a paginação local da tabela.
4. @Security apikey é a única declaração de credencial das operações. Remover @Param apikey e @Param X-Request-Id globalmente, mantendo securityDefinitions, auth real, middleware e @Header de resposta.
5. Três escopos de escrita separados: backend/listagem (inclui instances.go), Manager e Swagger (demais arquivos anotados e testes de contrato); geração final ocorre após o backend terminar.

## Risks / Trade-offs

- [Risk] Uma coleção muito grande usa mais memória e produz respostas maiores -> assumir a listagem completa solicitada, preservar cancelamento por contexto e registrar a troca, sem limite oculto.
- [Risk] Consumidores externos dependem de next_cursor -> marcar **BREAKING** no README e nos artefatos, atualizar todos os consumidores do repositório juntos.
- [Risk] Swagger servido vem de binário antigo -> verificar o novo documento e testar Authorize/Execute no navegador após atualizar o serviço local.
- [Risk] Main tem mudanças do usuário em arquivos compartilhados -> implementar em worktree limpa, salvar e reaplicar apenas os deltas locais sobre a integração, verificar preservação antes de remover a worktree.

## Migration Plan

Verificar e revisar a worktree, integrar localmente na main conforme a preferência já escolhida pelo usuário, atualizar o serviço local e conferir o Swagger. Rollback do contrato requer reverter o commit da alteração; nenhum dado ou esquema de banco muda. Sincronização/arquivo OpenSpec seguem o encerramento próprio do fluxo.
