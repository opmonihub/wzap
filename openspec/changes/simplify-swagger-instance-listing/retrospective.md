# Retrospectiva

## 0. Evidências

Pedido de browser em 2026-10-05; três relatórios de implementação/revisão e revisão final em `.superpowers/sdd/plan/`; gates.json com cinco gates PASS; RED/GREEN HTTP127 e SQL123 em schemas isolados; quatro checks de composables com1.001 instâncias; duas gerações Swagger idênticas; imagem local construída com sucesso. Verify.md contém comandos e limites. Este registro precede o merge e o teste visual; fatos posteriores serão anexados.

## 1. O que funcionou

Fixar o contrato data.items primeiro permitiu backend e Manager evoluírem em escopos separados. TDD tornou observáveis o corte após50/100 itens e a perda de instâncias do usuário depois da fronteira da página. Postgres real verificou ordenação por timestamp e UUID independentemente.

## 2. O que exigiu correção

O comando genérico de geração inicial foi corrigido para o comando da CI antes da execução. Um fake de criação ainda tinha assinatura antiga e foi ajustado após falha de compilação; a suíte focada passou depois. A ausência de next_cursor revelava repetição de20 páginas no overview antigo; a consulta única resolveu a causa.

## 3. Decisões preservadas

Manter data.items reduz a migração dos consumidores. Ignorar queries antigas segue o comportamento de queries desconhecidas. Headers de segurança e correlação continuam operacionais, com entrada da chave concentrada no Authorize. Paginação visual do console e de outras coleções fica preservada.

## 4. Verificação e limitações

Gates Go completos, build Nuxt, regressões de clientes, PostgreSQL focado e quatro revisões aprovaram a alteração. Nenhum teste opcional foi apresentado como executado sem variável/banco. Integração NATS e serviços externos não foram exercitados; o navegador e a integração na main ainda precisam do registro posterior.

## 5. Riscos restantes

Listagens grandes aumentam memória e tamanho da resposta, troca explicitamente solicitada e documentada, sem teto oculto. Clientes externos que usam next_cursor precisam migrar. Avisos de ferramentas existentes permanecem documentados e não indicam falha do código da mudança.

## 6. Próximos cuidados do fluxo

Integrar localmente preservando os48 caminhos locais e as oito sobreposições, reiniciar somente wzap com a imagem revisada, registrar Authorize/Execute e limpar worktree após merge. Sincronizar specs e arquivar a mudança no encerramento específico OpenSpec; nenhum push ou PR foi solicitado.
