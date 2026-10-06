import { describe, expect, it } from 'vitest'

import { groupLookupJIDOrNull } from './groupLookup'

describe('groupLookupJIDOrNull', () => {
  it('rejects empty and whitespace-only input', () => {
    expect(groupLookupJIDOrNull('')).toBeNull()
    expect(groupLookupJIDOrNull('   ')).toBeNull()
    expect(groupLookupJIDOrNull('\n\t')).toBeNull()
  })

  it('returns a trimmed JID', () => {
    expect(groupLookupJIDOrNull(' 120363000000000000@g.us ')).toBe('120363000000000000@g.us')
  })
})
