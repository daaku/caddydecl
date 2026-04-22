package caddydecl

import (
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/daaku/ensure"
)

type Gate struct {
	Name    string `caddydecl:"name,arg=0"`
	UserID  string `caddydecl:"user"`
	Tags    []string
	Expires time.Duration
	Age     int
	Admin   bool
	Factor  float32
}

func TestSuccess(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected Gate
	}{
		{
			"empty dispenser",
			`gate`,
			Gate{},
		},
		{
			"empty block",
			`gate {
			}`,
			Gate{},
		},
		{
			"arg name",
			`gate example.com`,
			Gate{
				Name: "example.com",
			},
		},
		{
			"block full",
			`gate {
				name example.com
				tags admin staff
				user naitik
				expires 1d
			}`,
			Gate{
				Name:    "example.com",
				UserID:  "naitik",
				Tags:    []string{"admin", "staff"},
				Expires: time.Hour * 24,
			},
		},
		{
			"arg and block",
			`gate example.com {
				tags admin staff
				user naitik
				expires 30d
				age 42
				admin true
				factor 1.5
			}`,
			Gate{
				Name:    "example.com",
				UserID:  "naitik",
				Tags:    []string{"admin", "staff"},
				Expires: 30 * 24 * time.Hour,
				Age:     42,
				Admin:   true,
				Factor:  1.5,
			},
		},
		{
			"repeat slice appends",
			`gate {
				user naitik
				tags admin
				tags staff
			}`,
			Gate{
				UserID: "naitik",
				Tags:   []string{"admin", "staff"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var g Gate
			ensure.Nil(t, Unmarshal(&g, caddyfile.NewTestDispenser(c.input)))
			ensure.DeepEqual(t, g, c.expected)
		})
	}
}

func TestNested(t *testing.T) {
	type Child struct {
		Age int
	}
	type Nested struct {
		Name  string
		Child Child
	}

	var n Nested
	d := caddyfile.NewTestDispenser(
		`nested {
				name foo
				child {
					age 10
				}
			}`)
	ensure.Nil(t, Unmarshal(&n, d))
	ensure.DeepEqual(t, n,
		Nested{
			Name: "foo",
			Child: Child{
				Age: 10,
			},
		})
}

func TestValidation(t *testing.T) {
	if err := Unmarshal(nil, caddyfile.NewTestDispenser(``)); err == nil {
		t.Fatal("expected error for nil pointer")
	}
	var s string
	if err := Unmarshal(&s, caddyfile.NewTestDispenser(``)); err == nil {
		t.Fatal("expected error for non-struct pointer")
	}
}

func TestNoTokens(t *testing.T) {
	type Empty struct{}
	var e Empty
	ensure.Nil(t, Unmarshal(&e, caddyfile.NewTestDispenser(``)))
}

