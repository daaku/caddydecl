// Package caddydecl provides reflect based Unmarshaling of Caddyfile config
// into a struct.
package caddydecl

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

// fieldInfo holds metadata about a struct field (possibly nested via embedding).
type fieldInfo struct {
	path          []int
	isSlice       bool
	isStruct      bool
	isUnmarshaler bool
}

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

	argFields, blockFields := collectFields(elem.Type())

	if !d.Next() {
		return nil
	}

	// Parse positional args or inline key-value pairs
	args := d.RemainingArgs()
	if len(argFields) > 0 {
		for idx, argVal := range args {
			fi, ok := argFields[idx]
			if !ok {
				continue
			}
			field := fieldByPath(elem, fi.path)
			if fi.isUnmarshaler {
				return d.Err(fmt.Sprintf("field with arg=%d implements Unmarshaler and cannot be used as positional argument", idx))
			}
			if err := setValue(field, argVal, false); err != nil {
				return d.WrapErr(fmt.Errorf("arg %d: %w", idx, err))
			}
		}
	} else {
		// Parse inline key-value pairs
		for i := 0; i < len(args); i++ {
			key := args[i]
			fi, ok := blockFields[key]
			if !ok {
				return d.Err(fmt.Sprintf("unrecognized key: %s", key))
			}
			field := fieldByPath(elem, fi.path)
			if fi.isStruct {
				return d.Err(fmt.Sprintf("struct field %s cannot be used inline", key))
			}
			if fi.isUnmarshaler {
				return d.Err(fmt.Sprintf("field %s implements Unmarshaler and cannot be used inline", key))
			}
			if i+1 >= len(args) {
				return d.Err(fmt.Sprintf("key %s has no value", key))
			}
			if fi.isSlice {
				for _, val := range args[i+1:] {
					if err := setValue(field, val, true); err != nil {
						return d.WrapErr(fmt.Errorf("%s: %w", key, err))
					}
				}
				break
			}
			if err := setValue(field, args[i+1], false); err != nil {
				return d.WrapErr(fmt.Errorf("%s: %w", key, err))
			}
			i++
		}
	}

	// Parse block properties
	for d.NextBlock(0) {
		key := d.Val()
		fi, ok := blockFields[key]
		if !ok {
			return d.Err(fmt.Sprintf("unrecognized key: %s", key))
		}

		field := fieldByPath(elem, fi.path)

		if fi.isUnmarshaler {
			file, line := d.File(), d.Line()
			unmarshaler := field.Addr().Interface().(caddyfile.Unmarshaler)
			if err := unmarshaler.UnmarshalCaddyfile(d); err != nil {
				return fmt.Errorf("%s (at %s:%d): %w", key, file, line, err)
			}
			continue
		}

		if fi.isStruct {
			file, line := d.File(), d.Line()
			seg := d.NewFromNextSegment()
			if err := Unmarshal(field.Addr().Interface(), seg); err != nil {
				return fmt.Errorf("%s (at %s:%d): %w", key, file, line, err)
			}
			continue
		}

		values := d.RemainingArgs()
		if fi.isSlice {
			for _, val := range values {
				if err := setValue(field, val, true); err != nil {
					return d.WrapErr(fmt.Errorf("%s: %w", key, err))
				}
			}
		} else {
			if len(values) > 0 {
				if err := setValue(field, values[0], false); err != nil {
					return d.WrapErr(fmt.Errorf("%s: %w", key, err))
				}
			}
		}
	}

	return nil
}

// collectFields recursively scans a struct type and returns mappings for
// positional arguments and block keys. Embedded (anonymous) struct fields are
// flattened into the parent so their fields are addressable directly.
func collectFields(t reflect.Type) (argFields map[int]*fieldInfo, blockFields map[string]*fieldInfo) {
	argFields = make(map[int]*fieldInfo)
	blockFields = make(map[string]*fieldInfo)
	unmarshalerType := reflect.TypeFor[caddyfile.Unmarshaler]()

	var walk func(rt reflect.Type, prefix []int)
	walk = func(rt reflect.Type, prefix []int) {
		// First pass: register all non-embedded fields at this level.
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if !field.IsExported() {
				continue
			}
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				// Skip embedded structs in the first pass; they are handled
				// recursively in the second pass.
				if !reflect.PointerTo(field.Type).Implements(unmarshalerType) {
					continue
				}
			}

			path := append([]int(nil), prefix...)
			path = append(path, i)

			fi := &fieldInfo{path: path}
			ft := field.Type
			if reflect.PointerTo(ft).Implements(unmarshalerType) {
				fi.isUnmarshaler = true
			} else if ft.Kind() == reflect.Slice {
				if ft.Elem().Kind() != reflect.Uint8 {
					fi.isSlice = true
				}
			} else if ft.Kind() == reflect.Struct {
				fi.isStruct = true
			}

			key := toSnakeCase(field.Name)
			tag := field.Tag.Get("caddydecl")
			if tag != "" {
				for part := range strings.SplitSeq(tag, ",") {
					part = strings.TrimSpace(part)
					kv := strings.SplitN(part, "=", 2)
					if len(kv) == 2 && kv[0] == "arg" {
						if idx, err := strconv.Atoi(kv[1]); err == nil {
							if _, exists := argFields[idx]; !exists {
								argFields[idx] = fi
							}
						}
					} else {
						key = part
					}
				}
			}
			if _, exists := blockFields[key]; !exists {
				blockFields[key] = fi
			}
		}

		// Second pass: recurse into embedded structs.
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if !field.IsExported() {
				continue
			}
			if !(field.Anonymous && field.Type.Kind() == reflect.Struct) {
				continue
			}
			// If the embedded struct implements Unmarshaler it was already
			// registered as a regular field in the first pass.
			if reflect.PointerTo(field.Type).Implements(unmarshalerType) {
				continue
			}
			path := append([]int(nil), prefix...)
			path = append(path, i)
			walk(field.Type, path)
		}
	}

	walk(t, nil)
	return argFields, blockFields
}

func fieldByPath(v reflect.Value, path []int) reflect.Value {
	for _, idx := range path {
		v = v.Field(idx)
	}
	return v
}

func setValue(field reflect.Value, val string, appendSlice bool) error {
	unmarshalerType := reflect.TypeFor[caddyfile.Unmarshaler]()

	// Special case: []byte as base64 URL encoded without padding
	if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Uint8 {
		decoded, err := base64.RawURLEncoding.DecodeString(val)
		if err != nil {
			return err
		}
		field.SetBytes(decoded)
		return nil
	}

	if field.Kind() == reflect.Slice {
		if !appendSlice {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		}
		elemType := field.Type().Elem()
		elem := reflect.New(elemType).Elem()
		if reflect.PointerTo(elemType).Implements(unmarshalerType) {
			unmarshaler := elem.Addr().Interface().(caddyfile.Unmarshaler)
			if err := unmarshaler.UnmarshalCaddyfile(caddyfile.NewTestDispenser(val)); err != nil {
				return err
			}
		} else if err := setScalar(elem, val); err != nil {
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
