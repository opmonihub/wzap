## ADDED Requirements

### Requirement: Confidencialidade da credencial perante redirects

Entregas autenticadas SHALL enviar a credencial somente ao destino configurado. Respostas de redirecionamento MUST ser tratadas como falha de entrega e MUST NOT provocar nova requisição que encaminhe apikey a outro destino ou conexão HTTP.

#### Scenario: Redirect para outro origin
- **WHEN** o destino responde 307 apontando para outro servidor
- **THEN** o segundo servidor não recebe requisição nem apikey e a entrega é considerada falha

#### Scenario: Downgrade de transporte
- **WHEN** o destino HTTPS responde redirect para HTTP
- **THEN** a credencial não é enviada ao destino HTTP
