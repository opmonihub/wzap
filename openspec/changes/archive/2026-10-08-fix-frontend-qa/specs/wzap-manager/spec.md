## ADDED Requirements

### Requirement: Usable form validation

The manager SHALL accept non-negative whole numeric quotas, preserve an explicitly entered zero as unlimited, omit a blank creation quota, and reject invalid quota values before a write request; login validation SHALL describe missing fields without exposing internal validator types.

#### Scenario: Numeric input creation and editing
- **WHEN** an administrator enters 0 or a positive whole quota using the numeric input and submits an otherwise valid account form
- **THEN** the form sends that exact numeric quota and completes successfully

#### Scenario: Empty and invalid quota
- **WHEN** a creation quota is blank or any quota is negative or fractional
- **THEN** a blank creation quota is omitted and an invalid value prevents a write with a readable field error

#### Scenario: Empty sign-in
- **WHEN** a visitor submits an untouched sign-in form
- **THEN** the email and password fields show readable instructions and no login request is sent

### Requirement: Correct accessible control targets

The manager SHALL provide distinct label targets for individual webhook events and accessible names describing icon navigation, file picker and selection controls.

#### Scenario: Webhook event label
- **WHEN** the operator clicks the visible label for a webhook event
- **THEN** only that event's selection changes and its accessible name identifies that event

#### Scenario: Named controls
- **WHEN** assistive technology encounters the instance back action, a photo or message file picker, or the confirmed unnamed selects
- **THEN** it announces the action or field name in the manager's locale

#### Scenario: Selected file preview
- **WHEN** the operator selects an image in a photo or message file picker
- **THEN** the thumbnail has valid alternative text or decorative image semantics, and file name and removal remain available

### Requirement: Legible theme and responsive instance navigation

The manager SHALL keep normal semantic text at a contrast ratio of at least 4.5:1 against its rendered background and keep every instance section label readable and reachable at 320px and wider without page-level horizontal overflow.

#### Scenario: Light semantic colors
- **WHEN** a visitor uses primary actions, active navigation or semantic feedback in light mode
- **THEN** the text meets the contrast requirement including hover states

#### Scenario: Narrow instance navigation
- **WHEN** the instance detail is opened on a narrow viewport
- **THEN** the seven sections remain readable and can be selected, with the current section visible

### Requirement: Available column controls

The manager SHALL show a column-display control only when the current table has optional columns; required instance identity and status columns SHALL remain visible.

#### Scenario: Required instance columns
- **WHEN** the instance table contains only required columns
- **THEN** no empty column-display menu is offered

#### Scenario: Optional account columns
- **WHEN** the account table has optional columns
- **THEN** its display menu allows toggling those columns while retaining row selection and actions

### Requirement: Accessible table row actions

Instance tables SHALL preserve native row semantics so the row's links, selection checkbox and action buttons remain individually accessible; pointer navigation and checkbox keyboard selection SHALL retain the displayed instance identity after filtering and sorting.

#### Scenario: Nested row controls
- **WHEN** an operator traverses an instance table row using assistive technology
- **THEN** its name link, checkbox and actions keep their own accessible roles and names without a containing button role

#### Scenario: Displayed row identity
- **WHEN** an operator filters or sorts the table and opens the displayed instance
- **THEN** navigation targets that displayed instance and checkbox selection remains independent

## MODIFIED Requirements

### Requirement: Listas e detalhe no padrão do template

As listas SHALL manter dados, filtros, ordenação, seleção e paginação atuais adotando o `ui` de tabela do template customers (bordas arredondadas, header com fundo) e footer com contagem de selecionados + paginação; o detalhe SHALL adotar header + faixa de stats + navegação por seções via `UNavigationMenu` horizontal no toolbar em desktop (padrão `settings.vue`) e seletor nomeado e legível no mobile, com as mesmas 7 seções (overview, messages, groups, channels, profile, integrations, settings), mantendo os fluxos atuais de pairing, nome, key, webhook, envio e danger; a seção messages SHALL usar o split inbox full-width (lista selecionável + painel de detalhe + composer no rodapé, `USlideover` no mobile) e as demais seções SHALL centralizar forms em `lg:max-w-2xl`.

#### Scenario: Lista preservada

- **WHEN** a conta usa busca, filtro de status, ordenação, seleção ou paginação
- **THEN** o comportamento é o atual, só o visual segue o template

#### Scenario: Detalhe preservado

- **WHEN** a conta abre o detalhe
- **THEN** vê pairing, nome, key, webhook, test-send, messages e danger com os mesmos fluxos, reorganizados em header + stats + 7 seções (overview, messages, groups, channels, profile, integrations, settings)

#### Scenario: Messages em split inbox

- **WHEN** a conta abre a seção messages no desktop
- **THEN** vê a lista selecionável à esquerda e o detalhe com composer à direita; no mobile o detalhe abre em `USlideover`
