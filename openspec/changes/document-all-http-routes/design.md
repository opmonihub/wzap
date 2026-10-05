## Context

Ver proposal.md para a motivação. O inventário inicial tem 88 operações explícitas e 82 documentadas. Swaggo v1.16.6 já gera os artefatos usados pelo servidor; o CI já verifica sua atualização. A árvore atual contém alterações locais do usuário, copiadas para uma worktree isolada antes desta mudança.

## Goals / Non-Goals

**Goals:** cobertura de método/path e schemas fiéis sem mudar handlers, autorização, armazenamento ou formatos de resposta.

**Non-Goals:** transformar mounts estáticos em uma lista de assets ou corrigir problemas funcionais independentes.

## Decisions

1. Continuar usando anotações junto aos handlers e composição `envelope{data=Tipo}`. Uma segunda especificação manual duplicaria os contratos e permitiria divergência. A estrutura `envelope` existente é reutilizada.
2. Documentar os cinco handlers Chatwoot e as entradas públicas úteis do Manager. O webhook entrega `content` diretamente; importação retorna o número já importado, não um identificador de job. O token de configuração é write-only e sua resposta permanece vazia.
3. Ler os registros de rotas com o parser Go em um teste e compará-los ao documento efetivamente servido em `/swagger/doc.json`. Isso acompanha novas rotas sem uma segunda lista fixa e captura omissão de métodos em paths compartilhados. Montagens estáticas/fallback sem método explícito não são operações REST; entradas públicas úteis podem ser documentadas adicionalmente.
4. Verificar schemas padronizados e exceções reais no Swagger servido. Não afirmar que todos os sucessos são envelopados: downloads, 204, webhook e superfícies HTML têm contratos próprios. Swagger 2.0 não possui segurança cookie; explicar a alternativa na descrição sem criar um esquema inválido.
5. Aplicar somente o delta desta tarefa de volta à árvore original, conferindo os arquivos contra um snapshot inicial. Não incluir alterações anteriores do usuário em commits.

## Risks / Trade-offs

- [Generator interpreta composição de schemas] → Gerar com a versão pinada e conferir JSON servido, referências e coleções.
- [Registro não literal de novas rotas pode escapar do inventário] → Limitar o teste ao padrão atual e falhar em registros com padrões não literais ou métodos desconhecidos.
- [Exceções públicas confundidas com REST] → Documentar formatos/status reais e manter exceções de schema explícitas e pequenas.
- [Alterações locais misturadas] → Worktree `.worktrees/document-all-http-routes`, snapshot e aplicação apenas do delta próprio.

## Migration Plan

Nenhuma migração de dados. Gerar docs, executar verificações e reconstruir o binário para publicar o documento embutido. Reverter os arquivos desta mudança restaura a documentação anterior.
