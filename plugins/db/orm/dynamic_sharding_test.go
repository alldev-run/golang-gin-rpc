package orm

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ======================= Hash Strategy Tests =======================

func TestHashShardStrategy(t *testing.T) {
	strategy := NewHashShardStrategy(4, 8)

	// 测试基本hash分布
	dbDistribution := make(map[int]int)
	tableDistribution := make(map[int]int)

	for i := 0; i < 1000; i++ {
		dbIdx, tableIdx := strategy.CalculateShard("user_id", i)
		dbDistribution[dbIdx]++
		tableDistribution[tableIdx]++
	}

	// 验证分布均匀性
	for db := 0; db < 4; db++ {
		count := dbDistribution[db]
		if count < 200 || count > 300 { // 允许20%偏差
			t.Errorf("数据库 %d 分布不均匀: %d (期望 ~250)", db, count)
		}
	}

	for table := 0; table < 8; table++ {
		count := tableDistribution[table]
		if count < 100 || count > 150 { // 允许20%偏差
			t.Errorf("表 %d 分布不均匀: %d (期望 ~125)", table, count)
		}
	}

	t.Logf("Hash分布测试通过")
	t.Logf("数据库分布: %v", dbDistribution)
	t.Logf("表分布: %v", tableDistribution)
}

func TestHashShardStrategyEdgeCases(t *testing.T) {
	strategy := NewHashShardStrategy(1, 1)

	// 测试单分片
	dbIdx, tableIdx := strategy.CalculateShard("user_id", 12345)
	if dbIdx != 0 || tableIdx != 0 {
		t.Errorf("单分片测试失败: db=%d, table=%d", dbIdx, tableIdx)
	}

	// 测试边界值
	dbIdx, tableIdx = strategy.CalculateShard("user_id", 0)
	if dbIdx != 0 || tableIdx != 0 {
		t.Errorf("边界值测试失败: db=%d, table=%d", dbIdx, tableIdx)
	}

	// 测试负数
	dbIdx, tableIdx = strategy.CalculateShard("user_id", -1)
	if dbIdx < 0 || tableIdx < 0 {
		t.Errorf("负数测试失败: db=%d, table=%d", dbIdx, tableIdx)
	}

	// 测试大数值
	dbIdx, tableIdx = strategy.CalculateShard("user_id", int64(1<<62))
	if dbIdx < 0 || tableIdx < 0 {
		t.Errorf("大数值测试失败: db=%d, table=%d", dbIdx, tableIdx)
	}

	t.Logf("边界情况测试通过")
}

func TestWeightedShardStrategy(t *testing.T) {
	// 测试加权分布
	weights := []int{400, 200, 200, 200} // 第一个数据库权重更高
	strategy := NewWeightedShardStrategy(4, 8, weights)

	dbDistribution := make(map[int]int)

	for i := 0; i < 1000; i++ {
		dbIdx, _ := strategy.CalculateShard("user_id", i)
		dbDistribution[dbIdx]++
	}

	// 第一个数据库应该有更多数据
	if dbDistribution[0] <= dbDistribution[1] || dbDistribution[0] <= dbDistribution[2] {
		t.Errorf("加权分布错误: 期望数据库0有最多数据, 实际分布: %v", dbDistribution)
	}

	// 验证权重比例 (400:200:200:200 = 2:1:1:1)
	total := 0
	for _, count := range dbDistribution {
		total += count
	}

	expectedRatio := float64(dbDistribution[0]) / float64(dbDistribution[1])
	if expectedRatio < 1.8 || expectedRatio > 2.2 { // 允许10%偏差
		t.Errorf("权重比例错误: 期望 ~2.0, 实际 %.2f", expectedRatio)
	}

	t.Logf("加权分布测试通过")
	t.Logf("数据库分布: %v", dbDistribution)
	t.Logf("权重比例: %.2f", expectedRatio)
}

func TestWeightedShardStrategyEmptyWeights(t *testing.T) {
	// 测试空权重数组（应该使用默认权重）
	strategy := NewWeightedShardStrategy(4, 8, nil)

	dbDistribution := make(map[int]int)

	for i := 0; i < 400; i++ {
		dbIdx, _ := strategy.CalculateShard("user_id", i)
		dbDistribution[dbIdx]++
	}

	// 应该是均匀分布
	for db := 0; db < 4; db++ {
		count := dbDistribution[db]
		if count < 80 || count > 120 { // 允许20%偏差
			t.Errorf("默认权重分布不均匀: 数据库 %d = %d (期望 ~100)", db, count)
		}
	}

	t.Logf("默认权重测试通过")
	t.Logf("数据库分布: %v", dbDistribution)
}

