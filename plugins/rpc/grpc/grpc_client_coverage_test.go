package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestClientPoolAndHealthCheckerLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	config := ClientConfig{Timeout: 2 * time.Second, MaxMsgSize: 1024}
	pool := NewClientPool(config)
	client, err := pool.Get(listener.Addr().String())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if reused, err := pool.Get(listener.Addr().String()); err != nil || reused != client {
		t.Fatalf("Get() reused client = %v, %v", reused, err)
	}
	if client.Address() != listener.Addr().String() || client.IsSecure() || client.Conn() == nil {
		t.Fatalf("client accessors returned unexpected values")
	}
	if err := (NewHealthChecker(client)).CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.WaitForReady(ctx); err != nil {
		t.Fatalf("WaitForReady() error = %v", err)
	}
	if pool.Size() != 1 || len(pool.Addresses()) != 1 {
		t.Fatalf("pool size/addresses = %d/%v", pool.Size(), pool.Addresses())
	}
	if err := pool.Remove(listener.Addr().String()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if pool.Size() != 0 {
		t.Fatalf("pool size after Remove() = %d", pool.Size())
	}
	if err := (NewHealthChecker(client)).CheckHealth(context.Background()); err == nil {
		t.Fatal("CheckHealth() succeeded after the client was closed")
	}
	if err := pool.Remove("missing"); err != nil {
		t.Fatalf("Remove() for missing client error = %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestClientConnectionFailure(t *testing.T) {
	_, err := NewClient(ClientConfig{Address: "127.0.0.1:0", Timeout: 20 * time.Millisecond})
	if err == nil {
		t.Fatal("NewClient() succeeded for an unreachable endpoint")
	}
}
