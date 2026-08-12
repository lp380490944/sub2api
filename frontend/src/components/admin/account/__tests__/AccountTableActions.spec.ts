import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountTableActions from '../AccountTableActions.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountTableActions', () => {
  it('hides the create-account button when hideCreate is true (readonly admin)', () => {
    const wrapper = mount(AccountTableActions, {
      props: { loading: false, hideCreate: true },
      global: { stubs: { Icon: true } }
    })

    const createButton = wrapper
      .findAll('button')
      .find((btn) => btn.text() === 'admin.accounts.createAccount')
    expect(createButton).toBeUndefined()

    // Refresh stays available -- it's a read-only re-fetch, not a write.
    expect(wrapper.emitted()).not.toHaveProperty('create')
  })

  it('shows the create-account button when hideCreate is false/unset (real admin)', () => {
    const wrapper = mount(AccountTableActions, {
      props: { loading: false },
      global: { stubs: { Icon: true } }
    })

    const createButton = wrapper
      .findAll('button')
      .find((btn) => btn.text() === 'admin.accounts.createAccount')
    expect(createButton).toBeDefined()
  })

  it('emits create when the button is clicked for a real admin', async () => {
    const wrapper = mount(AccountTableActions, {
      props: { loading: false, hideCreate: false },
      global: { stubs: { Icon: true } }
    })

    const createButton = wrapper
      .findAll('button')
      .find((btn) => btn.text() === 'admin.accounts.createAccount')
    await createButton?.trigger('click')

    expect(wrapper.emitted('create')).toHaveLength(1)
  })
})
