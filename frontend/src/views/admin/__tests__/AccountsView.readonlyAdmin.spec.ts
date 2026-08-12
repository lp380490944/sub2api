import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'
import AccountTableActions from '@/components/admin/account/AccountTableActions.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  authStoreMock
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  authStoreMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      probeUpstreamBillingBatch: vi.fn(),
      toggleSchedulable: vi.fn(),
      getTempUnschedulableStatus: vi.fn().mockResolvedValue({
        active: true,
        state: {
          rule_index: 0,
          triggered_at_unix: Math.floor(Date.now() / 1000) - 60,
          until_unix: Math.floor(Date.now() / 1000) + 3600,
          error_message: 'test error'
        }
      }),
      recoverState: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStoreMock()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

// Extends the DataTable stub used by other AccountsView specs to also expose
// the "actions" cell slot (edit/delete/more buttons), which this spec needs
// to inspect for readonly-admin visibility.
const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <span v-for="column in columns" :key="column.key" data-test="column-key">{{ column.key }}</span>
      <div v-for="row in data" :key="row.id">
        <div data-test="select-row"><slot name="cell-select" :row="row" /></div>
        <slot name="cell-created_at" :value="row.created_at" :row="row" />
        <div data-test="actions-row"><slot name="cell-actions" :row="row" /></div>
      </div>
    </div>
  `
}

// Mirrors AccountStatusIndicator's real event contract (@show-temp-unsched)
// without pulling in its own status-badge logic, so the test can open
// TempUnschedStatusModal the same way the real component does.
const AccountStatusIndicatorStub = {
  props: ['account'],
  emits: ['show-temp-unsched'],
  template: `<button data-test="open-temp-unsched" @click="$emit('show-temp-unsched', account)">status</button>`
}

const baseStubs = {
  AppLayout: { template: '<div><slot /></div>' },
  TablePageLayout: {
    template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
  },
  DataTable: DataTableStub,
  Pagination: true,
  ConfirmDialog: true,
  // AccountTableActions is left as the real component (not stubbed) so the
  // create-account trigger's v-if actually renders/hides.
  AccountTableActions,
  AccountTableFilters: { template: '<div></div>' },
  AccountBulkActionsBar: true,
  AccountActionMenu: true,
  ImportDataModal: true,
  ReAuthAccountModal: true,
  AccountTestModal: true,
  AccountStatsModal: true,
  ScheduledTestsPanel: true,
  SyncFromCrsModal: true,
  TempUnschedStatusModal: true,
  ErrorPassthroughRulesModal: true,
  TLSFingerprintProfilesModal: true,
  CreateAccountModal: true,
  EditAccountModal: true,
  BulkEditAccountModal: true,
  PlatformTypeBadge: true,
  AccountCapacityCell: true,
  AccountStatusIndicator: true,
  AccountTodayStatsCell: true,
  AccountGroupsCell: true,
  AccountUsageCell: true,
  Icon: true
}

describe('admin AccountsView readonly admin controls', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getUpstreamBillingProbeSettings.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    authStoreMock.mockReset()

    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'test-account',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z'
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('hides write-action triggers for a readonly admin', async () => {
    authStoreMock.mockReturnValue({
      isReadonlyAdmin: true,
      isSimpleMode: false,
      token: 'test-token'
    })

    const wrapper = mount(AccountsView, {
      global: {
        stubs: baseStubs
      }
    })

    await flushPromises()

    // create-account trigger (AccountTableActions' own button) is gone
    const createButton = wrapper
      .findAll('button')
      .find(btn => btn.text() === 'admin.accounts.createAccount')
    expect(createButton).toBeUndefined()

    // per-row edit/delete triggers are gone
    const actionsRow = wrapper.get('[data-test="actions-row"]')
    const editButton = actionsRow.findAll('button').find(btn => btn.text() === 'common.edit')
    const deleteButton = actionsRow.findAll('button').find(btn => btn.text() === 'common.delete')
    expect(editButton).toBeUndefined()
    expect(deleteButton).toBeUndefined()
  })

  it('shows write-action triggers for a non-readonly admin', async () => {
    authStoreMock.mockReturnValue({
      isReadonlyAdmin: false,
      isSimpleMode: false,
      token: 'test-token'
    })

    const wrapper = mount(AccountsView, {
      global: {
        stubs: baseStubs
      }
    })

    await flushPromises()

    const createButton = wrapper
      .findAll('button')
      .find(btn => btn.text() === 'admin.accounts.createAccount')
    expect(createButton).toBeDefined()

    const actionsRow = wrapper.get('[data-test="actions-row"]')
    const editButton = actionsRow.findAll('button').find(btn => btn.text() === 'common.edit')
    const deleteButton = actionsRow.findAll('button').find(btn => btn.text() === 'common.delete')
    expect(editButton).toBeDefined()
    expect(deleteButton).toBeDefined()
  })
})

// Regression coverage for the "child declares/uses a readonly prop but the
// parent forgets to pass it" failure mode: TempUnschedStatusModal only
// gates its own "Recover State" button on a `readonlyAdmin` prop -- if
// AccountsView ever stops forwarding `:readonly-admin="isReadonlyAdmin"` at
// its single call site, this must fail. TempUnschedStatusModal is
// deliberately left unstubbed here (unlike baseStubs) so the real
// parent-to-child prop binding is exercised, not a generic stub.
// DataTableStub above only forwards the cell-select/cell-created_at/cell-actions
// slots; this variant also forwards cell-status, since that's where
// AccountStatusIndicator (and its @show-temp-unsched trigger) actually lives.
const DataTableStubWithStatusCell = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id">
        <slot name="cell-status" :row="row" />
      </div>
    </div>
  `
}

