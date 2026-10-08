## Context

Ver proposal.md e os relatórios de review.md. HEAD inicial f9d1300; as alterações locais de instruções/specs são contexto e não pertencem a esta entrega. A baseline Go completa, vet/build/lint e manager test/lint/typecheck/build passaram, mas não cobrem os cenários identificados.

## Goals / Non-Goals

Corrigir causas demonstradas com interfaces internas mínimas e testes comportamentais; manter Chi, DTOs compartilhados, efeitos síncronos e limites entre transportes/domínio/storage. Não criar camada de compatibilidade, novos serviços ou mecanismos de revogação de logout.

## Decisions

1. Sessões consultam a conta atual antes de autorizar; usar a role persistida, rejeitar conta inexistente e falhar fechado em erro. API keys continuam independentes. Alternativa descartada: blacklist JWT/migração, pois a conta existente já é a fonte da autorização.
2. Idempotência cobre POST/PUT/PATCH das rotas explicitamente envolvidas; o fingerprint inclui recurso concreto e normaliza só o alvo da instância, mantendo equivalência UUID/nome. Replay valida status e envelope sem transformar bytes íntegros e conserva a chave na corrupção. Campos desconhecidos do JSON continuam tolerados, mas um segundo valor/lixo é inválido; payload acima do limite resulta 413.
3. Handlers multipart usam somente os campos do formulário. A ordem efetiva de valores repetidos deve fazer parte do fingerprint, ou duplicatas escalares devem ser recusadas explicitamente; preferir preservar semântica existente quando possível. Nunca executar efeito para conteúdo divergente com a mesma chave.
4. Limiter público tem um teto real de buckets: remover expirados, conservar budgets ativos e recusar novos IPs quando cheio. Evitar evicção de entradas ativas, que permitiria reiniciar o orçamento de tentativas.
5. `Service.Update` tem locker próprio por instância adquirido antes de ler e mantido até as gravações; identidade só é gravada quando presente. O locker é separado de envio/import para evitar reentrância. Uma réplica é o runtime suportado; não é necessário adicionar lock distribuído ou migração.
6. Remover JIDs de mensagens e campos de log na origem dos erros de dispositivo, inclusive dados vindos de erros upstream que possam alcançar `connection.last_error`; preservar identificadores opacos e classificação útil.
7. Clientes autenticados não seguem redirects; 3xx é falha existente, com corpo fechado normalmente. Isso evita transferência de credencial entre origins ou downgrade e dispensa política adicional de redirects. Todo token não vazio em Put é plaintext e sempre deve ser cifrado, inclusive quando começa com o prefixo do armazenamento; Get retorna plaintext.
8. Manager envia somente campos alterados do perfil; recado vazio continua explícito e nome realmente alterado conserva o 501 sem sucesso parcial silencioso. Recargas usam o UUID carregado, e rename corrige a URL preservando section. Coordenadas vazias são inválidas antes de Number, zero explícito é válido. Status sempre mantém sua linha/ação independentemente de legenda. Revogação de key depende da autorização, não do cache do navegador.
9. Segunda passagem do núcleo reproduz candidatos restantes antes de ampliar tasks/specs; nenhuma mudança nasce apenas de hipótese. Revisores cruzam as correções de outra área.
10. Recibos usam chave composta instance_id/wa_id no contrato interno e no SQL. MarkSent/MarkFailed recebem model.OutboxEvent e persistem estado sent/failed e seu evento na mesma transação do banco existente, retornando se ocorreu transição. Um evento construído mantém event_id durante tentativas de persistência, conservando o resultado do envio. Fan-out ocorre via notifier somente após commit, sem Writer.Write duplicado; um resultado terminal correspondente permite reconhecer commit incerto, e chamadas já confirmadas não recriam eventos removidos pelo relay. Claims ativos ficam fora da recuperação enquanto aguardam persistência; retry não chama WhatsApp novamente. A janela de crash anterior ao commit permanece, sem alegar exactly-once de transporte. Estados terminais legados sem evento não recebem backfill: depois que o relay remove a row, não se distinguem de estados com evento já publicado sem adicionar um marcador/migração.
11. Import reconhece somente o snapshot processado; chunks novos continuam pendentes e falha não descarta o lote. History-sync é habilitado explicitamente somente com URI de import. A exceção SSRF privada exige identidade confiável do host/endereço completo, sem aceitar uma única coincidência dentro de lista mista; dial e redirect aplicam a mesma regra. Busca contains de contatos é filtrada por variantes normalizadas exatas/identifier exato antes de longest/merge. init reutiliza o lifecycle de conexão que cria sessão fresca; status não ecoa device_jid.
12. `manager/.output/public/.gitkeep` é parte da entrega como placeholder documentado: clone sem Nuxt compila Go e a console não construída conserva 503. O script build recria o placeholder com Node/fs após nuxt generate, que remove o diretório anterior. Build Nuxt continua ignorado, sem versionar seus assets.

## Risks / Trade-offs

- [Consulta de usuário em cada sessão exige DB] -> falhar fechado, exercitar erros e preservar API keys globais.
- [Fingerprints antes incorretos não correspondem a alvos corrigidos] -> responder divergência com 422, nunca repetir efeito; conservar formato de fingerprints nas rotas não afetadas quando possível.
- [Deadlock em regressões concorrentes] -> barreiras/eventos determinísticos, sem sleeps e sem reutilizar locks reentrantes de outros fluxos.
- [Cliente externo configurado com URL que redireciona] -> retornar a falha de status e exigir destino final explicitamente, mantendo credenciais confidenciais.
- [Testes de UI puramente derivados não exercitam componente] -> usar fluxo real de componentes/composables ou helpers de produção utilizados por eles, e verificar renderização/ações relevantes sem novas dependências.
- [Revisão ampla deixa fronteiras sem evidência runtime externa] -> registrar cobertura e limitar afirmações às suites/probes executados, incluindo integrações opcionais realmente habilitadas.
- [Persistência pós-envio falha] -> evento e estado atômicos com retry apenas de persistência; preservar event_id e cobrir rollback real de SQL; janela de crash externo do envio continua documentada, sem promessa de exactly-once.

## Migration Plan

Nenhuma migração. Validar no worktree, integrar apenas arquivos da change como diff local com `git apply --check`, conferir hashes das alterações preexistentes e entregar artefatos/evidências. Rollback remove somente o patch desta change; não altera banco ou serviço ativo.
