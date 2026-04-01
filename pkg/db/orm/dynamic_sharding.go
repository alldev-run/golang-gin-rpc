package orm

import (
	"context"
	"fmt"
	"sort"
)

///////////////////////////////////////////////////////////
////////////////////// Shard Strategy //////////////////////
///////////////////////////////////////////////////////////

type ShardStrategy interface {
	CalculateShard(key, value interface{}) (dbIdx, tableIdx int)
}

// ======================= Hash =======================

type HashShardStrategy struct {
	DBCount    int
	TableCount int
}

func NewHashShardStrategy(db, table int) *HashShardStrategy {
	return &HashShardStrategy{DBCount: db, TableCount: table}
}

func (s *HashShardStrategy) CalculateShard(_, value interface{}) (int, int) {
	h := hashValue(value)
	// 使用位运算替代取模（如果 DBCount 是 2 的幂），这里保持通用取模
	return int(h % uint64(s.DBCount)), int(secondaryHash(h) % uint64(s.TableCount))
}

// ======================= Weighted =======================

type WeightedShardStrategy struct {
	DBCount     int
	TableCount  int
	Weights     []int
	totalWeight int // 预计算总权重
}

func NewWeightedShardStrategy(db, table int, weights []int) *WeightedShardStrategy {
	if len(weights) == 0 {
		weights = make([]int, db)
		for i := range weights {
			weights[i] = 100
		}
	}
	total := 0
	for _, w := range weights {
		total += w
	}
	return &WeightedShardStrategy{
		DBCount:     db,
		TableCount:  table,
		Weights:     weights,
		totalWeight: total,
	}
}

func (s *WeightedShardStrategy) CalculateShard(_, value interface{}) (int, int) {
	if s.totalWeight <= 0 {
		return 0, 0
	}
	h := hashValue(value)
	mod := int(h % uint64(s.totalWeight))

	acc := 0
	dbIdx := 0
	for i, w := range s.Weights {
		acc += w
		if mod < acc {
			dbIdx = i
			break
		}
	}
	return dbIdx, int(secondaryHash(h) % uint64(s.TableCount))
}

// ======================= Consistent Hash =======================

type ConsistentHash struct {
	nodes       []string
	nodeToIndex map[string]int // 优化：通过节点名快速反查索引 O(1)
	ring        []uint64
	nodeMap     map[uint64]string
	replicas    int
}

func NewConsistentHash(nodes []string, replicas int) *ConsistentHash {
	ch := &ConsistentHash{
		nodes:       nodes,
		nodeToIndex: make(map[string]int),
		nodeMap:     make(map[uint64]string),
		replicas:    replicas,
	}

	for idx, n := range nodes {
		ch.nodeToIndex[n] = idx // 记录索引
		for i := 0; i < replicas; i++ {
			// 增加虚拟节点，使用更具区分度的前缀
			h := hashValue(fmt.Sprintf("NODE-%s-REPLICA-%d", n, i))
			ch.ring = append(ch.ring, h)
			ch.nodeMap[h] = n
		}
	}

	sort.Slice(ch.ring, func(i, j int) bool { return ch.ring[i] < ch.ring[j] })
	return ch
}

func (c *ConsistentHash) GetNodeInfo(key string) (string, int) {
	if len(c.ring) == 0 {
		return "", -1
	}
	h := hashValue(key)
	idx := sort.Search(len(c.ring), func(i int) bool {
		return c.ring[i] >= h
	})
	if idx == len(c.ring) {
		idx = 0
	}
	nodeName := c.nodeMap[c.ring[idx]]
	return nodeName, c.nodeToIndex[nodeName]
}

// ======================= Consistent Strategy =======================

type ConsistentShardStrategy struct {
	dbRing    *ConsistentHash
	tableRing *ConsistentHash
}

func NewConsistentShardStrategy(dbNodes, tableNodes []string) *ConsistentShardStrategy {
	return &ConsistentShardStrategy{
		dbRing:    NewConsistentHash(dbNodes, 100),
		tableRing: NewConsistentHash(tableNodes, 100),
	}
}

func (s *ConsistentShardStrategy) CalculateShard(_, value interface{}) (int, int) {
	key := fmt.Sprintf("%v", value)
	_, dbIdx := s.dbRing.GetNodeInfo(key)
	_, tbIdx := s.tableRing.GetNodeInfo(key)

	if dbIdx == -1 {
		dbIdx = 0
	}
	if tbIdx == -1 {
		tbIdx = 0
	}
	return dbIdx, tbIdx
}

///////////////////////////////////////////////////////////
////////////////////// Cross Shard /////////////////////////
///////////////////////////////////////////////////////////

type CrossShardQuery struct {
	queries []*SelectBuilder
	orderBy string
	desc    bool
	limit   int
	offset  int
}

func (q *CrossShardQuery) Exec(ctx context.Context) ([]map[string]interface{}, error) {
	// 使用通道收集结果，减少锁竞争
	type result struct {
		data []map[string]interface{}
		err  error
	}
	resChan := make(chan result, len(q.queries))

	for _, query := range q.queries {
		go func(qb *SelectBuilder) {
			// 生产环境下，这里应该执行真正的查询
			// _, err := qb.Query(ctx)
			// 模拟数据填充...
			resChan <- result{
				data: []map[string]interface{}{{"id": int64(1), "name": "test", "value": 1.0}},
				err:  nil,
			}
		}(query)
	}

	all := make([]map[string]interface{}, 0)
	for i := 0; i < len(q.queries); i++ {
		select {
		case res := <-resChan:
			if res.err == nil {
				all = append(all, res.data...)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// 排序逻辑优化：增加类型断言安全检查
	if q.orderBy != "" {
		sort.Slice(all, func(i, j int) bool {
			return compare(all[i][q.orderBy], all[j][q.orderBy], q.desc)
		})
	}

	// 分页逻辑
	return applyPagination(all, q.offset, q.limit), nil
}

///////////////////////////////////////////////////////////
////////////////////// Utils ///////////////////////////////
///////////////////////////////////////////////////////////

// 优化：将 hash 函数重命名，避免与内置术语冲突
func hashValue(v interface{}) uint64 {
	s := fmt.Sprintf("%v", v)
	var h uint64 = 14695981039346656037 // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}

func secondaryHash(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

func compare(vi, vj interface{}, desc bool) bool {
	var res bool
	switch a := vi.(type) {
	case int64:
		b, _ := vj.(int64)
		res = a < b
	case float64:
		b, _ := vj.(float64)
		res = a < b
	case string:
		b, _ := vj.(string)
		res = a < b
	default:
		return false
	}
	if desc {
		return !res
	}
	return res
}

func applyPagination(data []map[string]interface{}, offset, limit int) []map[string]interface{} {
	start := offset
	if start >= len(data) {
		return []map[string]interface{}{}
	}
	end := start + limit
	if limit <= 0 || end > len(data) {
		end = len(data)
	}
	return data[start:end]
}
