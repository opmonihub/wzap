# wzap-phone-pairing Specification

## Purpose

Permitir parear instância por código numérico de 8 dígitos como alternativa
ao QR.

## Requirements

### Requirement: Emissão do código de pareamento

O serviço SHALL emitir o código de 8 dígitos para número válido quando
houver canal de pareamento aberto e MUST responder `409` sem canal aberto
ou instância já conectada.

#### Scenario: Código emitido

- **WHEN** o cliente pede código para número válido com canal aberto
- **THEN** a resposta traz `pairing_code` e expiração alinhada ao canal

#### Scenario: Sem canal de pareamento

- **WHEN** o cliente pede código sem `connect` prévio
- **THEN** a resposta é `409` e nenhum código é emitido

### Requirement: Expiração lida antes da emissão

A expiração do canal SHALL ser lida antes de emitir o código; se o canal
expirar ou cair entre a leitura e a emissão, a resposta SHALL ser `409`
sem código emitido. O serviço MUST NOT abrir canal implicitamente.

#### Scenario: Canal expira antes da emissão

- **WHEN** o canal cai após a verificação de status e antes da emissão
- **THEN** a resposta é `409` e nenhum código é emitido

### Requirement: Validação do número

O serviço SHALL validar o número e MUST responder `422` para número
inválido sem distinguir existência na plataforma.

#### Scenario: Número inválido

- **WHEN** o cliente pede código para número malformado
- **THEN** a resposta é `422` genérica e nenhum código é emitido

### Requirement: Autorização do pareamento por telefone

A operação SHALL exigir dual auth e ownership e MUST responder `404` em
instância inexistente e `403` sem ownership.

#### Scenario: Código sem ownership

- **WHEN** credencial de outro dono pede código
- **THEN** a resposta é `403` e nenhum código é emitido
