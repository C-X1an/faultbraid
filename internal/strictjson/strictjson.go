// Package strictjson rejects ambiguous JSON before typed decoding.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 1 << 20
const MaxDepth = 8

// ReadValue returns one bounded, syntactically valid JSON value. It rejects
// invalid UTF-8, duplicate keys, excessive nesting and trailing input. Callers
// must still apply a typed schema before using the value as executable input.
func ReadValue(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, errors.New("cannot read input")
	}
	if len(b) > MaxBytes {
		return nil, errors.New("input exceeds byte limit")
	}
	if !utf8.Valid(b) {
		return nil, errors.New("input is not valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := walk(d, 0); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON or invalid suffix")
	}
	return b, nil
}

// Decode accepts one bounded JSON value, with no duplicate or unknown keys.
// The destination must be a non-nil pointer suitable for encoding/json.
func Decode(r io.Reader, dst any) error {
	b, err := ReadValue(r)
	if err != nil {
		return err
	}
	shapeDecoder := json.NewDecoder(bytes.NewReader(b))
	shapeDecoder.UseNumber()
	var value any
	if err := shapeDecoder.Decode(&value); err != nil {
		return errors.New("invalid JSON")
	}
	typ := reflect.TypeOf(dst)
	if typ == nil || typ.Kind() != reflect.Pointer || reflect.ValueOf(dst).IsNil() {
		return errors.New("invalid decoder destination")
	}
	if err := shape(value, typ.Elem()); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return errors.New("input does not match schema")
	}
	return nil
}

func walk(d *json.Decoder, depth int) error {
	if depth > MaxDepth {
		return errors.New("JSON nesting limit exceeded")
	}
	t, err := d.Token()
	if err != nil {
		return errors.New("invalid JSON")
	}
	switch delim := t.(type) {
	case json.Delim:
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for d.More() {
				t, err := d.Token()
				if err != nil {
					return errors.New("invalid object")
				}
				key, ok := t.(string)
				if !ok {
					return errors.New("invalid key")
				}
				if _, ok := seen[key]; ok {
					return errors.New("duplicate JSON key")
				}
				seen[key] = struct{}{}
				if err := walk(d, depth+1); err != nil {
					return err
				}
			}
			t, err := d.Token()
			if err != nil || t != json.Delim('}') {
				return errors.New("invalid object end")
			}
		case '[':
			for d.More() {
				if err := walk(d, depth+1); err != nil {
					return err
				}
			}
			t, err := d.Token()
			if err != nil || t != json.Delim(']') {
				return errors.New("invalid array end")
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	return nil
}

// shape closes encoding/json's case-insensitive field and null-to-zero coercions.
func shape(value any, typ reflect.Type) error {
	if typ.Kind() == reflect.Pointer {
		if value == nil {
			return nil
		}
		return shape(value, typ.Elem())
	}
	if value == nil {
		return errors.New("null is not allowed for this field")
	}
	switch typ.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return errors.New("expected JSON object")
		}
		fields := make(map[string]reflect.StructField)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				name = f.Name
			}
			fields[name] = f
		}
		for name, v := range object {
			f, ok := fields[name]
			if !ok {
				return errors.New("unknown JSON field")
			}
			if err := shape(v, f.Type); err != nil {
				return err
			}
		}
		for name, f := range fields {
			_, present := object[name]
			if !present && f.Type.Kind() != reflect.Pointer && !strings.Contains(f.Tag.Get("json"), ",omitempty") {
				return errors.New("required JSON field missing")
			}
		}
	case reflect.Slice, reflect.Array:
		arr, ok := value.([]any)
		if !ok {
			return errors.New("expected JSON array")
		}
		for _, v := range arr {
			if err := shape(v, typ.Elem()); err != nil {
				return err
			}
		}
	case reflect.String:
		if _, ok := value.(string); !ok {
			return errors.New("expected JSON string")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := value.(json.Number)
		if !ok || strings.ContainsAny(string(n), ".eE") {
			return errors.New("expected JSON integer")
		}
	case reflect.Bool:
		if _, ok := value.(bool); !ok {
			return errors.New("expected JSON boolean")
		}
	default:
		return errors.New("unsupported decoder schema")
	}
	return nil
}
