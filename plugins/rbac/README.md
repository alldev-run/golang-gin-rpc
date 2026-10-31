# rbac

基于角色的访问控制插件。维护 role → permissions 映射，提供 `HasPermission(role, perm)` 判定。

## 配置段

无专属配置段；策略由业务在启动时注入。

## 依赖

- 无

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/rbac"

p := rbac.NewPolicy(map[string][]string{
    "admin":  {"user.read", "user.write"},
    "viewer": {"user.read"},
})
if p.HasPermission("admin", "user.write") { /* ... */ }
```
