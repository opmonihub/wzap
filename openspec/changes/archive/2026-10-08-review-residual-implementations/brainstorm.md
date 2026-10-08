# Registro da intenção

Pedido do usuário: lançar subagentes revisores, orquestrados em direções específicas, para revisar toda a codebase e revisar/refatorar problemas residuais das implementações recentes.

Decisão de execução: três revisores read-only para API/auth, núcleo Go/integrações e manager; o coordenador cobre operações/empacotamento e gates. Confirmar achados com reprodução/teste antes de corrigir, usar worktree em `.worktrees/`, manter um writer por arquivo e submeter correções a revisão cruzada.

Critério de sucesso: achados acionáveis corrigidos com regressões, cobertura/limitações registradas e gates apropriados aprovados, mantendo os contratos e as alterações locais do usuário. A segunda passagem fecha lacunas de cobertura; riscos sem evidência suficiente não viram mudanças especulativas.

Escopo autorizado pelo pedido inclui revisão e correções locais reversíveis. Não há solicitação de publicação, merge, novos módulos ou migração.
