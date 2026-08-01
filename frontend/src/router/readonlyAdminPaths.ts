/**
 * readonly_admin 角色可访问的管理后台页面根路径。
 *
 * 这是体验层的守卫，用于避免只读账号进入一堆全是 403 的页面。
 * 真正的权限边界在后端 middleware.ReadonlyAdminGuard 的白名单。
 * 两处需保持语义一致：后端放行哪些模块，这里就列哪些。
 */
const READONLY_ADMIN_ROOTS = [
  '/admin/accounts',
  '/admin/groups',
  '/admin/channels',
  '/admin/users',
  '/admin/ops'
] as const

export function isReadonlyAdminPathAllowed(path: string): boolean {
  return READONLY_ADMIN_ROOTS.some(
    // 精确匹配根路径，或匹配其子路径。用 '/' 分隔符判断，
    // 避免 '/admin/users' 意外放行 '/admin/users-export'。
    (root) => path === root || path.startsWith(root + '/')
  )
}

/** readonly_admin 登录后的落地页。/admin/dashboard 对该角色不可用。 */
export const READONLY_ADMIN_HOME = '/admin/accounts'
