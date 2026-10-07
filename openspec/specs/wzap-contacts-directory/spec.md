# wzap-contacts-directory Specification

## Purpose
TBD - created by archiving change whatsmeow-parity-routes. Update Purpose after archive.

## Requirements

### Requirement: Consulta em lote

O serviço SHALL resolver até 50 telefones por chamada (`phones[]`),
devolvendo por item `jid`, `is_on_whatsapp` e presença de último visto
best-effort, e MUST responder `422` para lote vazio ou acima do limite e
`409` quando desconectada.

#### Scenario: Lote resolvido

- **WHEN** o cliente envia lote válido em instância conectada
- **THEN** cada item volta com `jid` e o indicador de registro

#### Scenario: Lote acima do limite

- **WHEN** o cliente envia 51 telefones
- **THEN** a resposta é `422` e nada é resolvido

### Requirement: Dispositivos, foto e perfil business

O serviço SHALL consultar dispositivos do contato, foto de perfil alheia
(URL + versão, vazia quando ausente) e perfil business (nome, descrição,
vazios quando não-business), e MUST responder `404` para contato
desconhecido no upstream e `422` para JID malformado.

#### Scenario: Foto alheia consultada

- **WHEN** o cliente consulta a foto de contato existente
- **THEN** a URL é devolvida (ou vazia quando o contato não tem foto)

#### Scenario: JID malformado

- **WHEN** o cliente informa JID inválido
- **THEN** a resposta é `422`

### Requirement: Link próprio de contato

O serviço SHALL emitir o link QR próprio de contato e permitir revogá-lo
(`revoke=false|true`); revogar SHALL invalidar o link anterior.

#### Scenario: Link revogado

- **WHEN** o cliente revoga o link próprio
- **THEN** um link novo é devolvido e o antigo deixa de resolver

### Requirement: Autorização do diretório

As operações SHALL exigir dual auth e ownership e MUST responder `404` em
instância inexistente e `403` sem ownership.

#### Scenario: Diretório sem ownership

- **WHEN** credencial de outro dono consulta dispositivos
- **THEN** a resposta é `403`
