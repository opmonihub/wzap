import { describe, expect, it } from 'vitest'
import { parseQuota } from './accountQuota'

describe('quota input parsing', () => {
  it.each([
    [undefined, undefined],
    ['', undefined],
    ['   ', undefined],
    [0, 0],
    ['0', 0],
    [2, 2],
    ['2', 2],
    [' 3 ', 3]
  ])('normalizes %j to %j while keeping the default distinct from unlimited', (input, expected) => {
    expect(parseQuota(input)).toBe(expected)
  })

  it.each([-1, '-1', 1.5, '1.5', NaN, 'NaN', Infinity, 'Infinity', -Infinity, 'quota'])('rejects %j', (input) => {
    expect(parseQuota(input)).toBeNull()
  })
})
