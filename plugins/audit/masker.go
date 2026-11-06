package audit

import "strings"

const maskedValue = "***"

// Masker performs best-effort sensitive field masking.
type Masker struct {
	sensitiveKeys map[string]struct{}
}

// NewMasker creates a field masker with case-insensitive key matching.
func NewMasker(keys []string) *Masker {
	m := &Masker{sensitiveKeys: make(map[string]struct{})}
	for _, k := range keys {
		if k == "" {
			continue
		}
		m.sensitiveKeys[strings.ToLower(k)] = struct{}{}
	}
	return m
}

// MaskMap masks sensitive keys in a map and returns a copied result.
func (m *Masker) MaskMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for k, v := range input {
		if _, ok := m.sensitiveKeys[strings.ToLower(k)]; ok {
			out[k] = maskedValue
			continue
		}
		out[k] = v
	}
	return out
}