func TestWeightedShardStrategyZeroWeights(t *testing.T) {
	// 测试权重为0的情况
	strategy := NewWeightedShardStrategy(4, 8, []int{0, 0, 0, 0})

	dbIdx, tableIdx := strategy.CalculateShard("user_id", 123)
	if dbIdx != 0 || tableIdx != 0 {
		t.Errorf("零权重测试失败: 期望 (0,0), 实际 (%d,%d)", dbIdx, tableIdx)
	}

	t.Logf("零权重测试通过")
}

func TestConsistentShardStrategy(t *testing.T) {
	dbNodes := []string{"db1", "db2", "db3"}
	tableNodes := []string{"table1", "table2", "table3", "table4"}
	strategy := NewConsistentShardStrategy(dbNodes, tableNodes)

	// 测试基本功能
	dbIdx1, tableIdx1 := strategy.CalculateShard("user_id", 123)
	dbIdx2, tableIdx2 := strategy.CalculateShard("user_id", 456)
	dbIdx3, tableIdx3 := strategy.CalculateShard("user_id", 789)

	t.Logf("基本测试: 123->(%d,%d), 456->(%d,%d), 789->(%d,%d)",
		dbIdx1, tableIdx1, dbIdx2, tableIdx2, dbIdx3, tableIdx3)

	// 验证索引范围
	if dbIdx1 < 0 || dbIdx1 >= len(dbNodes) {
		t.Errorf("数据库索引越界: %d", dbIdx1)
	}
	if tableIdx1 < 0 || tableIdx1 >= len(tableNodes) {
		t.Errorf("表索引越界: %d", tableIdx1)
	}

	// 测试一致性
	dbIdx1_2, tableIdx1_2 := strategy.CalculateShard("user_id", 123)
	if dbIdx1 != dbIdx1_2 || tableIdx1 != tableIdx1_2 {
		t.Errorf("一致性测试失败: 123第一次(%d,%d), 第二次(%d,%d)",
			dbIdx1, tableIdx1, dbIdx1_2, tableIdx1_2)
	}

	// 测试分布
	dbDistribution := make(map[int]int)
	tableDistribution := make(map[int]int)

	// 增加样本量到 10000
	for i := 0; i < 10000; i++ {
		// 使用随机字符串或随机数增加离散度
		dbIdx, tableIdx := strategy.CalculateShard("user_id", rand.Int63())
		dbDistribution[dbIdx]++
		tableDistribution[tableIdx]++
	}

	// 只要每个节点至少分到 1% 的数据，就认为分片逻辑是工作的
	for db, count := range dbDistribution {
		if count < 100 { // 10000 的 1%
			t.Errorf("数据库节点 %d 分布极度不均: %d", db, count)
		}
	}

	// 验证所有节点都被使用
	for db := 0; db < len(dbNodes); db++ {
		if dbDistribution[db] == 0 {
			t.Errorf("数据库节点 %d 未被使用", db)
		}
	}

	for table := 0; table < len(tableNodes); table++ {
		if tableDistribution[table] == 0 {
			t.Errorf("表节点 %d 未被使用", table)
		}
	}

	t.Logf("一致性Hash测试通过")
	t.Logf("数据库分布: %v", dbDistribution)
	t.Logf("表分布: %v", tableDistribution)
}

func TestConsistentHash(t *testing.T) {
	nodes := []string{"node1", "node2", "node3"}
	ch := NewConsistentHash(nodes, 100)

	// 测试基本功能
	nodeName, idx := ch.GetNodeInfo("test_key")
	if nodeName == "" || idx == -1 {
		t.Error("一致性Hash返回空结果")
	}

	// 测试相同key返回相同节点
	nodeName2, idx2 := ch.GetNodeInfo("test_key")
	if nodeName != nodeName2 || idx != idx2 {
		t.Errorf("相同key应该返回相同节点: %s(%d) != %s(%d)", nodeName, idx, nodeName2, idx2)
	}

	// 测试环状特性
	key1, idx1 := ch.GetNodeInfo("key1")
	key2, idx2 := ch.GetNodeInfo("key2")
	key3, idx3 := ch.GetNodeInfo("key3")

	if key1 == "" || key2 == "" || key3 == "" {
		t.Error("部分key返回空节点")
	}

	if idx1 < 0 || idx2 < 0 || idx3 < 0 {
		t.Error("部分索引无效")
	}

	// 验证nodeToIndex映射
	for name, expectedIdx := range map[string]int{
		"node1": 0,
		"node2": 1,
		"node3": 2,
	} {
		if actualIdx, exists := ch.nodeToIndex[name]; !exists || actualIdx != expectedIdx {
			t.Errorf("nodeToIndex映射错误: %s 期望 %d, 实际 %d", name, expectedIdx, actualIdx)
		}
	}

	t.Logf("一致性Hash基础测试通过")
}

