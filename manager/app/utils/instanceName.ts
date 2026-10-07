const namePattern = /^[A-Za-z0-9](?:[A-Za-z0-9_-]{0,62}[A-Za-z0-9])?$/
const uuidPattern = /^(?:[0-9a-f]{32}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/i

export function isValidInstanceName(name: string, originalName?: string): boolean {
  if (name === originalName) {
    return true
  }
  // google/uuid.Parse also accepts a canonical UUID wrapped in any two
  // characters, not only braces. URNs are already excluded by the grammar.
  const uuidCandidate = name.length === 38 ? name.slice(1, -1) : name
  // Compare the complete match: JavaScript's $ can also match before a
  // trailing newline. Names are exact strings, with no whitespace cleanup.
  return namePattern.exec(name)?.[0] === name && name !== 'stats' && !uuidPattern.test(uuidCandidate)
}

export function instanceNameUpdate(name: string, originalName: string): { name?: string } {
  if (name === originalName) {
    return {}
  }
  if (!isValidInstanceName(name)) {
    throw new RangeError('Invalid instance name')
  }
  return { name }
}

export function instanceNameErrorKey(error: { status: number, code: string }): string | undefined {
  if (error.status === 422 && error.code === 'invalid_instance_name') {
    return 'instances.fields.nameHint'
  }
  if (error.status === 409 && error.code === 'instance_name_taken') {
    return 'instances.fields.nameTaken'
  }
  return undefined
}
