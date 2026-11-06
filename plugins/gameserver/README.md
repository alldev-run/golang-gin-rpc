# 游戏服务端插件

当前模块提供 `aoi` 子包：一个并发安全的九宫格 Area of Interest 空间索引，用于实时游戏
场景中的邻居查询。它不是完整的 room/session 服务端框架。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/gameserver/aoi`

## 配置项

没有 YAML 配置。`aoi.NewManager(cellSize float64)` 的 `cellSize` 是网格边长；非正值时
使用默认 `100`。坐标可为负数。

## 快速开始

```go
manager := aoi.NewManager(100)
manager.Enter(playerID, x, y)
manager.Move(playerID, nextX, nextY)
for _, entity := range manager.Around(playerID) {
	broadcast(entity.ID)
}
manager.Leave(playerID)
```

## API 概览

- `Entity`：`ID int64`、`X` / `Y float64`
- `NewManager`、`CellSize`、`Enter`、`Move`、`Leave`、`Get`、`Count`
- `Around(id)` 返回实体所在格周围 3×3 区域内的邻居；`AroundPoint(x, y)` 查询坐标周围邻居。

## 错误与注意事项

`Move` 对不存在的实体不执行操作；`Leave` 返回实体是否存在。`Enter` 重复 ID 等同于移动。
查询只覆盖相邻九个格子，不是固定半径的欧氏距离查询。

## 生产运维建议

Manager 可并发调用；应在 room 生命周期内复用实例，并在离开场景时调用 `Leave`。
查询/更新成本与相关网格中的实体数量相关。CI 对插件运行 `go test -race`。

## 当前边界

场景/Room 生命周期、状态同步及 Interest Management 尚未由此模块提供；这些能力仍需由应用实现。
