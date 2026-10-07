# wzap-instances Specification

## Purpose

Gerenciar o ciclo de vida das instâncias de WhatsApp (pareamento, estado de conexão, reconexão e remoção) para que o backend opere múltiplas contas isoladas sem lidar com o protocolo.

## Requirements

### Requirement: Criação de instância

**BREAKING**: o serviço SHALL permitir criar uma instância com nome e referência externa opcional, retornando data.instance com identificador, connection.status inicial disconnected, webhook e a instance key em claro uma única vez. O dono é a conta da sessão criadora; na criação pela key global sem sessão, o dono é a conta admin mais antiga, podendo ser sobrescrito pelo parâmetro opcional de dono somente por global/admin. A referência externa, quando informada, MUST ser única. Dono e referência MUST continuar persistidos, mas MUST NOT ser devolvidos na representação pública. Criação acima da cota MUST responder 403 quota_exceeded; somente key global, sessão admin ou sessão user dentro da cota SHALL criar instâncias.

#### Scenario: Criação bem-sucedida

- **WHEN** o cliente autorizado envia nome e referência externa inexistente
- **THEN** recebe 201 com data.instance, conexão disconnected e a key de criação, sem exposição do dono ou referência externa

#### Scenario: Referência externa duplicada

- **WHEN** o cliente autorizado tenta criar uma instância com referência externa já usada
- **THEN** recebe 409 e nenhuma instância é criada

#### Scenario: Criação acima da cota

- **WHEN** um usuário sem saldo de cota tenta criar uma instância
- **THEN** recebe 403 quota_exceeded e nada é criado

#### Scenario: Criação pela key global sem sessão

- **WHEN** a key global cria uma instância sem informar dono
- **THEN** o dono persistido é a conta admin mais antiga, sem aparecer na resposta pública

#### Scenario: Criação com key de instância

- **WHEN** o cliente usa uma instance key para criar instância
- **THEN** recebe 403, pois a key só opera a própria instância

### Requirement: Pareamento por QR

O serviço SHALL iniciar o pareamento de uma instância não conectada, retornando um QR code e sua validade, e MUST transicionar o estado para `pairing`. Após a leitura do QR, o estado MUST virar `connected` com o identificador público da conta registrado.

#### Scenario: Início de pareamento

- **WHEN** o cliente solicita conectar uma instância `disconnected`
- **THEN** o serviço responde com o QR e a expiração, e o estado passa a `pairing`

#### Scenario: QR expirado

- **WHEN** o QR expira sem leitura e o cliente consulta o estado de pareamento
- **THEN** o serviço fornece um novo QR válido

#### Scenario: Pareamento concluído

- **WHEN** o usuário lê o QR no aplicativo
- **THEN** o estado passa a `connected`, o identificador público é registrado e um evento de conexão é publicado

### Requirement: Pareamento por código de telefone

Como alternativa ao QR, o serviço SHALL emitir código de 8 dígitos via
`POST /instances/{id}/pair-phone` quando houver canal de pareamento aberto
(`connect` prévio), sem mudar `connect`/`qr`/`status`/`disconnect` (ver
`wzap-phone-pairing`).

#### Scenario: Código emitido com canal aberto

- **WHEN** o cliente pede código para número válido com canal aberto
- **THEN** a resposta traz `pairing_code` e expiração alinhada ao canal

### Requirement: Instância já conectada

O serviço SHALL ser idempotente ao receber pedido de conexão para instância já logada, sem gerar novo pareamento.

#### Scenario: Conectar instância conectada

- **WHEN** o cliente solicita conectar uma instância `connected`
- **THEN** o serviço responde `connected` sem emitir QR

### Requirement: Reconexão automática

Em queda transitória, o serviço SHALL tentar reconectar com espera crescente e MUST tornar o estado observável durante as tentativas.

#### Scenario: Queda transitória

- **WHEN** a conexão cai por falha de rede temporária
- **THEN** o estado reflete a tentativa de reconexão e volta a `connected` quando restabelecida

### Requirement: Desconexão definitiva

O serviço SHALL encerrar a sessão ao receber pedido de desconexão, remover as credenciais da instância e MUST NOT reconectar automaticamente depois disso.

#### Scenario: Desconectar instância

- **WHEN** o cliente solicita desconectar uma instância conectada
- **THEN** o estado vira `disconnected`, as credenciais são removidas e nenhuma reconexão é tentada

### Requirement: Restrição da conta

Quando a conta for restringida pela plataforma WhatsApp, o serviço SHALL marcar a instância com estado `error`, registrar o motivo e MUST NOT tentar reconexão automática.

#### Scenario: Restrição detectada

- **WHEN** a plataforma informa restrição ou banimento da conta
- **THEN** o estado vira `error` com motivo registrado e não há reconexão automática

### Requirement: Remoção de instância

O serviço SHALL permitir remover uma instância, encerrando a sessão e eliminando dados associados; operações posteriores sobre a instância MUST responder `404`.

#### Scenario: Remoção concluída

- **WHEN** o cliente remove uma instância existente
- **THEN** sessão, mensagens e mídias associadas são eliminadas e consultas seguintes respondem `404`

### Requirement: Restauração no boot

O serviço SHALL restaurar automaticamente as sessões registradas ao iniciar, com concorrência limitada, e MUST refletir o resultado de cada restauração no estado da instância.

#### Scenario: Reinício do serviço

- **WHEN** o serviço reinicia com sessões persistidas
- **THEN** as instâncias voltam a `connected` sem novo pareamento

### Requirement: Vínculo exclusivo de dispositivo

Reconexão e restauração SHALL carregar as credenciais do dispositivo vinculado exclusivamente à instância. Identidade, conexão e webhook SHALL ter estados persistidos separados sem cópias conflitantes; editar identidade MUST NOT reverter uma transição de conexão concorrente.

#### Scenario: Restaurar sessão vinculada

- **WHEN** o serviço reinicia com credenciais válidas para um dispositivo vinculado
- **THEN** restaura esse dispositivo sem parear outro nem reutilizar o vínculo de outra instância

#### Scenario: Dispositivo legado divergente

- **WHEN** dois campos legados de uma instância indicam dispositivos incompatíveis
- **THEN** a migração preserva os dados e exige tratamento explícito antes do corte
