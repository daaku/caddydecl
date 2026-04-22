// Package caddydecl provides reflect based Unmarshaling of Caddyfile config
// into a struct.
package caddydecl

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

// Unmarshal directives from caddyfiles into structs in a predictable fashion.
func Unmarshal(v any, d *caddyfile.Dispenser) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("v must be a non-nil pointer")
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("v must point to a struct")
	}

	rt := elem.Type()

	argFields := make(map[int]int)      // arg index -> field index
	blockFields := make(map[string]int) // key -> field index
	sliceFields := make(map[int]bool)   // field index -> is slice

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		if field.Type.Kind() == reflect.Slice {
			sliceFields[i] = true
		}

		// All exported fields are potential block properties
		key := toSnakeCase(field.Name)

		// Check for caddydecl tag (custom name and/or positional arg)
		tag := field.Tag.Get("caddydecl")
		if tag != "" {
			for _, part := range strings.Split(tag, ",") {
				part = strings.TrimSpace(part)
				kv := strings.SplitN(part, "=", 2)
				if len(kv) == 2 && kv[0] == "arg" {
					if idx, err := strconv.Atoi(kv[1]); err == nil {
						argFields[idx] = i
					}
				} else {
					key = part
				}
			}
		}
		blockFields[key] = i
	}

	if !d.Next() {
		return nil
	}

	// Parse positional args
	args := d.RemainingArgs()
	for idx, argVal := range args {
		if fieldIdx, ok := argFields[idx]; ok {
			field := elem.Field(fieldIdx)
			if err := setValue(field, argVal, false); err != nil {
				return fmt.Errorf("arg %d: %w", idx, err)
			}
		}
	}

	// Parse block properties
	for d.NextBlock(0) {
		key := d.Val()
		fieldIdx, ok := blockFields[key]
		if !ok {
			continue
		}

		values := d.RemainingArgs()
		field := elem.Field(fieldIdx)
		isSlice := sliceFields[fieldIdx]

		if isSlice {
			for _, val := range values {
				if err := setValue(field, val, true); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
			}
		} else {
			if len(values) > 0 {
				if err := setValue(field, values[0], false); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
			}
		}
	}

	return nil
}

func setValue(field reflect.Value, val string, appendSlice bool) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field")
	}

	if field.Kind() == reflect.Slice {
		if !appendSlice {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		}
		elem := reflect.New(field.Type().Elem()).Elem()
		if err := setScalar(elem, val); err != nil {
			return err
		}
		field.Set(reflect.Append(field, elem))
		return nil
	}

	return setScalar(field, val)
}

func setScalar(field reflect.Value, val string) error {
	// time.Duration is an int64, so check the type before the kind
	if field.Type() == reflect.TypeFor[time.Duration]() {
		d, err := caddy.ParseDuration(val)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(d))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(i)
	case reflect.Float32, reflect.Float64:
		bits := 64
		if field.Kind() == reflect.Float32 {
			bits = 32
		}
		f, err := strconv.ParseFloat(val, bits)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Bool:
		switch strings.ToLower(val) {
		case "true", "yes":
			field.SetBool(true)
		case "false", "no":
			field.SetBool(false)
		default:
			return fmt.Errorf("invalid bool value: %s", val)
		}
	default:
		return fmt.Errorf("unsupported type: %v", field.Type())
	}
	return nil
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 && s[i-1] >= 'a' && s[i-1] <= 'z' {
				result.WriteByte('_')
			}
			result.WriteByte(c + ('a' - 'A'))
		} else {
			result.WriteByte(c)
		}
	}
	return result.String()
}
