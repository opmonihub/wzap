# Retrospective — remodel-storage-and-json-responses

Retrospectiva pós-implementação (etapa 6 concluída em 2026-10-06, HEAD
`49a0024`). Escopo: a change inteira (persistência remodelada, mídia em
MinIO, durabilidade/replay, contrato HTTP/Manager/Swagger, verificação).

## O que funcionou bem

- **Estratégia não destrutiva da migração 00008**: expandir+renomear+backfill
  num único corte, com tabela `remodel_report` de auditoria e nenhuma
  invenção de dados (órfãos anulados e reportados; marcadores `chatwoot-*`
  nunca viram `wa_id`; erro legado sem data inventada). O ensaio de corte
  confirmou na prática: nenhum UUID perdido, owners/quotas intactos, 0
  referências órfãs após o corte.
- **Contrato fechado antes do código**: `response-matrix.md` + design §5
  definiram cada rota antes dos DTOs; a suíte de contrato (89 fixtures) e o
  Swagger regenerado saíram coerentes entre si e com o Manager.
- **Idempotência com política explícita para o legado**: converter por rota ou
  responder 410 sem reexecutar evitou o pior cenário (corpo antigo com campos
  removidos ressurgindo, ou mensagem duplicada). O ensaio com chaves criadas
  pré-corte e replayadas pós-corte validou a travessia do corte.
- **Ensaio de recuperação real**: clone restaurado idêntico ao baseline deu
  evidência (não suposição) de que o rollback funciona. O script
  `rehearsal-inventory.sql` tornou a comparação pré/pós reproduzível.
- **Fix round 2 de mídia bem direcionado**: o achado do re-review (Exists na
  bucket da row vs Put na bucket configurada) era real e a correção final
  (`49a0024`: upload na bucket configurada + `SetBucket` na row) foi
  comprovada no ensaio com a classe fs-era (`bucket='local'`).

## O que atrapalhou

- **Churn de escrita paralela em `internal/media`**: três commits de fix
  (`f248304`, `ad75dbb`, `49a0024`) mudaram a semântica de transferência
  durante a própria execução do ensaio. A transferência foi coletada contra
  `ad75dbb` e precisou de uma rodada complementar contra `49a0024`. Escopos
  disjuntos evitaram corrompimento, mas o custo de coordenação foi real.
- **Sem sessão WhatsApp para o caminho 202**: o ensaio não pode demonstrar ao
  vivo o store+replay de uma resposta 202 (o resolver de número exige sessão
  real). A semântica de liberação de chave para 4xx/503 só ficou clara lendo o
  middleware — a documentação de replay nos testes dá a entender que qualquer
  resposta é armazenada.
- **pg_dump 18 e o schema `public`**: o dump inclui `CREATE SCHEMA public`, que
  falha num banco recém-criado; a restauração exige `DROP SCHEMA public`
  primeiro no clone. Descoberto na primeira tentativa de restore (custo baixo,
  mas não documentado no preflight).
- **Fingerprint de idempotência não documentado como contrato**: calcular o
  `request_hash` offline exigiu ler o código (`sha256` de método + `r.Pattern`
  + body — e `r.Pattern` já inclui o método, armadilha clássica). Para ensaios
  e depuração de clientes, o algoritmo merece uma nota no README/spec.
- **Ambiente mistura dev/verificação**: o mesmo servidor Postgres hospeda o
  banco dev real e os bancos de teste; a disciplina de clones efêmeros
  (preflight) custou mais preparo, mas foi a decisão correta — nenhum banco
  real foi tocado.

## Decisões que o controller deve confirmar

1. **DEFER 1 (device_jid divergente)** é o único que toca a liberação do corte
   real: o design diz "bloqueia" e a migração "reporta e segue". Precisa de
   decisão antes do corte em produção (guard ou emenda de design).
2. Reconciliação de specs no sync: `specs/wzap-media/spec.md:35` (tag quay) e
   o histórico de `tasks.md` (`cccs/minio:latest`) → digest pinado.
3. Aceite do trabalho das sessões paralelas (fixes de mídia com
   `Co-authored-by` de terceiros) — já verificado por re-review e por este
   ensaio, mas a proveniência deve constar do merge.

## Aprendizados a levar para as próximas changes

- Ensaiar corte/recuperação **com chaves e corpos semeados pré-corte** rende
  evidência de travessia que testes de unidade não dão; fazer o seed antes do
  corte e o replay depois, no mesmo clone, é barato e forte.
- Inventário por digest md5 de conjuntos de IDs compara pré/pós de forma
  exata e compacta; vale virar ferramenta reusável (o
  `rehearsal-inventory.sql` já é).
- Políticas de edge (release de chave, 410, expiração) precisam estar no
  contrato documentado, não só no código: custaram tempo de descoberta.
- Mudança de contrato REST em um repo só (serviço + Manager + Swagger
  juntos) funcionou bem com a matriz primeiro; manter `event_version: 1`
  intocado foi o que manteve o consumidor de eventos sem BREAKING.
