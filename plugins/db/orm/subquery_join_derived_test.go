package orm

import (
	"reflect"
	"testing"
)

func TestWhereBuilder_SubqueryPredicates(t *testing.T) {
	mockDB := &MockDB{}

	sub := NewSelectBuilder(mockDB, "orders").
		Columns("1").
		Where("user_id = ?", 10).
		And("status = ?", "paid")

	sb := NewSelectBuilder(mockDB, "users").
		Columns("id")
	sb.WhereBuilder().ExistsSubquery(sub)

	q, args := sb.Build()
	wantQ := "SELECT `id` FROM `users` WHERE EXISTS (SELECT `1` FROM `orders` WHERE user_id = ? AND status = ?)"
	wantArgs := []interface{}{10, "paid"}

	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}

func TestSelectBuilder_AsDerived_UnionOuterOrderLimit(t *testing.T) {
	mockDB := &MockDB{}

	a := NewSelectBuilder(mockDB, "users").Columns("id").Where("status = ?", "active")
	b := NewSelectBuilder(mockDB, "admins").Columns("id").Where("enabled = ?", true)

	outer := a.UnionAll(b).AsDerived("t").
		Columns("t.id").
		OrderByDesc("t.id").
		Limit(10)

	q, args := outer.Build()
	wantQ := "SELECT `t`.`id` FROM (SELECT `id` FROM `users` WHERE status = ? UNION ALL SELECT `id` FROM `admins` WHERE enabled = ?) t ORDER BY `t`.`id` DESC LIMIT 10"
	wantArgs := []interface{}{"active", true}

	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}

func TestSelectBuilder_WithRecursive(t *testing.T) {
	mockDB := &MockDB{}

	seed := NewSelectBuilder(mockDB, "nodes").Columns("id", "parent_id").Where("id = ?", 1)
	rec := NewSelectBuilder(mockDB, "nodes").Columns("n.id", "n.parent_id").Join("tree t", "n.parent_id = t.id").FromRaw("nodes n")

	sb := NewSelectBuilder(mockDB, "tree").
		WithRecursive("tree", seed, rec).
		Columns("id").
		FromRaw("tree")

	q, args := sb.Build()
	wantQ := "WITH RECURSIVE `tree` AS (SELECT `id`, `parent_id` FROM `nodes` WHERE id = ? UNION ALL SELECT `n`.`id`, `n`.`parent_id` FROM nodes n INNER JOIN `tree` t ON n.parent_id = t.id) SELECT `id` FROM tree"
	wantArgs := []interface{}{1}
	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}

func TestSelectBuilder_JoinSubquery(t *testing.T) {
	mockDB := &MockDB{}

	sub := NewSelectBuilder(mockDB, "orders").Columns("user_id").Where("status = ?", "paid")
	sb := NewSelectBuilder(mockDB, "users u").
		Columns("u.id").
		JoinSubquery(sub, "o", "o.user_id = u.id")

	q, args := sb.Build()
	wantQ := "SELECT `u`.`id` FROM `users` u INNER JOIN (SELECT `user_id` FROM `orders` WHERE status = ?) o ON o.user_id = u.id"
	wantArgs := []interface{}{"paid"}
	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args. want %v got %v", wantArgs, args)
	}
}
