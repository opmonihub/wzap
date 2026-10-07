# Retrospectiva

## 0. Evidências

Pedido de browser em 2026-10-05; dois relatórios de exploração, três implementações em escopos exclusivos, três revisões de tarefa e uma re-revisão; branch-gates.json com sete checks técnicos PASS; testes PostgreSQL reais em schemas isolados, sete helper checks, Nuxt typecheck/build; documentos Swagger repetíveis. Registro anterior ao merge e revisão final; adicionar evidência posterior sem reescrever os fatos.

## 1. O que funcionou

Fixar GetByName, gramática e códigos antes de separar backend, HTTP e Manager manteve a identidade UUID entre componentes. Roteador real comprovou aliases, estatísticas e permissões; PostgreSQL real comprovou claims simultâneos e comparação do nome sob lock.

## 2. O que exigiu correção

A revisão encontrou um P2: validação no serviço usava leitura antiga e permitia restaurar nome legado inválido após renomeação concorrente. Predicado único compartilhado e validação da mudança efetiva dentro do lock corrigiram seis casos determinísticos. O parser UUID aceita wrappers arbitrários de 38 caracteres; helper Manager e testes cobrem a mesma reserva. Ordem dos annotations Failure/Header foi corrigida para conservar X-Request-Id em 409.

## 3. Decisões preservadas

Nomes exatos de 1–64 ASCII, distinção de caixa e stats/UUID reservados; nenhuma normalização automática. Nomes legados mantidos e updates exatamente inalterados permitidos. Sem migração por dados legados e migration00007 local; claims transacionais, busca ambígua409 e identidade UUID continuam explícitos.

## 4. Verificação e limitações

Gates completos Go, testes focados PostgreSQL realmente executados, Manager helper/types/build e Swagger freshness passaram. Checks não representam integração NATS ou chamadas reais às plataformas. Só arquivos próprios foram commitados; root ainda possui 48 caminhos locais protegidos.

## 5. Riscos restantes

Escritores antigos/SQL direto estão fora do protocolo de unicidade; lookup nunca escolhe arbitrariamente nome duplicado. Renomear muda o alias imediatamente; integrações que precisam de referência imutável podem manter o UUID. Nomes legados fora da gramática permanecem acessíveis por UUID.

## 6. Próximos cuidados do fluxo

Revisão final, imagem limpa Go1.26, integração local preservando alterações do usuário, gates da main e browser Authorize/Execute por nome/UUID. Remover somente worktree/branch desta tarefa após sucesso. Sincronização e arquivo OpenSpec ficam na etapa própria, sem push/PR.

## Registro posterior — revisão final e imagem

Revisão final independente APPROVED em 0cd8e4e, sem findings; checagem de fontes, contrato das 73 operações e hashes gerados. Imagem wzap:instance-name-20261005 construída com sucesso, Go1.26 e Manager, mantendo a fonte revisada. Merge/visual ainda não executados neste registro.

## Registro posterior — main e Swagger

Merge493f644 na main, restauração das 48 alterações locais, conflito PostgreSQL e adapter privado revisados/APPROVED. Provas:40 originais idênticos, seis merges automáticos exatos, adapter invertível byte a byte e resolução manual preservando todos os campos/métodos do usuário. Gates Go completos, typecheck/helper7/testes locais Manager PASS; PostgreSQL nomes/compatibilidade5.924s real e Swagger sem diff. Somente app atualizado, DB/NATS IDs iguais, health/ready200; seis consultas por UUID/nome equivalentes e três Execute no browser por FELIPE200 com uma autorização global, stats total1. Artefatos/proteção em /home/obsidian/dev/wzap/.superpowers/sdd/finished-support-instance-name-addressing-6x4o0s_p. Nenhum dado de instância alterado para o teste; imagem contém apenas fonte revisada.
