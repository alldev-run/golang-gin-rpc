package mw

import (
	"net/http"
	
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
)

func init() {
	Register("tracing", 50, Tracing())
}

func Tracing() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果全局追踪器已启用，添加追踪信息
			if tracer := tracing.GlobalTracer(); tracer.IsEnabled() {
				// 从请求中提取追踪信息
				ctx := tracing.ExtractHeaders(r.Context(), r.Header)
				r = r.WithContext(ctx)
				
				// 在响应头中添加追踪信息
				traceID := tracing.GetTraceID(ctx)
				spanID := tracing.GetSpanID(ctx)
				if traceID != "" {
					w.Header().Set("X-Trace-ID", traceID)
				}
				if spanID != "" {
					w.Header().Set("X-Span-ID", spanID)
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}
