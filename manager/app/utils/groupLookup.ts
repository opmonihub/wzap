/** Trimmed group JID for lookup, or null when the field is empty or whitespace-only. */
export function groupLookupJIDOrNull(raw: string): string | null {
  const trimmed = raw.trim()
  return trimmed === '' ? null : trimmed
}
