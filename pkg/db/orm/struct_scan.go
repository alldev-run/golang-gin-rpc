package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
)

// --- 新增：结构体元数据缓存，避免高并发下重复反射 ---
var fieldCache sync.Map

type structMeta struct {
	fieldMap map[string]int
}

func getStructMeta(t reflect.Type) *structMeta {
	if val, ok := fieldCache.Load(t); ok {
		return val.(*structMeta)
	}
	meta := &structMeta{fieldMap: make(map[string]int)}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("db")
		if tag != "" {
			meta.fieldMap[tag] = i
		} else {
			meta.fieldMap[ToSnakeCase(f.Name)] = i
		}
	}
	fieldCache.Store(t, meta)
	return meta
}

// --- 保持原方法名不变 ---

func StructScan(rows *sql.Rows, dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("dest must be a non-nil pointer to a struct")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return errors.New("dest must point to a struct")
	}

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	if !rows.Next() {
		return sql.ErrNoRows
	}

	// 优化点：直接准备扫描切片，避免循环分配
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return err
	}

	return scanRowToStruct(columns, values, v)
}

func StructScanAll(rows *sql.Rows, dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("dest must be a non-nil pointer to a slice")
	}

	sliceV := v.Elem()
	if sliceV.Kind() != reflect.Slice {
		return errors.New("dest must point to a slice")
	}

	elemType := sliceV.Type().Elem()
	if elemType.Kind() != reflect.Ptr || elemType.Elem().Kind() != reflect.Struct {
		return errors.New("slice elements must be pointers to structs")
	}

	structType := elemType.Elem()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// 优化点：复用扫描缓冲区
	colCount := len(columns)
	values := make([]interface{}, colCount)
	valuePtrs := make([]interface{}, colCount)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	results := reflect.MakeSlice(sliceV.Type(), 0, 16)

	for rows.Next() {
		// 每行需要新的 values 缓冲区，避免数据被覆盖
		values := make([]interface{}, colCount)
		valuePtrs := make([]interface{}, colCount)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		structV := reflect.New(structType).Elem()
		if err := scanRowToStruct(columns, values, structV); err != nil {
			return err
		}

		results = reflect.Append(results, structV.Addr())
	}

	sliceV.Set(results)
	return rows.Err()
}

func scanRowToStruct(columns []string, values []interface{}, structV reflect.Value) error {
	// 优化点：从缓存读取字段映射，不再实时解析 Tag 和 SnakeCase
	meta := getStructMeta(structV.Type())

	for i, col := range columns {
		fieldIdx, exists := meta.fieldMap[col]
		if !exists {
			continue
		}

		field := structV.Field(fieldIdx)
		if !field.CanSet() {
			continue
		}

		// 优化点：直接传入 values[i]，避免 reflect.ValueOf(*(values[i].(*interface{}))) 的多重解包开销
		if err := setFieldValue(field, values[i]); err != nil {
			return fmt.Errorf("cannot set field %s: %w", structV.Type().Field(fieldIdx).Name, err)
		}
	}
	return nil
}

func setFieldValue(field reflect.Value, val interface{}) error {
	// 处理数据库 NULL 值
	if val == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}

	fieldType := field.Type()

	// 快速路径：如果类型直接匹配（例如都是 string 或都是 int64）
	valV := reflect.ValueOf(val)
	if valV.Type().AssignableTo(fieldType) {
		field.Set(valV)
		return nil
	}

	// 如果 val 是指针（database/sql 扫描出来的有时会带指针），先取其值
	if valV.Kind() == reflect.Ptr {
		if valV.IsNil() {
			field.Set(reflect.Zero(fieldType))
			return nil
		}
		valV = valV.Elem()
		val = valV.Interface()
	}

	// 针对不同目标字段类型的优化转换逻辑
	switch field.Kind() {
	case reflect.String:
		switch v := val.(type) {
		case []byte:
			field.SetString(string(v))
		case string:
			field.SetString(v)
		default:
			// 只有兜底才使用 fmt.Sprintf
			field.SetString(fmt.Sprintf("%v", v))
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := val.(type) {
		case int64:
			field.SetInt(v)
		case []byte:
			if iv, err := strconv.ParseInt(string(v), 10, 64); err == nil {
				field.SetInt(iv)
			} else {
				return fmt.Errorf("cannot parse int from []byte: %w", err)
			}
		case string:
			if iv, err := strconv.ParseInt(v, 10, 64); err == nil {
				field.SetInt(iv)
			} else {
				return fmt.Errorf("cannot parse int from string: %w", err)
			}
		case float64:
			field.SetInt(int64(v))
		default:
			return fmt.Errorf("cannot convert %T to int", val)
		}

	case reflect.Bool:
		switch v := val.(type) {
		case bool:
			field.SetBool(v)
		case int64:
			field.SetBool(v != 0)
		case []byte:
			b, _ := strconv.ParseBool(string(v))
			field.SetBool(b)
		}

	case reflect.Float32, reflect.Float64:
		switch v := val.(type) {
		case float64:
			field.SetFloat(v)
		case []byte:
			if fv, err := strconv.ParseFloat(string(v), 64); err == nil {
				field.SetFloat(fv)
			} else {
				return fmt.Errorf("cannot parse float from []byte: %w", err)
			}
		default:
			return fmt.Errorf("cannot convert %T to float", val)
		}
	// 可以根据需要继续添加其他常用类型
	default:
		// 最后的兜底尝试
		if valV.Type().ConvertibleTo(fieldType) {
			field.Set(valV.Convert(fieldType))
		} else {
			return fmt.Errorf("unsupported conversion from %T to %v", val, fieldType)
		}
	}

	return nil
}
