package gateway

import "strings"

func normalizeGinRoutePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	// gin requires named wildcards, e.g. /*path not /*
	if strings.HasSuffix(p, "/*") {
		return p + "path"
	}
	return p
}
