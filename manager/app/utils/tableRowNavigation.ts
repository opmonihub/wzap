// Resolve pointer navigation separately from the table's native controls.
export function tableRowNavigation<T>(
  event: MouseEvent,
  table: HTMLTableElement | null | undefined,
  displayedRows: readonly T[]
): T | undefined {
  if (!table || event.defaultPrevented || event.button !== 0) return

  const target = event.target as Element | null
  if (!target?.closest || target.closest('a, button, input, select, textarea, label, [role="checkbox"], [role="menuitem"], [role="menuitemcheckbox"], [contenteditable="true"]')) return

  const row = target.closest('tr')
  const body = table.tBodies[0]
  if (!row || !body || row.parentElement !== body || target.closest('table') !== table) return

  return displayedRows[Array.from(body.rows).indexOf(row)]
}
