package mongodb

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// offlineMongoClient builds a mongo.Client that is never expected to reach a
// server. mongo.Connect is lazy, so handle-returning methods work offline and
// every network operation fails once the (short) context expires.
func offlineMongoClient(t *testing.T) (*Client, *mongo.Client) {
	t.Helper()
	client, err := mongo.Connect(options.Client().
		ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=250&connectTimeoutMS=250"))
	if err != nil {
		t.Fatalf("mongo.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		_ = client.Disconnect(context.Background())
	})
	return newClientWithClient(client, Config{
		URI:      "mongodb://127.0.0.1:1",
		Database: "testdb",
	}), client
}

// shortCtx returns a context that fails fast on unreachable servers.
func shortCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(t.Context(), 3*time.Second)
}

// ==================== Offline-safe handle methods ====================

func TestClient_Database(t *testing.T) {
	client, _ := offlineMongoClient(t)

	db := client.Database()
	if db == nil {
		t.Fatal("Database() returned nil")
	}
	if got := db.Name(); got != "testdb" {
		t.Errorf("Database().Name() = %q, want testdb", got)
	}
}

func TestClient_Collection(t *testing.T) {
	client, _ := offlineMongoClient(t)

	coll := client.Collection("users")
	if coll == nil {
		t.Fatal("Collection() returned nil")
	}
	if got := coll.Name(); got != "users" {
		t.Errorf("Collection().Name() = %q, want users", got)
	}
}

func TestClient_CollectionFromDB(t *testing.T) {
	client, _ := offlineMongoClient(t)

	coll := client.CollectionFromDB("other", "logs")
	if coll == nil {
		t.Fatal("CollectionFromDB() returned nil")
	}
	if got := coll.Name(); got != "logs" {
		t.Errorf("CollectionFromDB().Name() = %q, want logs", got)
	}
	if got := coll.Database().Name(); got != "other" {
		t.Errorf("CollectionFromDB().Database().Name() = %q, want other", got)
	}
}

func TestClient_ClientAccessor(t *testing.T) {
	client, raw := offlineMongoClient(t)

	if got := client.Client(); got != raw {
		t.Error("Client() should return the underlying mongo.Client")
	}
}

func TestClient_Ping_Unreachable(t *testing.T) {
	client, _ := offlineMongoClient(t)

	ctx, cancel := shortCtx(t)
	defer cancel()

	if err := client.Ping(ctx); err == nil {
		t.Error("Ping() against an unreachable server should fail")
	}
}

func TestClient_ServerOperations_Unreachable(t *testing.T) {
	client, _ := offlineMongoClient(t)

	ctx, cancel := shortCtx(t)
	defer cancel()

	if _, err := client.ListDatabases(ctx); err == nil {
		t.Error("ListDatabases() against an unreachable server should fail")
	}

	ctx2, cancel2 := shortCtx(t)
	defer cancel2()
	if _, err := client.ListCollections(ctx2); err == nil {
		t.Error("ListCollections() against an unreachable server should fail")
	}

	ctx3, cancel3 := shortCtx(t)
	defer cancel3()
	if err := client.CreateCollection(ctx3, "things"); err == nil {
		t.Error("CreateCollection() against an unreachable server should fail")
	}

	ctx4, cancel4 := shortCtx(t)
	defer cancel4()
	if err := client.DropCollection(ctx4, "things"); err == nil {
		t.Error("DropCollection() against an unreachable server should fail")
	}
}

func TestDbClient_AdapterPingAndClose(t *testing.T) {
	client, _ := offlineMongoClient(t)
	adapter := &dbClient{client: client}

	ctx, cancel := shortCtx(t)
	defer cancel()
	if err := adapter.Ping(ctx); err == nil {
		t.Error("dbClient.Ping() against an unreachable server should fail")
	}
	// Close on a lazily-connected client disconnects without error.
	if err := adapter.Close(); err != nil {
		t.Errorf("dbClient.Close() error = %v, want nil", err)
	}
}

// ==================== New() config fallbacks ====================

func TestNew_ConfigDefaultsApplied(t *testing.T) {
	// Empty database name falls back to the default; the constructor fails at
	// ping (no server), so assert the error shape only.
	cfg := Config{URI: "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=250&connectTimeoutMS=250"}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() without a server should fail")
	}
}

// ==================== Integration (opt-in) ====================

// TestClient_Integration exercises the full happy path against a real server.
// Opt in with MONGODB_INTEGRATION=1 and optionally MONGODB_URI.
func TestClient_Integration(t *testing.T) {
	if os.Getenv("MONGODB_INTEGRATION") != "1" {
		t.Skip("set MONGODB_INTEGRATION=1 and MONGODB_URI to run MongoDB integration tests")
	}
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	client, err := New(Config{URI: uri, Database: "goframe_it"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = client.Close(t.Context()) }()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	if err := client.CreateCollection(ctx, "items"); err != nil {
		t.Errorf("CreateCollection() error = %v", err)
	}
	names, err := client.ListCollections(ctx)
	if err != nil {
		t.Fatalf("ListCollections() error = %v", err)
	}
	found := false
	for _, name := range names {
		if name == "items" {
			found = true
		}
	}
	if !found {
		t.Errorf("collection %q not found in %v", "items", names)
	}

	if _, err := client.ListDatabases(ctx); err != nil {
		t.Errorf("ListDatabases() error = %v", err)
	}

	if err := client.DropCollection(ctx, "items"); err != nil {
		t.Errorf("DropCollection() error = %v", err)
	}
}
