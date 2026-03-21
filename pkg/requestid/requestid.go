package requestid

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"strings"
	"time"
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// New returns a distributed-friendly request id.
// Format: base32(no padding) encoding of 16 bytes:
//  - 6 bytes: timestamp in milliseconds (big endian)
//  - 10 bytes: cryptographically secure randomness
// Output length is 26 chars.
func New() (string, error) {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint64(b[0:8], ms)
	copy(b[0:6], b[2:8])
	if _, err := rand.Read(b[6:]); err != nil {
		return "", err
	}
	id := encoding.EncodeToString(b[:])
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
