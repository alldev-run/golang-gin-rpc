package orm

import (
	"reflect"
	"testing"
)

func TestJoinOnBuilder_Basic(t *testing.T) {
	jb := NewJoinOnBuilder(NewMySQLDialect())
	jb.Eq("u.id", "p.user_id").And("p.status = ?", "active")
	cond, args := jb.Build()
	if cond != "`u`.`id` = `p`.`user_id` AND p.status = ?" {
		t.Fatalf("unexpected cond: %s", cond)
	}
	if !reflect.DeepEqual(args, []interface{}{"active"}) {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestSelectBuilder_JoinOnBuilder(t *testing.T) {
	mockDB := &MockDB{}
	sb := NewSelectBuilder(mockDB, "users u").
		Columns("u.id").
		JoinOn("profiles p", func(on *JoinOnBuilder) {
			on.Eq("u.id", "p.user_id").And("p.status = ?", "active")
		})

	q, args := sb.Build()
	wantQ := "SELECT `u`.`id` FROM `users` u INNER JOIN `profiles` p ON `u`.`id` = `p`.`user_id` AND p.status = ?"
	wantArgs := []interface{}{"active"}
	if q != wantQ {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", wantQ, q)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: %v", args)
	}
}
