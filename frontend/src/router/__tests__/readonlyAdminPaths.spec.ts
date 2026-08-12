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

describe('isReadonlyAdminPathAllowed exclusions inside an otherwise-allowed root', () => {
  it('denies /admin/channels/monitor even though /admin/channels is allowed', () => {
    // 后端模块是独立的 /admin/channel-monitors，对 readonly_admin 整体拒绝，
    // 纯前缀规则会误放行 —— 这条断言就是防止那个回归。
    expect(isReadonlyAdminPathAllowed('/admin/channels/monitor')).toBe(false)
  })

  it('does not let the monitor exclusion swallow its sibling /admin/channels/pricing', () => {
    expect(isReadonlyAdminPathAllowed('/admin/channels/pricing')).toBe(true)
  })

  it('denies nested children under an excluded path too', () => {
    expect(isReadonlyAdminPathAllowed('/admin/channels/monitor/123')).toBe(false)
  })

  it('does not let the exclusion leak into an unrelated sibling segment', () => {
    // '/admin/channels/monitor' 被拒绝，但 '/admin/channels/monitor-x' 是另一个段
    expect(isReadonlyAdminPathAllowed('/admin/channels/monitor-x')).toBe(true)
  })
})
