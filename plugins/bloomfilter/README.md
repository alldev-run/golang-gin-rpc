# 布隆过滤器

提供并发安全、空间效率高的内存概率集合，用于判断元素“可能存在”或“一定不存在”。
该结构允许假阳性，不会产生假阴性。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/bloomfilter`

## 配置项

此包没有 YAML 配置结构。`New(n uint64, p float64)` 的 `n` 为预计元素数，
`p` 为目标假阳性率（取值 `(0,1)`）。

## 快速开始

```go
filter := bloomfilter.New(100_000, 0.01)
filter.Add([]byte("account:42"))
if filter.Contains([]byte("account:42")) {
	// 可能存在；仍应查询真实数据源确认
}
```

## API 概览

- `New` 创建过滤器。
- `(*BloomFilter).Add`、`Contains`、`Clear`、`Size`、`HashCount`。

## 错误与注意事项

`Contains` 返回 `true` 不是存在证明；不支持删除单个元素。容量和误判率在创建时确定，
预计元素数量应接近实际规模。

## 生产运维建议

过滤器驻留在进程内，重启后需重建；不要作为唯一数据源或安全边界。多实例部署需由
应用同步重建或预热各实例。
