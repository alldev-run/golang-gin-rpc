# 角色访问控制（RBAC）

维护 role → permissions 映射，并根据调用方提供的角色列表检查某项权限。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/rbac`

## 配置项

无 YAML 配置结构；应用在启动时创建策略并注入角色权限。

| 构造参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `NewPolicy(permissions)` | `map[string][]string` | 空映射 | key 为角色名，value 为权限列表 |

## 快速开始

```go
policy := rbac.NewPolicy(map[string][]string{
	"admin":  {"user.read", "user.write"},
	"viewer": {"user.read"},
})
if policy.HasPermission([]string{"admin"}, "user.write") {
	// allow operation
}
```

## API 概览

- `NewPolicy` 创建策略。
- `(*Policy).HasPermission(roles []string, permission string)` 任一角色有权限时返回 `true`。
- `SetRolePermissions` 替换单个角色权限；`AddPermission` 增加权限。

## 错误与注意事项

权限字符串由应用定义并保持一致；RBAC 不负责认证、角色来源校验或资源级条件表达式。
拒绝访问应由调用方转换为适当的 HTTP/RPC 状态。

## 生产运维建议

策略适合进程内读取；变更通过内部互斥锁保护。多实例部署应通过版本化配置同步策略，
并记录授权拒绝事件但不要泄漏敏感请求内容。
