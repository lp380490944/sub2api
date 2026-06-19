# 分组账号选择功能实现总结

## 功能概述

为分组管理添加了直接选择账号的功能。用户现在可以在创建/编辑分组时，通过 `account_ids` 字段直接指定要归入该分组的账号列表，作为 `copy_accounts_from_group_ids`（从分组复制账号）的替代或补充方案。

**优先级规则**：当同时指定 `account_ids` 和 `copy_accounts_from_group_ids` 时，`account_ids` 优先生效。

## 实现的功能模块

### 1. 后端 API 层 (✅ 完成)

#### 修改的文件
- `backend/internal/api/admin/groups.go`
  - `CreateGroupRequest` 添加 `AccountIDs []int64` 字段
  - `UpdateGroupRequest` 添加 `AccountIDs []int64` 字段

#### 新增功能
- 创建分组时支持 `account_ids` 参数
- 更新分组时支持 `account_ids` 参数
- API 自动验证账号是否存在、是否为 API Key 类型、是否与分组平台匹配

### 2. 后端 Service 层 (✅ 完成)

#### 修改的文件
- `backend/internal/service/admin_service.go`
  - `CreateGroupInput` 添加 `AccountIDs []int64` 字段
  - `UpdateGroupInput` 添加 `AccountIDs []int64` 字段
  - `CreateGroup()` 方法实现优先级逻辑
  - `UpdateGroup()` 方法实现优先级逻辑

#### 核心逻辑
```go
// 优先级：AccountIDs > CopyAccountsFromGroupIDs
if len(input.AccountIDs) > 0 {
    accountIDs = input.AccountIDs
} else if len(input.CopyAccountsFromGroupIDs) > 0 {
    // 从源分组获取账号
    accountIDs, err = s.groupRepo.GetAccountIDsByGroupIDs(ctx, input.CopyAccountsFromGroupIDs)
}
```

#### 验证规则
1. 账号必须存在
2. 账号平台必须与分组平台匹配
3. 特殊限制：当分组设置 `require_oauth_only = true` 时，系统会自动过滤掉 API Key 类型账号

### 3. 前端 UI (✅ 完成)

#### 修改的文件
- `frontend/src/views/admin/GroupsView.vue`
  - 创建表单添加账号多选器
  - 编辑表单添加账号多选器
  - 加载账号列表数据
  - 编辑时自动回填当前分组的账号

#### UI 组件
- **账号选择器**：下拉选择器，显示当前平台的所有活跃账号
- **已选账号展示**：Chips 展示已选账号，可单独移除
- **提示文案**：说明 `account_ids` 优先于 `copy_accounts_from_group_ids`

#### 实现细节
```typescript
// 加载所有账号供选择
const loadAccounts = async () => {
  const response = await adminAPI.accounts.list(1, 1000, {
    sort_by: 'name',
    sort_order: 'asc',
  });
  allAccountsForSelection.value = response.items.map(...);
};

// 编辑时加载当前分组的账号
const response = await adminAPI.accounts.list(1, 1000, {
  group: String(group.id),
  status: 'active',
});
editForm.account_ids = response.items.map((acc: any) => acc.id);
```

### 4. 国际化 (✅ 完成)

#### 修改的文件
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

#### 新增翻译
```typescript
accountIds: {
  title: '选择账号',
  tooltip: '直接选择归入该分组的账号。若同时指定了"从分组复制账号"，此选项优先。',
  selectPlaceholder: '选择账号...',
  hint: '可选择多个账号，优先于"从分组复制账号"'
}
```

### 5. 测试与验证 (✅ 完成)

#### 验证内容
- ✅ 后端代码编译通过
- ✅ 前端 TypeScript 类型检查通过
- ✅ 现有单元测试全部通过（确保没有破坏现有功能）

## 使用示例

### API 请求示例

#### 创建分组并直接指定账号
```json
POST /admin/groups
{
  "name": "OpenAI Team A",
  "platform": "openai",
  "rate_multiplier": 1.2,
  "account_ids": [1, 2, 3],  // 直接指定账号 ID
  "subscription_type": "standard"
}
```

#### 同时指定 account_ids 和 copy_accounts_from_group_ids
```json
POST /admin/groups
{
  "name": "New Group",
  "platform": "anthropic",
  "account_ids": [10, 20],              // 优先使用这个
  "copy_accounts_from_group_ids": [5],  // 被忽略
  "rate_multiplier": 1.0,
  "subscription_type": "standard"
}
```
**结果**：分组会绑定账号 10 和 20，而不是从分组 5 复制账号。

#### 更新分组账号
```json
PUT /admin/groups/123
{
  "account_ids": [30, 40, 50]  // 替换分组的账号为 30, 40, 50
}
```

## 关键设计决策

1. **优先级设计**：`account_ids` 优先于 `copy_accounts_from_group_ids`
   - 理由：直接选择比复制更明确，用户意图更清晰

2. **验证规则**：当分组设置 `require_oauth_only = true` 时过滤 API Key 账号
   - 理由：某些场景需要强制使用 OAuth 认证（如需要刷新 token）

3. **平台匹配**：账号平台必须与分组平台一致
   - 理由：跨平台账号无法在同一分组中使用

4. **UI 实现**：使用 Chips + 下拉选择器
   - 理由：直观展示已选账号，支持逐个移除，交互体验好

## 文件变更清单

### 后端
- `backend/internal/api/admin/groups.go` - API 请求结构体
- `backend/internal/service/admin_service.go` - 业务逻辑实现

### 前端
- `frontend/src/views/admin/GroupsView.vue` - UI 组件与逻辑
- `frontend/src/i18n/locales/zh.ts` - 中文翻译
- `frontend/src/i18n/locales/en.ts` - 英文翻译

## 未来改进建议

1. **批量操作**：支持批量添加/移除账号
2. **搜索过滤**：账号列表支持搜索功能（当账号数量很多时）
3. **集成测试**：添加端到端测试验证完整流程
4. **性能优化**：大量账号时考虑分页加载

## 测试建议

### 手动测试场景

1. **创建分组时选择账号**
   - 创建新分组，选择 2-3 个账号
   - 验证分组创建后账号正确绑定

2. **编辑分组账号**
   - 编辑现有分组，修改账号列表
   - 验证保存后账号绑定更新

3. **优先级测试**
   - 同时指定 `account_ids` 和 `copy_accounts_from_group_ids`
   - 验证 `account_ids` 生效，`copy_accounts_from_group_ids` 被忽略

4. **验证错误处理**
   - 尝试选择不存在的账号
   - 尝试选择错误平台的账号
   - 测试 `require_oauth_only = true` 时 API Key 账号被自动过滤

5. **UI 交互测试**
   - 验证账号列表正确加载
   - 验证已选账号 Chips 显示
   - 验证单个账号可以移除

## 兼容性说明

- ✅ **向后兼容**：现有的 `copy_accounts_from_group_ids` 功能不受影响
- ✅ **数据库兼容**：无需修改数据库 schema
- ✅ **API 兼容**：新增字段为可选，不影响现有 API 调用

## 结论

该功能为分组管理提供了更灵活的账号绑定方式，用户可以根据需求选择：
1. 直接选择账号（`account_ids`）
2. 从其他分组复制账号（`copy_accounts_from_group_ids`）
3. 两者结合使用（`account_ids` 优先）

实现符合设计要求，代码质量良好，测试验证通过。