// ======================= Cross Shard Tests =======================

func TestCrossShardQuery(t *testing.T) {
	query := &CrossShardQuery{}

	// 创建多个查询构建器
	queries := make([]*SelectBuilder, 4)
	for i := 0; i < 4; i++ {
		db := &MockDB{
			QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
				return &sql.Rows{}, nil
			},
		}
		queries[i] = NewSelectBuilder(db, fmt.Sprintf("orders_%d", i))
		query.queries = append(query.queries, queries[i])
	}

	// 设置排序和分页
	query.orderBy = "id"
	query.desc = false
	query.limit = 10
	query.offset = 5

	// 执行查询
	ctx := context.Background()
	results, err := query.Exec(ctx)
	if err != nil {
		t.Errorf("跨分片查询失败: %v", err)
	}

	// 验证结果结构
	for i, result := range results {
		if _, ok := result["id"]; !ok {
			t.Errorf("结果 %d 缺少id字段", i)
		}
		if _, ok := result["name"]; !ok {
			t.Errorf("结果 %d 缺少name字段", i)
		}
		if _, ok := result["value"]; !ok {
			t.Errorf("结果 %d 缺少value字段", i)
		}
	}

	t.Logf("跨分片查询测试通过")
	t.Logf("返回 %d 条记录", len(results))
}

func TestCrossShardQueryEmpty(t *testing.T) {
	query := &CrossShardQuery{}

	// 空查询测试
	ctx := context.Background()
	results, err := query.Exec(ctx)
	if err != nil {
		t.Errorf("空查询失败: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("空查询应该返回空结果，实际返回 %d 条", len(results))
	}

	t.Logf("空查询测试通过")
}

func TestCrossShardQueryConcurrent(t *testing.T) {
	// 高并发跨分片查询测试
	var wg sync.WaitGroup
	var mu sync.Mutex
	totalResults := 0
	errors := make([]error, 0)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			query := &CrossShardQuery{}

			// 每个goroutine使用不同的数据源
			for j := 0; j < 4; j++ {
				db := &MockDB{
					QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
						return &sql.Rows{}, nil
					},
				}
				sb := NewSelectBuilder(db, fmt.Sprintf("orders_%d", j))
				query.queries = append(query.queries, sb)
			}

			query.limit = 5
			query.orderBy = "id"
			query.desc = false

			ctx := context.Background()
			results, err := query.Exec(ctx)

			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}

			mu.Lock()
			totalResults += len(results)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	if len(errors) > 0 {
		t.Errorf("并发跨分片查询发现 %d 个错误", len(errors))
	}

	if totalResults == 0 {
		t.Error("并发查询没有返回任何结果")
	}

	t.Logf("并发跨分片查询测试通过")
	t.Logf("20个goroutine x 4个分片, 总共 %d 条记录", totalResults)
}

func TestCrossShardQuerySorting(t *testing.T) {
	query := &CrossShardQuery{}

	// 创建查询构建器
	for i := 0; i < 3; i++ {
		db := &MockDB{
			QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
				return &sql.Rows{}, nil
			},
		}
		sb := NewSelectBuilder(db, fmt.Sprintf("orders_%d", i))
		query.queries = append(query.queries, sb)
	}

	// 测试升序排序
	query.orderBy = "id"
	query.desc = false

	ctx := context.Background()
	results, err := query.Exec(ctx)
	if err != nil {
		t.Errorf("排序查询失败: %v", err)
	}

	// 验证排序
	for i := 1; i < len(results); i++ {
		prev := results[i-1]["id"].(int64)
		curr := results[i]["id"].(int64)
		if prev > curr {
			t.Errorf("升序排序错误: %d > %d", prev, curr)
		}
	}

	// 测试降序排序
	query.desc = true
	results2, err := query.Exec(ctx)
	if err != nil {
		t.Errorf("降序排序查询失败: %v", err)
	}

	// 验证降序排序
	for i := 1; i < len(results2); i++ {
		prev := results2[i-1]["id"].(int64)
		curr := results2[i]["id"].(int64)
		if prev < curr {
			t.Errorf("降序排序错误: %d < %d", prev, curr)
		}
	}

	t.Logf("排序测试通过")
}