describe('admin AccountsView TempUnschedStatusModal prop forwarding', () => {
  // Same as baseStubs except TempUnschedStatusModal is intentionally omitted
  // (left as the real component), AccountStatusIndicator is replaced with a
  // stub that exposes the real @show-temp-unsched event contract, and
  // DataTable is swapped for a variant that renders the cell-status slot.
  const { TempUnschedStatusModal: _omitted, ...baseStubsWithoutTempUnsched } = baseStubs
  const stubsWithRealTempUnschedModal = {
    ...baseStubsWithoutTempUnsched,
    DataTable: DataTableStubWithStatusCell,
    AccountStatusIndicator: AccountStatusIndicatorStub,
    // BaseDialog teleports to <body>; render inline instead of stubbing it
    // away, so the real modal's footer button is reachable via wrapper queries.
    Teleport: { template: '<div><slot /></div>' }
  }

  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getUpstreamBillingProbeSettings.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    authStoreMock.mockReset()

    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'test-account',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z'
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('hides the recover-state button inside the real modal for a readonly admin', async () => {
    authStoreMock.mockReturnValue({ isReadonlyAdmin: true, isSimpleMode: false, token: 'test-token' })

    const wrapper = mount(AccountsView, {
      global: { stubs: stubsWithRealTempUnschedModal }
    })
    await flushPromises()

    await wrapper.get('[data-test="open-temp-unsched"]').trigger('click')
    await flushPromises()

    const recoverButton = wrapper
      .findAll('button')
      .find((btn) => btn.text().includes('admin.accounts.recoverState'))
    expect(recoverButton).toBeUndefined()
  })

  it('shows the recover-state button inside the real modal for a non-readonly admin', async () => {
    authStoreMock.mockReturnValue({ isReadonlyAdmin: false, isSimpleMode: false, token: 'test-token' })

    const wrapper = mount(AccountsView, {
      global: { stubs: stubsWithRealTempUnschedModal }
    })
    await flushPromises()

    await wrapper.get('[data-test="open-temp-unsched"]').trigger('click')
    await flushPromises()

    const recoverButton = wrapper
      .findAll('button')
      .find((btn) => btn.text().includes('admin.accounts.recoverState'))
    expect(recoverButton).toBeDefined()
  })
})
