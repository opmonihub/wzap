## REMOVED Requirements

### Requirement: Mídia via armazenamento local

**Reason**: arquivos persistentes passam ao MinIO, mantendo a mesma capacidade de espelhamento.
**Migration**: o espelho lê a mídia pelo armazenamento de objetos usando a referência existente do evento; adaptadores locais temporários são detalhes de execução.

## ADDED Requirements

### Requirement: Mídia via armazenamento de objetos

Mídia inbound SHALL ser enviada ao Chatwoot a partir dos bytes do objeto identificado pela referência do evento. media_omitted SHALL repassar o motivo sem impedir o espelho do texto.

#### Scenario: Mídia espelhada

- **WHEN** uma mensagem com mídia disponível é espelhada
- **THEN** o anexo aparece na conversa com o mesmo conteúdo

#### Scenario: Mídia omitida

- **WHEN** o evento marca media_omitted
- **THEN** só o texto é espelhado sem falhar por ausência do anexo

### Requirement: Correlações sem dependência de histórico geral

Correlação de entrada e edição SHALL conservar a chave WhatsApp e o ID externo do Chatwoot sem exigir mensagem na fila. As chaves de edição MUST NOT ser confundidas com identificadores reais de envio.

#### Scenario: Edição recebida

- **WHEN** uma edição é espelhada com uma chave composta de deduplicação
- **THEN** sua correlação permanece distinta do original sem criar mensagem artificial na fila
