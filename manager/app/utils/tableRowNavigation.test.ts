import { describe, expect, it } from 'vitest'
import { tableRowNavigation } from './tableRowNavigation'

// Only native DOM lookup boundaries are supplied: the resolver itself chooses
// whether and which displayed row can navigate. Browser QA checks selectors.
function tableFixture() {
  const body = { rows: [] as object[] }
  const first = { parentElement: body }
  const second = { parentElement: body }
  body.rows = [first, second]
  const table = { tBodies: [body] } as unknown as HTMLTableElement

  function click(row: object | null, options: {
    interactive?: boolean
    owner?: HTMLTableElement
    defaultPrevented?: boolean
    button?: number
  } = {}): MouseEvent {
    return {
      button: options.button ?? 0,
      defaultPrevented: options.defaultPrevented ?? false,
      target: {
        closest: (selector: string) => {
          if (selector === 'tr') return row
          if (selector === 'table') return options.owner ?? table
          return options.interactive ? {} : null
        }
      }
    } as unknown as MouseEvent
  }

  return { table, body, first, second, click }
}

describe('table row pointer navigation', () => {
  it('resolves the clicked row against the displayed order after sorting', () => {
    const { table, second, click } = tableFixture()
    expect(tableRowNavigation(click(second), table, [{ id: 'zulu' }, { id: 'alpha' }])).toEqual({ id: 'alpha' })
  })

  it('uses the current filtered page rather than an index into all instances', () => {
    const { table, first, click } = tableFixture()
    expect(tableRowNavigation(click(first), table, [{ id: 'page-two-filtered-match' }])).toEqual({ id: 'page-two-filtered-match' })
  })

  it('leaves interactive descendants to their own controls', () => {
    const { table, first, click } = tableFixture()
    expect(tableRowNavigation(click(first, { interactive: true }), table, [{ id: 'alpha' }])).toBeUndefined()
  })

  it('ignores header rows and elements outside rows', () => {
    const { table, click } = tableFixture()
    expect(tableRowNavigation(click({ parentElement: {} }), table, [{ id: 'alpha' }])).toBeUndefined()
    expect(tableRowNavigation(click(null), table, [{ id: 'alpha' }])).toBeUndefined()
  })

  it('does not resolve an empty-state body row to an instance', () => {
    const { table, first, click } = tableFixture()
    expect(tableRowNavigation(click(first), table, [])).toBeUndefined()
  })

  it('rejects nested or foreign tables even when the event reaches the root', () => {
    const { table, first, click } = tableFixture()
    const foreignTable = {} as HTMLTableElement
    expect(tableRowNavigation(click(first, { owner: foreignTable }), table, [{ id: 'alpha' }])).toBeUndefined()
  })

  it('ignores prevented and non-primary clicks', () => {
    const { table, first, click } = tableFixture()
    expect(tableRowNavigation(click(first, { defaultPrevented: true }), table, [{ id: 'alpha' }])).toBeUndefined()
    expect(tableRowNavigation(click(first, { button: 1 }), table, [{ id: 'alpha' }])).toBeUndefined()
  })
})
