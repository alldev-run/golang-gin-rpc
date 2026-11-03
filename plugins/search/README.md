# 搜索插件

> ⚠️ **Experimental** — 目前没有通用搜索抽象；现有实现是 Elasticsearch 专用子包，
> API 与后端绑定，后续可能调整。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/search/elasticsearch`

## 配置项

| `elasticsearch.Config` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Addresses` | `[]string` | `["http://localhost:9200"]` | Elasticsearch 节点 URL |
| `Username` / `Password` | `string` | 空 | Basic Auth |
| `APIKey` | `string` | 空 | API key 认证 |
| `CloudID` | `string` | 空 | Elastic Cloud ID |
| `Timeout` | `time.Duration` | `30s` | 配置字段；请求 context 控制请求生命周期 |
| `MaxRetries` | `int` | `3` | 客户端重试次数 |

## 快速开始

```go
import (
	"context"
	"time"

	elasticsearch "github.com/alldev-run/golang-gin-rpc/plugins/search/elasticsearch"
)

func search(ctx context.Context) error {
	client, err := elasticsearch.New(elasticsearch.Config{
		Addresses: []string{"https://search.example.com:9200"},
		Username:  "service-user",
		Password:  "load-from-secret-store",
		Timeout:   10 * time.Second,
	})
	if err != nil {
		return err
	}
	response, err := client.SearchSimple(ctx, "users", "name", "alice")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return nil
}
```

`New` 会在构造时执行 5 秒连接检查；后续操作应传递带 deadline 的 context。导入该子包
也会向 `plugins/db` 注册 Elasticsearch driver；不导入时不会注册。

## API 概览

- `DefaultConfig`、`New`、`Client.Info`
- 文档操作：`Index`、`BulkIndex`、`Get`、`Delete`、`Exists`
- 查询/索引管理：`Search`、`SearchSimple`、`Count`、`CreateIndex`、`DeleteIndex`、
  `IndexExists`、`Refresh`、`Aggregate`
- `BuildMatchAllQuery`、`BuildTermQuery`、`BuildRangeQuery`、`BuildBoolQuery`、
  `BuildMultiMatchQuery`、`ParseResponse`

## 错误与注意事项

构造时连接检查失败会返回错误；搜索响应 body 由调用方关闭，并应检查 HTTP 状态和响应体。
子包还会注册数据库驱动，但不提供跨搜索后端的兼容层。

## 生产运维建议

使用 TLS 和 secret store，不记录认证信息；按集群容量设置重试，配置调用 deadline，并
监控连接失败、响应延迟和集群拒绝情况。
