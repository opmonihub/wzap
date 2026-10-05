# Decisões do pedido

Em 2026-10-05, o usuário marcou os campos `apikey`, `X-Request-Id`, `limit` e `cursor` de `GET /instances` no Swagger e pediu sua remoção, incluindo a paginação no backend. A chave deve ser informada uma única vez no Authorize da página.

Requisitos definidos: remover parâmetros de entrada duplicados de autenticação e correlação em todas as operações documentadas; listar todas as instâncias autorizadas em uma chamada. Manter o envelope e `data.items`, retirar `next_cursor`, conservar autenticação e o request ID automático. A paginação visual local das tabelas do Manager e as outras coleções permanecem fora do pedido.
