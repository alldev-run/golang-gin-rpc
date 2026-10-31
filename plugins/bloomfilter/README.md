# bloomfilter

布隆过滤器插件。提供空间效率高的概率型集合成员判定，可配置期望误判率；并发安全。

## 配置段

无专属配置段；按需在代码中构造。

## 依赖

- 无

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/bloomfilter"

bf := bloomfilter.New(1_000_000, 0.01)
bf.Add([]byte("user:42"))
if bf.Test([]byte("user:42")) { /* probably exists */ }
```
