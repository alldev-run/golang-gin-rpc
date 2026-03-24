package requestid

import (
	"crypto/rand"
	"encoding/base32"
	"io"
	"strings"
	"sync"
	"time"
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// 可替换随机源（方便测试 & fallback）
var randReader io.Reader = rand.Reader

// buffer 池（减少 GC 压力）
var bytePool = sync.Pool{
	New: func() any {
		b := make([]byte, 16)
		return &b
	},
}

// New returns a distributed-friendly request id.
func New() (string, error) {
	// 从池中取 buffer
	bufPtr := bytePool.Get().(*[]byte)
	b := *bufPtr
	defer bytePool.Put(bufPtr)

	// 清零（防止复用污染）
	for i := range b {
		b[i] = 0
	}

	// === 1. 写入 6 字节时间戳（毫秒）===
	ms := uint64(time.Now().UnixMilli())

	// 只取低 6 字节（更清晰写法）
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// === 2. 写入 10 字节随机数 ===
	if _, err := io.ReadFull(randReader, b[6:]); err != nil {
		return "", err
	}

	// === 3. 编码 ===
	id := encoding.EncodeToString(b)

	// === 4. 小写 ===
	return strings.ToLower(id), nil
}

// MustNew returns a request id or empty string on failure.
func MustNew() string {
	id, err := New()
	if err != nil {
		return ""
	}
	return id
}
