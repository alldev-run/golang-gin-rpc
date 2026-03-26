package orm

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStructScan_StringConversion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	// 模拟返回中文字符串（数据库返回 []byte）
	rows := sqlmock.NewRows([]string{"title"}).AddRow([]byte("测试标题"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	type Item struct {
		Title string `db:"title"`
	}

	row, err := db.Query("SELECT title FROM items")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer row.Close()

	var item Item
	if err := StructScan(row, &item); err != nil {
		t.Fatalf("StructScan failed: %v", err)
	}

	if item.Title != "测试标题" {
		t.Errorf("expected '测试标题', got '%s'", item.Title)
	}
}

func TestStructScanAll_MultipleRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	// 模拟多行数据
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "张三").
		AddRow(2, "李四").
		AddRow(3, "王五")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	rs, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rs.Close()

	var users []*User
	if err := StructScanAll(rs, &users); err != nil {
		t.Fatalf("StructScanAll failed: %v", err)
	}

	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}

	// 验证每行数据独立（检查Bug 1修复）
	expected := []struct {
		id   int
		name string
	}{
		{1, "张三"},
		{2, "李四"},
		{3, "王五"},
	}

	for i, u := range users {
		if u.ID != expected[i].id {
			t.Errorf("user[%d].ID: expected %d, got %d", i, expected[i].id, u.ID)
		}
		if u.Name != expected[i].name {
			t.Errorf("user[%d].Name: expected '%s', got '%s'", i, expected[i].name, u.Name)
		}
	}
}

func TestSetFieldValue_TypeConversions(t *testing.T) {
	tests := []struct {
		name     string
		field    interface{}
		value    interface{}
		expected interface{}
	}{
		{"[]byte to string", new(string), []byte("hello"), "hello"},
		{"string to string", new(string), "world", "world"},
		{"int64 to int", new(int), int64(42), int(42)},
		{"float64 to int", new(int), float64(100), int(100)},
		{"[]byte to int", new(int), []byte("123"), int(123)},
		{"bool to bool", new(bool), true, true},
		{"int64 to bool", new(bool), int64(1), true},
		{"float64 to float", new(float64), float64(3.14), float64(3.14)},
		{"nil value", new(string), nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflect.ValueOf(tt.field).Elem()
			if err := setFieldValue(field, tt.value); err != nil {
				t.Fatalf("setFieldValue failed: %v", err)
			}

			actual := field.Interface()
			if actual != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, actual, actual)
			}
		})
	}
}

func TestFieldCache(t *testing.T) {
	type TestStruct struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	// 第一次获取，创建缓存
	meta1 := getStructMeta(reflect.TypeOf(TestStruct{}))
	if meta1 == nil || len(meta1.fieldMap) != 2 {
		t.Errorf("expected 2 fields in cache, got %v", meta1)
	}

	// 第二次获取，从缓存读取
	meta2 := getStructMeta(reflect.TypeOf(TestStruct{}))
	if meta1 != meta2 {
		t.Error("expected same cached structMeta instance")
	}
}
