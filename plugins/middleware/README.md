# middleware

HTTP 中间件集合。提供与框架解耦的 `net/http` 中间件：audit / apikey / cors / ip_filter / rate_limiter / recovery 等，可独立用于任何 `net/http` 服务。

## 配置段

无专属配置段；按需在路由上装配。

## 依赖

- `gin-gonic/gin`（gin 适配子包）

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/middleware"

mux := http.NewServeMux()
mux.Handle("/", middleware.Recovery(middleware.APIKey(handler, cfg)))
```
