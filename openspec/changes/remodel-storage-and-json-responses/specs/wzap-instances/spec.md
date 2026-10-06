## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Vínculo exclusivo de dispositivo

Reconexão e restauração SHALL carregar as credenciais do dispositivo vinculado exclusivamente à instância. Identidade, conexão e webhook SHALL ter estados persistidos separados sem cópias conflitantes; editar identidade MUST NOT reverter uma transição de conexão concorrente.

#### Scenario: Restaurar sessão vinculada

- **WHEN** o serviço reinicia com credenciais válidas para um dispositivo vinculado
- **THEN** restaura esse dispositivo sem parear outro nem reutilizar o vínculo de outra instância

#### Scenario: Dispositivo legado divergente

- **WHEN** dois campos legados de uma instância indicam dispositivos incompatíveis
- **THEN** a migração preserva os dados e exige tratamento explícito antes do corte
