## ADDED Requirements

### Requirement: Recibos isolados pela instância de origem

Atualização de recibo SHALL usar a instância de origem junto do identificador WhatsApp. Mensagens de outra instância MUST NOT receber delivered_at ou read_at mesmo que possuam o mesmo identificador WhatsApp; somente IDs correspondentes ao alvo SHALL compor seu evento.

#### Scenario: Identificador WhatsApp repetido
- **WHEN** duas instâncias possuem mensagens com o mesmo identificador e uma recebe recibo
- **THEN** somente a mensagem da instância de origem é atualizada e o evento pertence àquela instância
