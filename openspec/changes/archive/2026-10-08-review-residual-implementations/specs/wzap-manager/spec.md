## ADDED Requirements

### Requirement: Perfil editado por campos alterados

O manager SHALL permitir alterar ou limpar somente o recado sem enviar nome inalterado. Nome/foto não suportados SHALL continuar apresentando o aviso 501; o manager MUST NOT informar sucesso parcial silencioso quando nome e recado são alterados juntos.

#### Scenario: Apenas recado
- **WHEN** o operador altera ou limpa o recado mantendo o nome
- **THEN** o pedido contém apenas status_text, inclusive vazio explícito, e o recado é aplicado

#### Scenario: Nome realmente alterado
- **WHEN** nome alterado recebe 501, isolado ou junto ao recado
- **THEN** o manager apresenta o aviso sem anunciar atualização concluída

### Requirement: Navegação preservada após rename por alias

Após rename em um detalhe aberto por alias, o manager SHALL recarregar usando identidade válida e SHALL manter URL atualizável com section preservada.

#### Scenario: Ação posterior ao rename
- **WHEN** o detalhe aberto pelo nome antigo é renomeado e depois recarregado ou atualizado
- **THEN** o pedido usa o UUID válido e refresh da URL mantém a instância e a seção

### Requirement: Coordenadas explícitas no envio de localização

O formulário SHALL exigir latitude e longitude presentes antes da conversão numérica. Vazio ou espaços MUST NOT ser interpretados como zero; zero explícito SHALL permanecer válido.

#### Scenario: Coordenada ausente
- **WHEN** pelo menos uma coordenada está vazia ou contém apenas espaços
- **THEN** o manager mostra erro e não chama a API

#### Scenario: Origem explícita
- **WHEN** latitude e longitude são preenchidas explicitamente com zero
- **THEN** o payload contém zero em ambos os campos

### Requirement: Status visível sem legenda

Todo status existente SHALL manter linha identificável e ação de remoção, mesmo sem text/caption. Estado vazio SHALL depender de coleção vazia e não de ausência de legenda.

#### Scenario: Mídia sem legenda
- **WHEN** o status de imagem ou vídeo não possui texto nem legenda
- **THEN** a linha identifica o status e mantém Delete disponível

### Requirement: Revogação independente do cache de key

A disponibilidade de revogação SHALL depender da autorização administrativa e estado da operação, sem exigir reconhecimento da key no cache local. Key recém-rotacionada SHALL poder ser revogada; confirmação e apresentação única SHALL ser preservadas.

#### Scenario: Outro navegador
- **WHEN** admin acessa settings sem cache local ou com storage indisponível
- **THEN** pode confirmar a revogação sem rotacionar antes

#### Scenario: Key recém-rotacionada
- **WHEN** admin revoga a key ainda apresentada uma única vez
- **THEN** sucesso limpa a exibição/cache e falha conserva estado para nova tentativa
