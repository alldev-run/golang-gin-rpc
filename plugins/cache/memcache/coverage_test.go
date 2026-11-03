package memcache

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientOperationsAgainstLocalServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	var mu sync.Mutex
	items := make(map[string][]byte)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveMemcacheTestConn(conn, &mu, items)
		}
	}()

	client, err := New(Config{Hosts: []string{listener.Addr().String()}, Timeout: time.Second})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()
	ctx := context.Background()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if err := client.Set(ctx, &Item{Key: "key", Value: []byte("value")}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := client.Get(ctx, "key")
	if err != nil || string(got.Value) != "value" {
		t.Fatalf("Get() = %v, %v", got, err)
	}
	multi, err := client.GetMulti(ctx, []string{"key", "missing"})
	if err != nil || len(multi) != 1 || string(multi["key"].Value) != "value" {
		t.Fatalf("GetMulti() = %v, %v", multi, err)
	}
	if err := client.Add(ctx, &Item{Key: "key", Value: []byte("other")}); err == nil {
		t.Fatal("Add() succeeded for an existing key")
	}
	if err := client.Replace(ctx, &Item{Key: "key", Value: []byte("updated")}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if err := client.Set(ctx, &Item{Key: "counter", Value: []byte("10")}); err != nil {
		t.Fatal(err)
	}
	if value, err := client.Increment(ctx, "counter", 1); err != nil || value != 11 {
		t.Fatalf("Increment() = %d, %v", value, err)
	}
	if value, err := client.Decrement(ctx, "counter", 1); err != nil || value != 10 {
		t.Fatalf("Decrement() = %d, %v", value, err)
	}
	if err := client.Touch(ctx, "key", 60); err != nil {
		t.Fatalf("Touch() error = %v", err)
	}
	if err := client.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := client.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	if _, err := client.Get(ctx, "missing"); err == nil {
		t.Fatal("Get() succeeded for a missing key")
	}
}

func serveMemcacheTestConn(conn net.Conn, mu *sync.Mutex, items map[string][]byte) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "version":
			_, _ = io.WriteString(conn, "VERSION 1.6.0\r\n")
		case "set", "add", "replace":
			size, _ := strconv.Atoi(fields[4])
			value := make([]byte, size+2)
			if _, err := io.ReadFull(reader, value); err != nil {
				return
			}
			mu.Lock()
			_, exists := items[fields[1]]
			switch fields[0] {
			case "add":
				if exists {
					mu.Unlock()
					_, _ = io.WriteString(conn, "NOT_STORED\r\n")
					continue
				}
			case "replace":
				if !exists {
					mu.Unlock()
					_, _ = io.WriteString(conn, "NOT_STORED\r\n")
					continue
				}
			}
			items[fields[1]] = append([]byte(nil), value[:size]...)
			mu.Unlock()
			_, _ = io.WriteString(conn, "STORED\r\n")
		case "get", "gets":
			mu.Lock()
			for _, key := range fields[1:] {
				if value, ok := items[key]; ok {
					_, _ = fmt.Fprintf(conn, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
				}
			}
			mu.Unlock()
			_, _ = io.WriteString(conn, "END\r\n")
		case "delete", "touch":
			mu.Lock()
			_, exists := items[fields[1]]
			if fields[0] == "delete" {
				delete(items, fields[1])
			}
			mu.Unlock()
			if exists {
				if fields[0] == "delete" {
					_, _ = io.WriteString(conn, "DELETED\r\n")
				} else {
					_, _ = io.WriteString(conn, "TOUCHED\r\n")
				}
			} else {
				_, _ = io.WriteString(conn, "NOT_FOUND\r\n")
			}
		case "incr", "decr":
			mu.Lock()
			value, exists := items[fields[1]]
			if !exists {
				mu.Unlock()
				_, _ = io.WriteString(conn, "NOT_FOUND\r\n")
				continue
			}
			number, _ := strconv.ParseUint(string(value), 10, 64)
			delta, _ := strconv.ParseUint(fields[2], 10, 64)
			if fields[0] == "incr" {
				number += delta
			} else if delta > number {
				number = 0
			} else {
				number -= delta
			}
			items[fields[1]] = []byte(strconv.FormatUint(number, 10))
			mu.Unlock()
			_, _ = fmt.Fprintf(conn, "%d\r\n", number)
		case "flush_all":
			mu.Lock()
			clear(items)
			mu.Unlock()
			_, _ = io.WriteString(conn, "OK\r\n")
		default:
			_, _ = io.WriteString(conn, "ERROR\r\n")
		}
	}
}
