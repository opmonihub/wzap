## ADDED Requirements

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