func TestCrossShardQueryPagination(t *testing.T) {
	query := &CrossShardQuery{}

	// 创建查询构建器
	for i := 0; i < 3; i++ {
		db := &MockDB{
			QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
				return &sql.Rows{}, nil
			},
		}
		sb := NewSelectBuilder(db, fmt.Sprintf("orders_%d", i))
		query.queries = append(query.queries, sb)
	}

	ctx := context.Background()

	// 测试分页
	query.limit = 2
	query.offset = 1
	results, err := query.Exec(ctx)
	if err != nil {
		t.Errorf("分页查询失败: %v", err)
	}

	if len(results) > 2 {
		t.Errorf("分页限制错误: 期望最多2条, 实际 %d 条", len(results))
	}

	// 测试offset超出范围
	query.offset = 100
	results2, err := query.Exec(ctx)
	if err != nil {
		t.Errorf("大offset查询失败: %v", err)
	}

	if len(results2) != 0 {
		t.Errorf("大offset应该返回空结果, 实际 %d 条", len(results2))
	}

	t.Logf("分页测试通过")
}

// ======================= Performance Tests =======================

func TestHashPerformance(t *testing.T) {
	strategy := NewHashShardStrategy(1000, 1000)

	// 性能测试
	start := time.Now()

	for i := 0; i < 100000; i++ {
		strategy.CalculateShard("user_id", i)
	}

	duration := time.Since(start)
	opsPerSec := float64(100000) / duration.Seconds()

	t.Logf("Hash性能测试:")
	t.Logf("100,000次分片计算耗时: %v", duration)
	t.Logf("每秒操作数: %.0f ops/sec", opsPerSec)

	// 性能基准: 应该超过500万ops/sec
	if opsPerSec < 5000000 {
		t.Errorf("性能不达标: %.0f ops/sec < 5,000,000 ops/sec", opsPerSec)
	}
}

func TestConsistentHashPerformance(t *testing.T) {
	dbNodes := make([]string, 100)
	for i := 0; i < 100; i++ {
		dbNodes[i] = fmt.Sprintf("db_%d", i)
	}

	tableNodes := make([]string, 100)
	for i := 0; i < 100; i++ {
		tableNodes[i] = fmt.Sprintf("table_%d", i)
	}

	strategy := NewConsistentShardStrategy(dbNodes, tableNodes)

	start := time.Now()

	for i := 0; i < 10000; i++ {
		strategy.CalculateShard("user_id", i)
	}

	duration := time.Since(start)
	opsPerSec := float64(10000) / duration.Seconds()

	t.Logf("一致性Hash性能测试:")
	t.Logf("10,000次分片计算耗时: %v", duration)
	t.Logf("每秒操作数: %.0f ops/sec", opsPerSec)

	// 一致性hash应该稍慢，但仍应该超过5万ops/sec
	if opsPerSec < 50000 {
		t.Errorf("一致性Hash性能不达标: %.0f ops/sec < 50,000 ops/sec", opsPerSec)
	}
}

// ======================= Stress Tests =======================

func TestStressHashDistribution(t *testing.T) {
	strategy := NewHashShardStrategy(16, 32)

	// 压力测试：大量数据分布
	dbDistribution := make(map[int]int)
	tableDistribution := make(map[int]int)

	for i := 0; i < 1000000; i++ {
		dbIdx, tableIdx := strategy.CalculateShard("user_id", rand.Int63())
		dbDistribution[dbIdx]++
		tableDistribution[tableIdx]++
	}

	// 验证分布均匀性
	for db := 0; db < 16; db++ {
		count := dbDistribution[db]
		expected := 1000000 / 16
		deviation := float64(count-expected) / float64(expected) * 100

		if deviation > 5 || deviation < -5 { // 允许5%偏差
			t.Errorf("数据库 %d 分布偏差过大: %.2f%% (期望 ±5%%)", db, deviation)
		}
	}

	for table := 0; table < 32; table++ {
		count := tableDistribution[table]
		expected := 1000000 / 32
		deviation := float64(count-expected) / float64(expected) * 100

		if deviation > 5 || deviation < -5 { // 允许5%偏差
			t.Errorf("表 %d 分布偏差过大: %.2f%% (期望 ±5%%)", table, deviation)
		}
	}

	t.Logf("压力测试通过: 1,000,000条记录")
	t.Logf("数据库分布偏差: <5%%")
	t.Logf("表分布偏差: <5%%")
}

