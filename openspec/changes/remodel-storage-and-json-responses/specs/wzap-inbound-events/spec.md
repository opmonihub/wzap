## ADDED Requirements

### Requirement: Outbox somente de eventos pendentes

Eventos persistidos ainda não confirmados SHALL permanecer recuperáveis após reinício ou indisponibilidade do broker. Após confirmação de publicação pelo JetStream, seu registro no outbox SHALL ser removido; retenção e replay dos eventos publicados SHALL permanecer no JetStream. Retentativas MUST conservar o event_id e a entrega permanece at-least-once.

#### Scenario: Broker indisponível

- **WHEN** a publicação de um evento persistido falha porque o broker está indisponível
- **THEN** o evento permanece no outbox e é publicado com o mesmo ID quando o broker retorna

#### Scenario: Publicação confirmada

- **WHEN** o JetStream confirma que recebeu o evento
- **THEN** o registro é removido do outbox e o evento segue disponível no stream durante sua retenção

#### Scenario: Confirmação seguida de falha de exclusão

- **WHEN** a publicação é confirmada mas o registro SQL não pode ser removido
- **THEN** uma retentativa mantém o mesmo ID para permitir deduplicação sem prometer exactly-once
