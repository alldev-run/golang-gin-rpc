# push

推送通知插件。同时支持 APNs（HTTP/2 + JWT 鉴权）与 FCM，提供统一发送接口。

## 配置段

```yaml
plugins:
  push:
    enabled: true
    apns:
      key_id: "..."
      team_id: "..."
      bundle_id: "..."
    fcm:
      credentials_file: "..."
```

## 依赖

- `golang.org/x/crypto/pkcs12`、`golang-jwt/jwt/v5`

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/push"

p, _ := push.NewPusher(pushCfg)
_ = p.SendToAPNs(ctx, token, &push.Notification{Title: "hi", Body: "world"})
```
