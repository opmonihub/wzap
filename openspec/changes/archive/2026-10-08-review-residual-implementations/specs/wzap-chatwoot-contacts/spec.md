## ADDED Requirements

### Requirement: Correspondência exata antes de seleção e merge

Resultados de busca SHALL corresponder exatamente às variantes normalizadas da mesma identidade antes de selecionar contato ou fundir duplicatas. Um telefone que apenas contém os dígitos de outro MUST NOT ser usado nem fundido; buscas por identifier de grupo ou fallback SHALL exigir identifier exato.

#### Scenario: Contains retorna contato alheio mais longo
- **WHEN** a busca retorna contato BR exato e telefone alheio mais longo contendo seus dígitos
- **THEN** somente a identidade exata/variantes BR participa da seleção e merge

#### Scenario: Identifier parcialmente coincidente
- **WHEN** a busca por identifier retorna string apenas parcialmente correspondente
- **THEN** o contato alheio não é selecionado nem correlacionado
