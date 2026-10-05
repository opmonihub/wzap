import assert from 'node:assert/strict'
import test from 'node:test'
import { instanceNameErrorKey, instanceNameUpdate, isValidInstanceName } from '../app/utils/instanceName.ts'

test('accepts exact ASCII names, case variants and the 64-character boundary', () => {
  for (const name of ['A', '0', 'Loja_SP-1', 'loja_sp-1', 'Stats', 'STATS', 'a'.repeat(64)]) {
    assert.equal(isValidInstanceName(name), true, name)
  }
})

test('rejects whitespace, non-ASCII, punctuation and names outside the bounds', () => {
  for (const name of ['', ' a', 'a ', 'a\n', 'a b', 'café', '店', 'a.b', 'a/b', '-a', 'a_', 'a'.repeat(65)]) {
    assert.equal(isValidInstanceName(name), false, JSON.stringify(name))
  }
})

test('reserves stats and all UUID forms accepted as identities', () => {
  for (const name of [
    'stats',
    '550e8400-e29b-41d4-a716-446655440000',
    '550E8400-E29B-41D4-A716-446655440000',
    '550e8400e29b41d4a716446655440000',
    '00000000000000000000000000000000',
    'urn:uuid:550e8400-e29b-41d4-a716-446655440000',
    '{550e8400-e29b-41d4-a716-446655440000}'
  ]) {
    assert.equal(isValidInstanceName(name), false, name)
  }
  assert.equal(isValidInstanceName('550g8400e29b41d4a716446655440000'), true)
})

test('reserves canonical UUIDs wrapped in arbitrary characters accepted by Go UUID parsing', () => {
  assert.equal(isValidInstanceName('x550e8400-e29b-41d4-a716-446655440000y'), false)
  assert.equal(isValidInstanceName('0550E8400-E29B-41D4-A716-4466554400009'), false)
  assert.equal(isValidInstanceName('x550g8400-e29b-41d4-a716-446655440000y'), true)
})

test('keeps unchanged legacy names and omits them from updates', () => {
  for (const name of ['', '  Legacy café  ', 'stats', '550e8400e29b41d4a716446655440000', 'a'.repeat(300)]) {
    assert.equal(isValidInstanceName(name, name), true)
    assert.deepEqual(instanceNameUpdate(name, name), {})
  }
  assert.deepEqual(instanceNameUpdate('Loja_SP-1', 'Loja_SP-1'), {})
})

test('renames preserve exact input and validate changes rather than trimmed equivalents', () => {
  assert.equal(isValidInstanceName('legacy', ' legacy '), true)
  assert.deepEqual(instanceNameUpdate('legacy', ' legacy '), { name: 'legacy' })
  assert.deepEqual(instanceNameUpdate('Loja', 'loja'), { name: 'Loja' })
  for (const name of [' legacy ', '', 'stats', 'a'.repeat(65)]) {
    assert.equal(isValidInstanceName(name, 'legacy'), false)
    assert.throws(() => instanceNameUpdate(name, 'legacy'), RangeError)
  }
})

test('name validation and conflicts use name messages without swallowing other errors', () => {
  assert.equal(instanceNameErrorKey({ status: 422, code: 'invalid_instance_name' }), 'instances.fields.nameHint')
  assert.equal(instanceNameErrorKey({ status: 409, code: 'instance_name_taken' }), 'instances.fields.nameTaken')
  assert.equal(instanceNameErrorKey({ status: 409, code: 'instance_name_ambiguous' }), 'instances.fields.nameAmbiguous')
  assert.equal(instanceNameErrorKey({ status: 409, code: 'conflict' }), undefined)
  assert.equal(instanceNameErrorKey({ status: 500, code: 'instance_name_taken' }), undefined)
})
