## MODIFIED Requirements

### Requirement: Texto e anexos do atendente

Texto SHALL enfileirar com assinatura *nome:* quando is_sign_enabled, usando delimitador configurável e markdown convertido. Cada anexo SHALL ser baixado de data_url, armazenado e enfileirado como mídia: áudio não-PTT, demais pelo MIME, com extensões de desenho/documento forçando documento e caption acompanhando. Falha de envio SHALL gerar nota privada de erro na conversa. Sem correlação, quoted SHALL ser ignorado sem falhar. Saídas SHALL registrar o UUID do envio para acompanhar seu resultado; múltiplos anexos SHALL poder corresponder ao mesmo ID de mensagem Chatwoot sem violar isolamento da instância.

#### Scenario: Resposta com anexo

- **WHEN** o atendente envia texto e imagem
- **THEN** o WhatsApp recebe texto assinado e imagem com caption e o envio fica correlacionável pelo UUID da fila

#### Scenario: Reply do atendente

- **WHEN** a mensagem cita outra via in_reply_to conhecido
- **THEN** o envio cita a mensagem original no WhatsApp

#### Scenario: Vários anexos

- **WHEN** uma resposta Chatwoot contém vários anexos válidos
- **THEN** cada envio conserva seu UUID e todos podem apontar para o mesmo ID externo Chatwoot
