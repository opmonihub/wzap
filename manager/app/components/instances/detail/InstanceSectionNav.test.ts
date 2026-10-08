import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  flushComponent, installComponentGlobals, loadComponent, mountComponent, primitive, unmountComponents
} from '../../../../tests/componentHarness'

beforeEach(installComponentGlobals)

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function sectionNav(section = 'overview') {
  const selections: string[] = []
  const component = Object.assign(loadComponent('components/instances/detail/InstanceSectionNav.vue'), {
    components: {
      UDashboardToolbar: primitive('UDashboardToolbar'),
      UNavigationMenu: primitive('UNavigationMenu')
    }
  })
  const view = await mountComponent(component, { section, onSelect: (value: string) => selections.push(value) })
  return { ...view, selections }
}

type SectionChoice = { value: string, label: string, active?: boolean, onSelect?: () => void }

describe('instance section navigation', () => {
  it('lets the mobile selector emit each of the seven sections once', async () => {
    const nav = await sectionNav()
    const selector = nav.findAll('USelect')[0]
    expect(selector).toBeDefined()
    const choices = selector!.props.items as SectionChoice[]
    expect(choices.map(({ value, label }) => ({ value, label }))).toEqual([
      { value: 'overview', label: 'Overview' },
      { value: 'messages', label: 'Messages' },
      { value: 'groups', label: 'Groups' },
      { value: 'channels', label: 'Channels' },
      { value: 'profile', label: 'Profile' },
      { value: 'integrations', label: 'Integrations' },
      { value: 'settings', label: 'Settings' }
    ])
    for (const choice of choices) {
      await nav.trigger(selector!, 'onUpdate:modelValue', choice.value)
    }
    expect(nav.selections).toEqual(['overview', 'messages', 'groups', 'channels', 'profile', 'integrations', 'settings'])
  })

  it('tracks externally changed current sections in both mobile and desktop controls', async () => {
    const nav = await sectionNav('integrations')
    expect(nav.findAll('USelect')[0]).toBeDefined()
    for (const current of ['integrations', 'settings', 'overview'] as const) {
      nav.props.section = current
      await flushComponent()
      const selector = nav.findAll('USelect')[0]!
      expect(selector.props.modelValue).toBe(current)
      const choices = selector.props.items as SectionChoice[]
      expect(choices.find(choice => choice.value === selector.props.modelValue)?.label).toBe({
        integrations: 'Integrations', settings: 'Settings', overview: 'Overview'
      }[current])
      const menu = nav.findAll('UNavigationMenu')[0]!
      expect(menu.props.modelValue).toBe(current)
      expect((menu.props.items as SectionChoice[]).filter(choice => choice.active).map(choice => choice.value)).toEqual([current])
    }
    expect(nav.selections).toEqual([])
  })

  it('keeps desktop selection callbacks connected to the same section event', async () => {
    const nav = await sectionNav()
    const menu = nav.findAll('UNavigationMenu')[0]!
    const choices = menu.props.items as SectionChoice[]
    choices.find(choice => choice.value === 'integrations')!.onSelect!()
    choices.find(choice => choice.value === 'settings')!.onSelect!()
    await flushComponent()
    expect(nav.selections).toEqual(['integrations', 'settings'])
  })
})
