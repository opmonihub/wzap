// Empty stays omitted so the server default applies; 0 is a valid explicit
// value meaning unlimited.
export function parseQuota(raw: string | number | undefined): number | undefined | null {
  const value = typeof raw === 'string' ? raw.trim() : raw
  if (value === undefined || value === '') {
    return undefined
  }
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < 0) {
    return null
  }
  return parsed
}
