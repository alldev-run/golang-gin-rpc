package orm

import (
	"reflect"
	"testing"
)

func TestSelectBuilder_WithCTE(t *testing.T) {
	mockDB := &MockDB{}

	cte := NewSelectBuilder(mockDB, "orders").
		Columns("user_id", "COUNT(*)").
		Where("status = ?", "paid").
		GroupBy("user_id")

	main := NewSelectBuilder(mockDB, "active_users").
		With("active_users", cte).
		Columns("user_id").
		Where("COUNT(*) > ?", 10)

	q, args := main.Build()
	wantQ := "WITH `active_users` AS (SELECT `user_id`, COUNT(*) FROM `orders` WHERE status = ? GROUP BY `user_id`) SELECT `user_id` FROM `active_users` WHERE COUNT(*) > ?"
	wantArgs := []interface{}{"paid", 10}

	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}

func TestSelectBuilder_FromSubquery(t *testing.T) {
	mockDB := &MockDB{}

	sub := NewSelectBuilder(mockDB, "users").
		Columns("id").
		Where("status = ?", "active")

	sb := NewSelectBuilder(mockDB, "ignored").
		FromSubquery(sub, "u").
		Columns("u.id")

	q, args := sb.Build()
	wantQ := "SELECT `u`.`id` FROM (SELECT `id` FROM `users` WHERE status = ?) u"
	wantArgs := []interface{}{"active"}

	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}

func TestSelectBuilder_UnionAll(t *testing.T) {
	mockDB := &MockDB{}

	a := NewSelectBuilder(mockDB, "users").Columns("id").Where("status = ?", "active")
	b := NewSelectBuilder(mockDB, "admins").Columns("id").Where("enabled = ?", true)

	sb := a.UnionAll(b)
	q, args := sb.Build()
	wantQ := "SELECT `id` FROM `users` WHERE status = ? UNION ALL SELECT `id` FROM `admins` WHERE enabled = ?"
	wantArgs := []interface{}{"active", true}

	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}