func TestUnknownKey(t *testing.T) {
	type Simple struct {
		Name string
	}
	var s Simple
	err := Unmarshal(&s, caddyfile.NewTestDispenser(`simple {
		name foo
		unknown bar
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unrecognized key: unknown") {
		t.Fatalf("expected unrecognized key error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestUnexportedField(t *testing.T) {
	type Mixed struct {
		Name    string
		secret  string
		Visible int
	}
	var m Mixed
	ensure.Nil(t, Unmarshal(&m, caddyfile.NewTestDispenser(`mixed {
		name foo
		visible 42
	}`)))
	ensure.DeepEqual(t, m, Mixed{Name: "foo", Visible: 42})
}

func TestUint(t *testing.T) {
	type Unsigned struct {
		Count uint
	}
	var u Unsigned
	ensure.Nil(t, Unmarshal(&u, caddyfile.NewTestDispenser(`unsigned {
		count 99
	}`)))
	ensure.DeepEqual(t, u, Unsigned{Count: 99})
}

func TestBoolVariations(t *testing.T) {
	type Flags struct {
		A bool
		B bool
		C bool
		D bool
	}
	var f Flags
	ensure.Nil(t, Unmarshal(&f, caddyfile.NewTestDispenser(`flags {
		a true
		b yes
		c false
		d no
	}`)))
	ensure.DeepEqual(t, f, Flags{A: true, B: true, C: false, D: false})
}

func TestSlicePositionalArg(t *testing.T) {
	type Sliced struct {
		Tags []string `caddydecl:"arg=0"`
	}
	var s Sliced
	ensure.Nil(t, Unmarshal(&s, caddyfile.NewTestDispenser(`sliced foo bar`)))
	ensure.DeepEqual(t, s, Sliced{Tags: []string{"foo"}})
}

func TestInvalidPositionalArg(t *testing.T) {
	type Bad struct {
		Age int `caddydecl:"arg=0"`
	}
	var b Bad
	err := Unmarshal(&b, caddyfile.NewTestDispenser(`bad notanumber`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestInvalidBlockValues(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"bad int", `v { age notanumber }`},
		{"bad uint", `v { count notanumber }`},
		{"bad float", `v { factor notanumber }`},
		{"bad bool", `v { admin maybe }`},
		{"bad duration", `v { expires bad }`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			type All struct {
				Age     int
				Count   uint
				Factor  float32
				Admin   bool
				Expires time.Duration
			}
			var a All
			err := Unmarshal(&a, caddyfile.NewTestDispenser(c.input))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "Testfile") {
				t.Fatalf("expected file info in error, got: %v", err)
			}
		})
	}
}

func TestInvalidSliceElement(t *testing.T) {
	type BadSlice struct {
		Ages []int
	}
	var b BadSlice
	err := Unmarshal(&b, caddyfile.NewTestDispenser(`bad {
		ages 1 two 3
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestUnsupportedType(t *testing.T) {
	type Bad struct {
		Ch chan int
	}
	var b Bad
	err := Unmarshal(&b, caddyfile.NewTestDispenser(`bad {
		ch foo
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestNestedError(t *testing.T) {
	type Child struct {
		Age int
	}
	type Parent struct {
		Child Child
	}
	var p Parent
	err := Unmarshal(&p, caddyfile.NewTestDispenser(`parent {
		child {
			age notanumber
		}
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestInvalidTagArg(t *testing.T) {
	// Invalid arg index should be silently ignored, field still usable as block key
	type Weird struct {
		Name string `caddydecl:"name,arg=xyz"`
	}
	var w Weird
	ensure.Nil(t, Unmarshal(&w, caddyfile.NewTestDispenser(`weird {
		name foo
	}`)))
	ensure.DeepEqual(t, w, Weird{Name: "foo"})
}

func TestErrorsWrapped(t *testing.T) {
	type Bad struct {
		Age int `caddydecl:"arg=0"`
	}
	var b Bad
	err := Unmarshal(&b, caddyfile.NewTestDispenser(`bad notanumber`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "arg 0:") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestByteSlice(t *testing.T) {
	type Bytes struct {
		Data []byte
	}
	var b Bytes
	// "Zm9v" is base64url for "foo"
	ensure.Nil(t, Unmarshal(&b, caddyfile.NewTestDispenser(`bytes {
		data Zm9v
	}`)))
	ensure.DeepEqual(t, b, Bytes{Data: []byte("foo")})
}

func TestByteSlicePositionalArg(t *testing.T) {
	type Bytes struct {
		Data []byte `caddydecl:"arg=0"`
	}
	var b Bytes
	// "YmFy" is base64url for "bar"
	ensure.Nil(t, Unmarshal(&b, caddyfile.NewTestDispenser(`bytes YmFy`)))
	ensure.DeepEqual(t, b, Bytes{Data: []byte("bar")})
}

func TestByteSliceMultipleValues(t *testing.T) {
	type Bytes struct {
		Data []byte
	}
	var b Bytes
	// "Zm9v" = "foo", "YmFy" = "bar"; last value overwrites
	ensure.Nil(t, Unmarshal(&b, caddyfile.NewTestDispenser(`bytes {
		data Zm9v
		data YmFy
	}`)))
	ensure.DeepEqual(t, b, Bytes{Data: []byte("bar")})
}

func TestByteSliceInvalidBase64(t *testing.T) {
	type Bytes struct {
		Data []byte
	}
	var b Bytes
	err := Unmarshal(&b, caddyfile.NewTestDispenser(`bytes {
		data not-valid!!!
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestByteSliceSingleValueOnly(t *testing.T) {
	type Bytes struct {
		Data []byte
	}
	var b Bytes
	// Only the first token is used; the rest are ignored
	ensure.Nil(t, Unmarshal(&b, caddyfile.NewTestDispenser(`bytes {
		data Zm9v YmFy
	}`)))
	ensure.DeepEqual(t, b, Bytes{Data: []byte("foo")})
}

func TestInlineKeyValues(t *testing.T) {
	type Inline struct {
		Name string
		Age  int
		Tags []string
	}

	cases := []struct {
		name     string
		input    string
		expected Inline
	}{
		{
			"multiple inline",
			`inline name foo age 42`,
			Inline{
				Name: "foo",
				Age:  42,
			},
		},
		{
			"some inline some block",
			`inline name foo {
				age 42
			}`,
			Inline{
				Name: "foo",
				Age:  42,
			},
		},
		{
			"inline with slice",
			`inline tags foo bar {
				age 42
			}`,
			Inline{
				Age:  42,
				Tags: []string{"foo", "bar"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var i Inline
			ensure.Nil(t, Unmarshal(&i, caddyfile.NewTestDispenser(c.input)))
			ensure.DeepEqual(t, i, c.expected)
		})
	}
}

func TestInlineUnknownKeyError(t *testing.T) {
	type Inline struct {
		Name string
	}
	var i Inline
	err := Unmarshal(&i, caddyfile.NewTestDispenser(`inline name foo unknown bar`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unrecognized key: unknown") {
		t.Fatalf("expected unrecognized key error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestInlineStructFieldError(t *testing.T) {
	type Child struct {
		Age int
	}
	type Inline struct {
		Name  string
		Child Child
	}
	var i Inline
	err := Unmarshal(&i, caddyfile.NewTestDispenser(`inline name foo child`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "struct field child cannot be used inline") {
		t.Fatalf("expected struct inline error, got: %v", err)
	}
}

func TestInlineSliceConsumesAll(t *testing.T) {
	type Inline struct {
		Tags []string
	}
	var i Inline
	// Slice consumes every remaining token to the end of the line
	ensure.Nil(t, Unmarshal(&i, caddyfile.NewTestDispenser(`inline tags foo bar age 42`)))
	ensure.DeepEqual(t, i, Inline{Tags: []string{"foo", "bar", "age", "42"}})
}

func TestInlineKeysBeforeSlice(t *testing.T) {
	type Inline struct {
		Name string
		Tags []string
	}
	var i Inline
	ensure.Nil(t, Unmarshal(&i, caddyfile.NewTestDispenser(`inline name foo tags bar baz`)))
	ensure.DeepEqual(t, i, Inline{Name: "foo", Tags: []string{"bar", "baz"}})
}

func TestInlineKeyWithoutValueError(t *testing.T) {
	type Inline struct {
		Name string
		Age  int
	}
	var i Inline
	err := Unmarshal(&i, caddyfile.NewTestDispenser(`inline name foo age`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "key age has no value") {
		t.Fatalf("expected no value error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestInlineSliceWithoutValueError(t *testing.T) {
	type Inline struct {
		Tags []string
	}
	var i Inline
	err := Unmarshal(&i, caddyfile.NewTestDispenser(`inline tags`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "key tags has no value") {
		t.Fatalf("expected no value error, got: %v", err)
	}
}

func TestInlineInvalidValue(t *testing.T) {
	type Inline struct {
		Age int
	}
	var i Inline
	err := Unmarshal(&i, caddyfile.NewTestDispenser(`inline age notanumber`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}

func TestInlineInvalidSliceValue(t *testing.T) {
	type Inline struct {
		Ages []int
	}
	var i Inline
	err := Unmarshal(&i, caddyfile.NewTestDispenser(`inline ages 1 two`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Testfile") {
		t.Fatalf("expected file info in error, got: %v", err)
	}
}
