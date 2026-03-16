package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)
// It uses reflection to map column names to struct fields.
// Struct tags are supported with "db" key for custom column names.
// Fields without db tag use field name converted to snake_case.
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

	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	if err := rows.Scan(values...); err != nil {
		return err
	}

	return scanRowToStruct(columns, values, v)
}

// StructScanAll scans all rows into a slice of structs.
func StructScanAll(rows *sql.Rows, dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("dest must be a non-nil pointer to a slice")
	}

	sliceV := v.Elem()
	if sliceV.Kind() != reflect.Slice {
		return errors.New("dest must point to a slice")
	}

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	elemType := sliceV.Type().Elem()
	if elemType.Kind() != reflect.Ptr || elemType.Elem().Kind() != reflect.Struct {
		return errors.New("slice elements must be pointers to structs")
	}

	structType := elemType.Elem()
	results := reflect.MakeSlice(reflect.SliceOf(elemType), 0, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		for i := range values {
			values[i] = new(interface{})
		}

		if err := rows.Scan(values...); err != nil {
			return err
		}

		structV := reflect.New(structType)
		if err := scanRowToStruct(columns, values, structV.Elem()); err != nil {
			return err
		}

		results = reflect.Append(results, structV)
	}

	sliceV.Set(results)
	return rows.Err()
}

// scanRowToStruct is a helper function that maps column values to struct fields.
func scanRowToStruct(columns []string, values []interface{}, structV reflect.Value) error {
	structType := structV.Type()

	// Build field map for efficient lookup
	fieldMap := make(map[string]int)
	for i := 0; i < structV.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		// Check for db tag
		tag := field.Tag.Get("db")
		if tag != "" {
			fieldMap[tag] = i
		} else {
			// Use snake_case field name
			fieldMap[ToSnakeCase(field.Name)] = i
		}
	}

	for i, col := range columns {
		fieldIdx, exists := fieldMap[col]
		if !exists {
			continue // Skip columns that don't have matching fields
		}

		field := structV.Field(fieldIdx)
		if !field.CanSet() {
			continue
		}

		val := reflect.ValueOf(*(values[i].(*interface{})))
		if !val.IsValid() {
			continue
		}

		// Handle nil values
		if val.Kind() == reflect.Ptr && val.IsNil() {
			continue
		}

		// Convert value to field type
		if err := setFieldValue(field, val); err != nil {
			return fmt.Errorf("cannot set field %s: %w", structType.Field(fieldIdx).Name, err)
		}
	}

	return nil
}

// setFieldValue sets a value to a struct field with type conversion.
func setFieldValue(field reflect.Value, val reflect.Value) error {
	fieldType := field.Type()

	// Handle nil values for nullable types
	if val.Kind() == reflect.Ptr && val.IsNil() {
		switch field.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
			field.Set(reflect.Zero(fieldType))
		case reflect.String:
			field.SetString("")
		case reflect.Bool:
			field.SetBool(false)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(0)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(0)
		case reflect.Float32, reflect.Float64:
			field.SetFloat(0)
		default:
			field.Set(reflect.Zero(fieldType))
		}
		return nil
	}

	// Unwrap pointer if needed
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Direct assignment if types match
	if val.Type().AssignableTo(fieldType) {
		field.Set(val)
		return nil
	}

	// Type conversion for basic types
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", val.Interface()))
	case reflect.Bool:
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetBool(val.Int() != 0)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetBool(val.Uint() != 0)
		default:
			return fmt.Errorf("cannot convert %v to bool", val.Type())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(val.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetInt(int64(val.Uint()))
		case reflect.Float32, reflect.Float64:
			field.SetInt(int64(val.Float()))
		case reflect.String:
			if intVal, err := strconv.ParseInt(val.String(), 10, 64); err == nil {
				field.SetInt(intVal)
			} else {
				return err
			}
		default:
			return fmt.Errorf("cannot convert %v to int", val.Type())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetUint(uint64(val.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(val.Uint())
		case reflect.Float32, reflect.Float64:
			field.SetUint(uint64(val.Float()))
		case reflect.String:
			if uintVal, err := strconv.ParseUint(val.String(), 10, 64); err == nil {
				field.SetUint(uintVal)
			} else {
				return err
			}
		default:
			return fmt.Errorf("cannot convert %v to uint", val.Type())
		}
	case reflect.Float32, reflect.Float64:
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetFloat(float64(val.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetFloat(float64(val.Uint()))
		case reflect.Float32, reflect.Float64:
			field.SetFloat(val.Float())
		case reflect.String:
			if floatVal, err := strconv.ParseFloat(val.String(), 64); err == nil {
				field.SetFloat(floatVal)
			} else {
				return err
			}
		default:
			return fmt.Errorf("cannot convert %v to float", val.Type())
		}
	case reflect.Slice:
		if fieldType.Elem().Kind() == reflect.Uint8 && val.Kind() == reflect.Slice && val.Type().Elem().Kind() == reflect.Uint8 {
			// []byte conversion
			field.Set(val)
		} else {
			return fmt.Errorf("cannot convert %v to slice", val.Type())
		}
	default:
		return fmt.Errorf("unsupported field type: %v", fieldType)
	}

	return nil
}