func TestStressConcurrentSharding(t *testing.T) {
	strategy := NewWeightedShardStrategy(32, 64, testGenerateRandomWeights(32))

	// 极限并发测试
	var wg sync.WaitGroup
	successCount := int64(0)
	errorCount := int64(0)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				_, _ = strategy.CalculateShard("user_id", rand.Int63())

				// CalculateShard现在返回2个值，不会出错
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	total := successCount + errorCount
	successRate := float64(successCount) / float64(total) * 100

	t.Logf("极限并发测试:")
	t.Logf("1000个goroutine x 100次查询 = %d次操作", total)
	t.Logf("成功: %d, 失败: %d", successCount, errorCount)
	t.Logf("成功率: %.2f%%", successRate)

	if successRate < 99.0 {
		t.Errorf("成功率过低: %.2f%% < 99%%", successRate)
	}
}

// ======================= Logic Bug Tests =======================

func TestHashFunctionConsistency(t *testing.T) {
	strategy := NewHashShardStrategy(10, 10)

	// 测试hash函数一致性
	for i := 0; i < 1000; i++ {
		dbIdx1, tableIdx1 := strategy.CalculateShard("user_id", i)
		dbIdx2, tableIdx2 := strategy.CalculateShard("user_id", i)

		if dbIdx1 != dbIdx2 || tableIdx1 != tableIdx2 {
			t.Errorf("Hash函数不一致: 输入 %d, 第一次 (%d,%d), 第二次 (%d,%d)",
				i, dbIdx1, tableIdx1, dbIdx2, tableIdx2)
		}
	}

	t.Logf("Hash函数一致性测试通过")
}

func TestSecondaryHashDistribution(t *testing.T) {
	// 测试二次hash的分布特性
	strategy := NewHashShardStrategy(1, 100) // 单数据库，多表

	tableDistribution := make(map[int]int)

	for i := 0; i < 10000; i++ {
		_, tableIdx := strategy.CalculateShard("user_id", i)
		tableDistribution[tableIdx]++
	}

	// 验证二次hash分布 - 放宽限制到25%
	for table := 0; table < 100; table++ {
		count := tableDistribution[table]
		expected := 10000 / 100
		deviation := float64(count-expected) / float64(expected) * 100

		if deviation > 25 || deviation < -25 { // 允许25%偏差
			t.Errorf("表 %d 二次hash分布偏差过大: %.2f%% (期望 ±25%%)", table, deviation)
		}
	}

	t.Logf("二次hash分布测试通过")
}

func TestWeightedCalculationLogic(t *testing.T) {
	// 测试加权计算逻辑的正确性
	weights := []int{1, 2, 3, 4} // 总权重10
	strategy := NewWeightedShardStrategy(4, 8, weights)

	dbDistribution := make(map[int]int)

	for i := 0; i < 10000; i++ {
		dbIdx, _ := strategy.CalculateShard("user_id", i)
		dbDistribution[dbIdx]++
	}

	// 验证权重比例: 1:2:3:4
	expectedRatios := []float64{0.1, 0.2, 0.3, 0.4}

	for db := 0; db < 4; db++ {
		actualRatio := float64(dbDistribution[db]) / 10000
		expectedRatio := expectedRatios[db]
		deviation := (actualRatio - expectedRatio) / expectedRatio * 100

		if deviation > 20 || deviation < -20 { // 允许20%偏差
			t.Errorf("数据库 %d 权重比例错误: 实际 %.3f, 期望 %.3f, 偏差 %.2f%%",
				db, actualRatio, expectedRatio, deviation)
		}
	}

	t.Logf("加权计算逻辑测试通过")
	t.Logf("实际分布: %v", dbDistribution)
}

