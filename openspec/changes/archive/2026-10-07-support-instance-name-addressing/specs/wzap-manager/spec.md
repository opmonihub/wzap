## ADDED Requirements

### Requirement: Instance name form validation

Create, edit and overview forms SHALL explain and validate the same name grammar and reserved names as the API. Editing SHALL permit an exactly unchanged legacy name, without trimming or silently renaming it. Name-conflict errors SHALL be distinguished from external-reference conflicts.

#### Scenario: Invalid new name
- **WHEN** a user enters spaces, accents, invalid punctuation, an overlong value or a reserved name
- **THEN** the form explains the rule and prevents submission

#### Scenario: Legacy name unchanged
- **WHEN** a user edits another field while retaining the original legacy name
- **THEN** the unchanged name is omitted or retained exactly and the unrelated update remains possible

#### Scenario: Name already occupied
- **WHEN** the API returns instance_name_taken
- **THEN** the form reports the instance-name conflict instead of an external-reference conflict
