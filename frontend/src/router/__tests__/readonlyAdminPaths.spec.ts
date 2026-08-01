import { describe, it, expect } from 'vitest'
import { isReadonlyAdminPathAllowed } from '../readonlyAdminPaths'

describe('isReadonlyAdminPathAllowed', () => {
  it('allows the five whitelisted admin sections', () => {
    expect(isReadonlyAdminPathAllowed('/admin/accounts')).toBe(true)
    expect(isReadonlyAdminPathAllowed('/admin/groups')).toBe(true)
    expect(isReadonlyAdminPathAllowed('/admin/channels')).toBe(true)
    expect(isReadonlyAdminPathAllowed('/admin/channels/pricing')).toBe(true)
    expect(isReadonlyAdminPathAllowed('/admin/users')).toBe(true)
    expect(isReadonlyAdminPathAllowed('/admin/ops')).toBe(true)
    expect(isReadonlyAdminPathAllowed('/admin/ops/alerts')).toBe(true)
  })

  it('denies everything else under /admin', () => {
    for (const p of [
      '/admin/dashboard',
      '/admin/settings',
      '/admin/orders',
      '/admin/audit-logs',
      '/admin/proxies',
      '/admin/redeem',
      '/admin/backup',
      '/admin/subscriptions'
    ]) {
      expect(isReadonlyAdminPathAllowed(p)).toBe(false)
    }
  })

  it('does not allow a prefix to leak into a sibling section', () => {
    // '/admin/users' 允许，但 '/admin/users-export' 是另一个段，必须拒绝
    expect(isReadonlyAdminPathAllowed('/admin/users-export')).toBe(false)
  })
})