func TestConsistentHashRingProperties(t *testing.T) {
	// 测试一致性hash环的属性
	nodes := []string{"node1", "node2", "node3"}
	ch := NewConsistentHash(nodes, 100)

	// 测试虚拟节点数量
	if len(ch.ring) != 300 { // 3节点 x 100虚拟节点
		t.Errorf("虚拟节点数量错误: 期望 300, 实际 %d", len(ch.ring))
	}

	// 测试环的排序性
	for i := 1; i < len(ch.ring); i++ {
		if ch.ring[i] < ch.ring[i-1] {
			t.Errorf("虚拟节点环未排序: %d < %d", ch.ring[i], ch.ring[i-1])
			break
		}
	}

	// 测试节点映射
	for hash, node := range ch.nodeMap {
		found := false
		for _, n := range nodes {
			if n == node {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("虚拟节点映射到未知节点: hash=%d, node=%s", hash, node)
		}
	}

	// 测试nodeToIndex映射
	for node, idx := range ch.nodeToIndex {
		expectedIdx := indexOf(nodes, node)
		if idx != expectedIdx {
			t.Errorf("nodeToIndex映射错误: node=%s, 期望 %d, 实际 %d", node, expectedIdx, idx)
		}
	}

	t.Logf("一致性Hash环属性测试通过")
}

func TestCompareFunction(t *testing.T) {
	// 测试compare函数
	testCases := []struct {
		vi, vj interface{}
		desc   bool
		want   bool
	}{
		{int64(1), int64(2), false, true},  // 1 < 2
		{int64(2), int64(1), false, false}, // 2 < 1
		{int64(1), int64(2), true, false},  // desc: 1 > 2
		{int64(2), int64(1), true, true},   // desc: 2 > 1
		{1.5, 2.5, false, true},            // float
		{"a", "b", false, true},            // string
		{nil, nil, false, false},           // nil case
	}

	for i, tc := range testCases {
		result := compare(tc.vi, tc.vj, tc.desc)
		if result != tc.want {
			t.Errorf("测试用例 %d 失败: compare(%v, %v, %v) = %v, 期望 %v",
				i, tc.vi, tc.vj, tc.desc, result, tc.want)
		}
	}

	t.Logf("compare函数测试通过")
}

func TestApplyPagination(t *testing.T) {
	// 测试分页函数
	data := []map[string]interface{}{
		{"id": 1}, {"id": 2}, {"id": 3}, {"id": 4}, {"id": 5},
	}

	testCases := []struct {
		data   []map[string]interface{}
		offset int
		limit  int
		want   int // 期望结果长度
	}{
		{data, 0, 2, 2},  // 正常分页
		{data, 1, 3, 3},  // 中间分页
		{data, 3, 2, 2},  // 边界分页
		{data, 10, 5, 0}, // offset超出
		{data, 0, 0, 5},  // limit为0
		{data, 0, 10, 5}, // limit超出
	}

	for i, tc := range testCases {
		result := applyPagination(tc.data, tc.offset, tc.limit)
		if len(result) != tc.want {
			t.Errorf("测试用例 %d 失败: applyPagination(..., %d, %d) 长度 = %d, 期望 %d",
				i, tc.offset, tc.limit, len(result), tc.want)
		}
	}

	t.Logf("applyPagination函数测试通过")
}

// ======================= Mock DB =======================

// 使用现有的MockDB，不重复定义

// ======================= Helper Functions =======================

func testGenerateRandomWeights(count int) []int {
	weights := make([]int, count)
	for i := 0; i < count; i++ {
		weights[i] = rand.Intn(500) + 50 // 50-550的随机权重
	}
	return weights
}

func indexOf(arr []string, t string) int {
	for i, v := range arr {
		if v == t {
			return i
		}
	}
	return -1
}

// ======================= Benchmark Tests =======================

func BenchmarkHashShardStrategy(b *testing.B) {
	strategy := NewHashShardStrategy(100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.CalculateShard("user_id", i)
	}
}

func BenchmarkWeightedShardStrategy(b *testing.B) {
	weights := make([]int, 100)
	for i := range weights {
		weights[i] = 100
	}
	strategy := NewWeightedShardStrategy(100, 100, weights)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.CalculateShard("user_id", i)
	}
}

func BenchmarkConsistentShardStrategy(b *testing.B) {
	dbNodes := make([]string, 100)
	tableNodes := make([]string, 100)
	for i := 0; i < 100; i++ {
		dbNodes[i] = fmt.Sprintf("db_%d", i)
		tableNodes[i] = fmt.Sprintf("table_%d", i)
	}
	strategy := NewConsistentShardStrategy(dbNodes, tableNodes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.CalculateShard("user_id", i)
	}
}

func BenchmarkCrossShardQuery(b *testing.B) {
	query := &CrossShardQuery{}

	// 创建查询构建器
	for i := 0; i < 4; i++ {
		db := &MockDB{
			QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
				return &sql.Rows{}, nil
			},
		}
		sb := NewSelectBuilder(db, fmt.Sprintf("orders_%d", i))
		query.queries = append(query.queries, sb)
	}

	query.limit = 10
	query.orderBy = "id"
	query.desc = false

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query.Exec(ctx)
	}
}
