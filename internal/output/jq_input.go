package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// jqInput converts typed command metadata to jq maps and arrays without copying
// response strings through a second JSON encoding and decoding cycle.
func jqInput(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case json.Number:
		return v, nil
	case json.RawMessage:
		var result any
		dec := json.NewDecoder(bytes.NewReader(v))
		dec.UseNumber()
		err := dec.Decode(&result)
		return result, err
	}
	if marshaler, ok := value.(json.Marshaler); ok {
		raw, err := marshaler.MarshalJSON()
		if err != nil {
			return nil, err
		}
		return jqInput(json.RawMessage(raw))
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil, nil
		}
		return jqInput(v.Elem().Interface())
	case reflect.Struct:
		result := map[string]any{}
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}
			tag := strings.Split(field.Tag.Get("json"), ",")
			if tag[0] == "-" {
				continue
			}
			fv := v.Field(i)
			omit := false
			for _, option := range tag[1:] {
				if option == "omitempty" && emptyJSONValue(fv) {
					omit = true
				}
			}
			if omit {
				continue
			}
			item, err := jqInput(fv.Interface())
			if err != nil {
				return nil, err
			}
			if field.Anonymous && tag[0] == "" {
				if fields, ok := item.(map[string]any); ok {
					for k, v := range fields {
						result[k] = v
					}
					continue
				}
			}
			name := tag[0]
			if name == "" {
				name = field.Name
			}
			result[name] = item
		}
		return result, nil
	case reflect.Map:
		if v.IsNil() {
			return nil, nil
		}
		result := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			item, err := jqInput(iter.Value().Interface())
			if err != nil {
				return nil, err
			}
			result[fmt.Sprint(iter.Key().Interface())] = item
		}
		return result, nil
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return nil, nil
		}
		result := make([]any, v.Len())
		for i := range result {
			item, err := jqInput(v.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			result[i] = item
		}
		return result, nil
	default:
		return value, nil
	}
}
func emptyJSONValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}
