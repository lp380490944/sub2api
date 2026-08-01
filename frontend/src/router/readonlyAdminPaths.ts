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

/**
 * 前端路由路径 ≠ 后端模块路径的例外清单。
 *
 * READONLY_ADMIN_ROOTS 是按前缀放行的：只要路径落在某个根下面就算允许。这个假设
 * 在大多数情况下成立（前端路由结构基本照抄后端模块结构），但不是恒成立 —— 有的
 * 前端路由挂在一个允许的根下面，实际调用的却是另一个被后端拒绝的模块。
 *
 * 典型例子：`/admin/channels/monitor` 是 `/admin/channels` 根下的子路由，但它对应
 * 的后端模块是完全独立的 `/admin/channel-monitors`（见
 * backend/internal/server/routes/readonly_admin_coverage_test.go 的
 * reviewedDenyPrefixes），不是 `/admin/channels`。这个模块对 readonly_admin 整体
 * 拒绝，所以纯前缀规则会误放行——菜单里出现一个点进去全是 403 的页面，正是这份
 * 白名单存在的目的所要防止的场景。
 *
 * 新增路由时如果它的后端模块名字和它的前端路径前缀对不上，把它记在这里，并注明
 * 对应的后端模块，别只加一行不写理由。
 */
const READONLY_ADMIN_EXCLUDED_PATHS = [
  '/admin/channels/monitor' // 后端模块 /admin/channel-monitors，对 readonly_admin 整体拒绝
] as const

export function isReadonlyAdminPathAllowed(path: string): boolean {
  // 排除项优先于前缀放行判断：一个允许根下的已知例外必须先被拦下，
  // 否则下面的 root-prefix 匹配会把它当成允许根的普通子路径放行。
  const isExcluded = READONLY_ADMIN_EXCLUDED_PATHS.some(
    (excluded) => path === excluded || path.startsWith(excluded + '/')
  )
  if (isExcluded) return false

  return READONLY_ADMIN_ROOTS.some(
    // 精确匹配根路径，或匹配其子路径。用 '/' 分隔符判断，
    // 避免 '/admin/users' 意外放行 '/admin/users-export'。
    (root) => path === root || path.startsWith(root + '/')
  )
}

/** readonly_admin 登录后的落地页。/admin/dashboard 对该角色不可用。 */
export const READONLY_ADMIN_HOME = '/admin/accounts'
