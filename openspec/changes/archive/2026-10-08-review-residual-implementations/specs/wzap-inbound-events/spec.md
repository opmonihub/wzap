## ADDED Requirements

### Requirement: Confirmação terminal com evento durável

Estado terminal de envio e seu evento de status SHALL ser persistidos atomicamente. Falha da persistência MUST NOT confirmar estado sem evento recuperável. Retry de persistência após retorno do envio SHALL manter o identificador do evento e MUST NOT repetir a chamada de envio WhatsApp; fan-out SHALL ocorrer somente após persistência concluída.

#### Scenario: Falha ao persistir evento após envio
- **WHEN** o envio retorna mas a transação de estado e evento falha
- **THEN** nenhum estado terminal isolado é confirmado e a tentativa de persistência conserva resultado e evento sem novo envio

#### Scenario: Falha definitiva
- **WHEN** uma falha definitiva é persistida
- **THEN** estado failed e evento correspondente aparecem em conjunto ou nenhum deles é confirmado
