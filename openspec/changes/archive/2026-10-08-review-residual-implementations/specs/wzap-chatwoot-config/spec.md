## ADDED Requirements

### Requirement: Token opaco sempre cifrado na persistência

Todo token de entrada não vazio SHALL ser tratado como plaintext opaco e persistido como ciphertext autenticado, inclusive quando começa pelo prefixo reservado ao armazenamento. A leitura interna autorizada SHALL recuperar exatamente o token de entrada; empty, adulteração e chave incorreta SHALL preservar suas regras existentes.

#### Scenario: Token iniciado pelo prefixo
- **WHEN** a configuração recebe token que começa com enc:v1:
- **THEN** seu valor SQL não é plaintext e a leitura interna recupera exatamente o token

### Requirement: Token de acesso sem transferência por redirect

Requisições autenticadas ao Chatwoot MUST NOT seguir redirects nem transferir api_access_token para outro destino; 3xx SHALL ser falha do pedido.

#### Scenario: API redireciona
- **WHEN** uma operação responde 307 para outro origin ou HTTP
- **THEN** o destino secundário não recebe chamada nem token e a operação informa falha
