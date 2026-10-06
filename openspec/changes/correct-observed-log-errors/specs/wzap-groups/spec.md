## ADDED Requirements

### Requirement: Foto de grupo que não é imagem

Bytes enviados em `PUT /instances/{id}/groups/{group_id}/photo` com `Content-Type` `image/*` que o upstream não aceita como imagem SHALL responder `422` e MUST NOT alterar a foto do grupo. A resposta MUST NOT ser `500`.

#### Scenario: Corpo não decodificável como imagem

- **WHEN** o cliente envia um corpo `image/*` que não é uma imagem válida para uma instância conectada
- **THEN** a resposta é `422` e a foto do grupo permanece a anterior
