// loadbot 是 WebSocket 机器人压测工具:批量建立连接,按速率发送
// envelope 请求并统计 RTT,用于验证网关/游戏服的连接与消息承载能力。
//
// 用法:
//
//	go run ./cmd/loadbot --url ws://localhost:8091/ws --bots 100 --rate 10 --duration 30s --action echo
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/websocket"
)

type stats struct {
	mu        sync.Mutex
	rtts      []time.Duration
	sent      atomic.Int64
	received  atomic.Int64
	errors    atomic.Int64
	connected atomic.Int64
}

func (s *stats) recordRTT(d time.Duration) {
	s.mu.Lock()
	s.rtts = append(s.rtts, d)
	s.mu.Unlock()
	s.received.Add(1)
}

func (s *stats) percentile(p float64) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rtts) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), s.rtts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func (s *stats) avg() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rtts) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range s.rtts {
		total += d
	}
	return total / time.Duration(len(s.rtts))
}

// bot 是单个压测机器人:一条连接 + pending 请求表。
type bot struct {
	client  *websocket.Client
	pending sync.Map // reqID -> chan time
}

func (b *bot) readLoop(ctx context.Context) {
	for {
		msgType, payload, err := b.client.Receive(ctx)
		if err != nil {
			return
		}
		if msgType != 1 { // 只处理文本帧 envelope
			continue
		}
		var env struct {
			Action string          `json:"action"`
			ReqID  string          `json:"req_id"`
			Code   int             `json:"code"`
			Data   json.RawMessage `json:"data"`
		}
		if json.Unmarshal(payload, &env) != nil || env.ReqID == "" {
			continue
		}
		if v, ok := b.pending.LoadAndDelete(env.ReqID); ok {
			if ch, ok := v.(chan time.Time); ok {
				ch <- time.Now()
				close(ch)
			}
		}
	}
}

func (b *bot) run(ctx context.Context, url, action string, rate int, st *stats, rng *rand.Rand) {
	cfg := websocket.Config{URL: url, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
	client := websocket.NewClient(cfg)
	b.client = client
	if err := client.Connect(ctx); err != nil {
		st.errors.Add(1)
		return
	}
	defer client.Close()
	st.connected.Add(1)

	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()
	go b.readLoop(readCtx)

	interval := time.Second / time.Duration(max(rate, 1))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	seq := atomic.Int64{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reqID := fmt.Sprintf("bot-%d-%d", st.connected.Load(), seq.Add(1))
			ch := make(chan time.Time, 1)
			b.pending.Store(reqID, ch)
			start := time.Now()
			env := map[string]interface{}{
				"action": action,
				"req_id": reqID,
				"data":   map[string]interface{}{"n": rng.Intn(1000)},
			}
			raw, _ := json.Marshal(env)
			if err := client.SendText(ctx, string(raw)); err != nil {
				st.errors.Add(1)
				b.pending.Delete(reqID)
				continue
			}
			st.sent.Add(1)
			select {
			case done := <-ch:
				st.recordRTT(done.Sub(start))
			case <-time.After(5 * time.Second):
				st.errors.Add(1)
				b.pending.Delete(reqID)
			}
		}
	}
}

func main() {
	url := flag.String("url", "ws://localhost:8091/ws", "websocket endpoint")
	bots := flag.Int("bots", 10, "number of concurrent bots")
	rate := flag.Int("rate", 5, "requests per second per bot")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	action := flag.String("action", "echo", "action to invoke in envelope")
	flag.Parse()

	logger.Init(logger.DefaultConfig())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()

	st := &stats{}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < *bots; i++ {
		wg.Add(1)
		b := &bot{}
		go func() {
			defer wg.Done()
			b.run(ctx, *url, *action, *rate, st, rng)
		}()
		// 错峰建连,避免握手风暴干扰测量。
		time.Sleep(time.Duration(min(50, 1000/max(*bots, 1))) * time.Millisecond)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Println("==== loadbot 结果 ====")
	fmt.Printf("时长:      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("机器人:    %d(成功连接 %d)\n", *bots, st.connected.Load())
	fmt.Printf("发送:      %d\n", st.sent.Load())
	fmt.Printf("收到响应:  %d\n", st.received.Load())
	fmt.Printf("错误:      %d\n", st.errors.Load())
	if st.sent.Load() > 0 {
		fmt.Printf("吞吐:      %.1f req/s\n", float64(st.sent.Load())/elapsed.Seconds())
	}
	fmt.Printf("RTT avg:   %s\n", st.avg().Round(time.Microsecond))
	fmt.Printf("RTT p50:   %s\n", st.percentile(0.50).Round(time.Microsecond))
	fmt.Printf("RTT p95:   %s\n", st.percentile(0.95).Round(time.Microsecond))
	fmt.Printf("RTT p99:   %s\n", st.percentile(0.99).Round(time.Microsecond))
	fmt.Printf("RTT max:   %s\n", st.percentile(1.0).Round(time.Microsecond))
}
