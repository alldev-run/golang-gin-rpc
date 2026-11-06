package orm

import "testing"

func TestMySQLSemantics_DefaultDialectDoesNotEmitFullOuterJoin(t *testing.T) {
	mockDB := &MockDB{}

	sb := NewSelectBuilder(mockDB, "users").
		FullOuterJoin("profiles", "users.id = profiles.user_id")

	q, _ := sb.Build()
	want := "SELECT * FROM `users` LEFT JOIN `profiles` ON users.id = profiles.user_id"
	if q != want {
		t.Fatalf("unexpected query.\nwant: %s\n got: %s", want, q)
	}
}

func TestMySQLSemantics_QuoteIdentifier_CommonCases(t *testing.T) {
	d := NewDefaultDialect() // default is MySQL-compatible

	cases := map[string]string{
		"id":       "`id`",
		"u.id":     "`u`.`id`",
		"users u":  "`users` u",
		"COUNT(*)": "COUNT(*)",
	}

	for in, want := range cases {
		if got := d.QuoteIdentifier(in); got != want {
			t.Fatalf("QuoteIdentifier(%q) want %q got %q", in, want, got)
		}
	}
}

func TestMySQLSemantics_InsertVariants(t *testing.T) {
	mockDB := &MockDB{}

	q1, _, err := NewInsertBuilder(mockDB, "users").Ignore().Set("id", 1).Set("name", "a").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q1 != "INSERT IGNORE INTO `users` (`id`, `name`) VALUES (?, ?)" {
		t.Fatalf("unexpected ignore insert: %s", q1)
	}

	q2, _, err := NewInsertBuilder(mockDB, "users").Replace().Set("id", 1).Set("name", "a").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q2 != "REPLACE INTO `users` (`id`, `name`) VALUES (?, ?)" {
		t.Fatalf("unexpected replace insert: %s", q2)
	}

	q3, _, err := NewInsertBuilder(mockDB, "users").Set("id", 1).Set("name", "a").OnDuplicateKeyUpdate("name").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q3 != "INSERT INTO `users` (`id`, `name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)" {
		t.Fatalf("unexpected upsert: %s", q3)
	}
}
