# wzap-chatwoot-inbound Specification

## Purpose

Entrada do Chatwoot para o WhatsApp: resposta do atendente vira mensagem enfileirada no wzap (com retry e status), mais sync reverso e comandos operacionais via conversa operacional.

## Requirements

### Requirement: Webhook aberto por instância

`POST /chatwoot/webhook/{id}` SHALL ser público (sem credencial — fiel ao Evolution, risco documentado, secret é v2) e responder `200` com corpo de bot mesmo quando nada há a fazer. Evento sem `conversation`, `private`, `message_updated` sem delete e eco `WAID:` MUST ser descartado sem efeito.

#### Scenario: Eco descartado

- **WHEN** chega evento cujo `source_id` começa com `WAID:`
- **THEN** responde `200` sem enfileirar nada (anti-loop)

#### Scenario: Nota privada

- **WHEN** chega mensagem `private`
- **THEN** responde `200` sem enviar ao WhatsApp

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

### Requirement: Sync reverso e leitura

Delete com `deleted` SHALL apagar no WhatsApp quando habilitado; template SHALL enviar texto direto sem assinatura; com `MESSAGE_READ`, o envio do atendente SHALL marcar a última recebida como lida.

#### Scenario: Apagar dos dois lados

- **WHEN** o atendente apaga mensagem com a flag ligada
- **THEN** a mensagem some no WhatsApp

### Requirement: Comandos da conversa operacional

Na conversa operacional, `init[:number]` SHALL parear (por código com número, ou QR), `status` SHALL informar estado, `clearcache` SHALL limpar caches do conector, `disconnect` SHALL desconectar a sessão; cada um SHALL confirmar na conversa.

#### Scenario: Comando status

- **WHEN** o operador envia `status` na conversa operacional
- **THEN** o estado atual da instância é respondido lá

### Requirement: Rota autenticada de comandos operacionais

`POST /instances/{id}/chatwoot/command` SHALL executar comandos operacionais atrás da auth dual (global ou por instância, com ownership), aceitando `{"command":"status|init[:number]|clearcache|disconnect","conversation_id":N}` e respondendo `200` com `{"ok":true}` mais confirmação na conversa. O conector desabilitado SHALL responder `400`; instância sem configuração SHALL responder `404`. O webhook aberto `POST /chatwoot/webhook/{id}` SHALL nunca executar comandos (descarta `200` sem efeito).

#### Scenario: Comando via rota autenticada

- **WHEN** cliente autenticado envia `{"command":"status","conversation_id":7}`
- **THEN** responde `200` com `{"ok":true}` e confirma na conversa 7

### Requirement: Exceção privada de anexos sem sobreposição parcial de DNS

Downloads SHALL admitir destinos privados somente quando a identidade completa do destino é confiável como host configurado do Chatwoot. Host diferente com apenas um IP coincidente MUST NOT liberar os demais endereços privados; dial e redirect SHALL aplicar a mesma política.

#### Scenario: DNS misto de host alheio
- **WHEN** um host de anexo diferente resolve para loopback e um IP público que coincide com o Chatwoot
- **THEN** o download é recusado sem conectar ao loopback

#### Scenario: Host privado configurado
- **WHEN** o anexo usa legitimamente o host privado configurado do Chatwoot
- **THEN** a exceção prevista permite o download dentro da política existente

### Requirement: Comando init usa lifecycle da instância fresca

init e init:number SHALL iniciar/criar sessão quando a instância persistida ainda não tiver sessão, reutilizando o fluxo existente de conexão/pareamento. Confirmação status MUST NOT publicar device_jid.

#### Scenario: Instância sem sessão
- **WHEN** init é solicitado para instância recém-criada sem sessão
- **THEN** o lifecycle inicia a sessão e permite o pareamento

#### Scenario: Pareamento por número
- **WHEN** init:number é solicitado sem sessão existente
- **THEN** a sessão é iniciada antes de solicitar o código

#### Scenario: Status sem JID interno
- **WHEN** status é confirmado no Chatwoot
- **THEN** informa nome e estado sem ecoar device_jid
