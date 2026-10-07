## ADDED Requirements

### Requirement: Busca de grupo sem identificador

A seção de grupos SHALL recusar a busca quando o campo de JID está vazio ou só tem espaço, e MUST NOT chamar a API nesse caso. O grupo exibido permanece o anterior.

#### Scenario: Busca com campo vazio

- **WHEN** a conta aciona a busca de grupo com o campo de JID vazio
- **THEN** o console não emite `GET` de grupo e mostra o erro de identificador ausente sem trocar o grupo em tela
