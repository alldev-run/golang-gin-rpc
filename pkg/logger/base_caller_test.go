package logger

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestBaseAndWrapperReportCallSite(t *testing.T) {
	prevL, prevBase := defaultL.Load(), defaultBase.Load()
	t.Cleanup(func() { defaultL.Store(prevL); defaultBase.Store(prevBase) })

	core, logs := observer.New(zapcore.InfoLevel)
	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	defaultL.Store(l)
	defaultBase.Store(l.WithOptions(zap.AddCallerSkip(-1)))

	Info("wrapper")
	Base().Info("base")

	for _, e := range logs.All() {
		if e.Caller.File == "" || !e.Caller.Defined {
			t.Fatalf("%s: caller undefined", e.Message)
		}
		if got := e.Caller.TrimmedPath(); got[:len("logger/base_caller_test.go")] != "logger/base_caller_test.go" {
			t.Fatalf("%s: caller = %s, want this test file", e.Message, got)
		}
	}
}
