// Package aoi 提供九宫格 AOI(Area of Interest)空间索引,用于大世界/房间内
// 的邻居查询。实体跨格移动时自动维护索引,周身查询复杂度为 O(邻格实体数),
// 与全图规模无关。
//
// 典型用法(游戏房间):
//
//	m := aoi.NewManager(100)          // 格子边长 100
//	m.Enter(playerID, x, y)
//	m.Move(playerID, nx, ny)
//	neighbors := m.Around(playerID)   // 3x3 格内的其他实体
//	m.Leave(playerID)
//
// 坐标系无边界限制(支持负坐标);并发安全。
package aoi

import "sync"

// Entity 是空间中的实体快照。
type Entity struct {
	ID   int64
	X, Y float64
}

type cellKey struct{ cx, cy int }

// cell 是一个格子内的实体集合(按 ID 索引)。
type cell struct {
	entities map[int64]*Entity
}

// Manager 是网格 AOI 管理器。
type Manager struct {
	cellSize float64

	mu         sync.RWMutex
	cells      map[cellKey]*cell
	entities   map[int64]*Entity
	entityCell map[int64]cellKey
}

// NewManager 创建 AOI 管理器,cellSize 为格子边长(必须为正)。
func NewManager(cellSize float64) *Manager {
	if cellSize <= 0 {
		cellSize = 100
	}
	return &Manager{
		cellSize:   cellSize,
		cells:      make(map[cellKey]*cell),
		entities:   make(map[int64]*Entity),
		entityCell: make(map[int64]cellKey),
	}
}

// CellSize 返回格子边长。
func (m *Manager) CellSize() float64 { return m.cellSize }

// floorDiv 对负坐标同样正确的地板除。
func floorDiv(v, d float64) int {
	c := int(v / d)
	if v < 0 && float64(c)*d != v {
		c--
	}
	return c
}

func (m *Manager) keyAt(x, y float64) cellKey {
	return cellKey{cx: floorDiv(x, m.cellSize), cy: floorDiv(y, m.cellSize)}
}

func (m *Manager) cellAt(k cellKey) *cell {
	c, ok := m.cells[k]
	if !ok {
		c = &cell{entities: make(map[int64]*Entity)}
		m.cells[k] = c
	}
	return c
}

// Enter 将实体放入世界(若已存在则等价于 Move)。
func (m *Manager) Enter(id int64, x, y float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.entities[id]; exists {
		m.moveLocked(id, x, y)
		return
	}
	e := &Entity{ID: id, X: x, Y: y}
	k := m.keyAt(x, y)
	m.cellAt(k).entities[id] = e
	m.entities[id] = e
	m.entityCell[id] = k
}

// Move 更新实体位置,跨格时迁移索引。
func (m *Manager) Move(id int64, x, y float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.moveLocked(id, x, y)
}

func (m *Manager) moveLocked(id int64, x, y float64) {
	e, ok := m.entities[id]
	if !ok {
		return
	}
	e.X, e.Y = x, y
	nk := m.keyAt(x, y)
	ok2 := m.entityCell[id]
	if ok2 != nk {
		if c := m.cells[ok2]; c != nil {
			delete(c.entities, id)
		}
		m.cellAt(nk).entities[id] = e
		m.entityCell[id] = nk
	}
}

// Leave 将实体移出世界。返回是否确实存在过。
func (m *Manager) Leave(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entities[id]
	if !ok {
		return false
	}
	k := m.entityCell[id]
	if c := m.cells[k]; c != nil {
		delete(c.entities, e.ID)
	}
	delete(m.entities, id)
	delete(m.entityCell, id)
	return true
}

// Get 返回实体当前位置快照。
func (m *Manager) Get(id int64) (Entity, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entities[id]
	if !ok {
		return Entity{}, false
	}
	return *e, true
}

// Count 返回世界中的实体总数。
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entities)
}

// Around 返回实体周身 3x3 格内的其他实体(不含自身)。
func (m *Manager) Around(id int64) []Entity {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.entityCell[id]
	if !ok {
		return nil
	}
	return m.aroundCellLocked(k, id)
}

// AroundPoint 返回以 (x, y) 为中心 3x3 格内的所有实体。
func (m *Manager) AroundPoint(x, y float64) []Entity {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.aroundCellLocked(m.keyAt(x, y), -1)
}

func (m *Manager) aroundCellLocked(k cellKey, excludeID int64) []Entity {
	out := make([]Entity, 0, 8)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			c, ok := m.cells[cellKey{cx: k.cx + dx, cy: k.cy + dy}]
			if !ok {
				continue
			}
			for id, e := range c.entities {
				if id == excludeID {
					continue
				}
				out = append(out, *e)
			}
		}
	}
	return out
}
