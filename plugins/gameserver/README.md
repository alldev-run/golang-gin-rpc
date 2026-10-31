# plugins/gameserver

`plugins/gameserver` 承载实时游戏服务端的域组件。

## aoi — 九宫格 AOI 空间管理

- `NewManager(cellSize)` 创建网格管理器,实体跨格移动自动迁移索引
- `Enter / Move / Leave / Get` 维护实体位置;`Count` 全图规模
- `Around(id)` / `AroundPoint(x, y)`:3x3 格内邻居查询,O(邻格实体数),与全图规模无关
- 支持负坐标、并发安全(`go test -race` 待 CI)

```go
m := aoi.NewManager(100)
m.Enter(playerID, x, y)
m.Move(playerID, nx, ny)
for _, e := range m.Around(playerID) { /* 广播/碰撞 */ }
m.Leave(playerID)
```

## 路线图

- 场景/Room 生命周期与 bootstrap GameServer 服务类型
- 状态同步原语(兴趣管理/增量快照)
