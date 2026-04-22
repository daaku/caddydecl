# caddydecl

Reflection-based unmarshaling of Caddyfile directives into structs.

## Usage

```go
package main

import (
    "fmt"
    "time"

    "github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
    "github.com/daaku/caddydecl"
)

type Gate struct {
    Name    string        `caddydecl:"name,arg=0"`
    UserID  string        `caddydecl:"user"`
    Tags    []string
    Expires time.Duration
    Age     int
    Admin   bool
    Factor  float32
}

func main() {
    input := `gate example.com {
        tags admin staff
        user naitik
        expires 30d
        age 42
        admin true
        factor 1.5
    }`

    var g Gate
    if err := caddydecl.Unmarshal(&g, caddyfile.NewTestDispenser(input)); err != nil {
        panic(err)
    }

    fmt.Printf("%+v\n", g)
}
```

## Supported Types

| Type                                          | Notes                                             |
| --------------------------------------------- | ------------------------------------------------- |
| `string`                                      |                                                   |
| `int`, `int8`, `int16`, `int32`, `int64`      | Base 10 parsing                                   |
| `uint`, `uint8`, `uint16`, `uint32`, `uint64` | Base 10 parsing                                   |
| `float32`, `float64`                          |                                                   |
| `bool`                                        | `true`, `false`, `yes`, `no` (case-insensitive)   |
| `time.Duration`                               | Supports Caddy's `d` (day) suffix                 |
| Slices                                        | Any token after the key is appended as an element |
| Structs                                       | Recursively unmarshaled from nested blocks        |

### Slices

When a field is a slice, every remaining token on the same line is parsed and
appended:

```caddyfile
tags admin staff
```

```go
Tags []string // []string{"admin", "staff"}
```

### Nested Structs

A struct field is populated from a nested block with the same key:

```go
type Child struct {
    Age int
}
type Parent struct {
    Name  string
    Child Child
}
```

```caddyfile
parent {
    name foo
    child {
        age 10
    }
}
```

## Struct Tags

The `caddydecl` tag controls field mapping:

| Tag                     | Meaning                                                            |
| ----------------------- | ------------------------------------------------------------------ |
| (none)                  | Key defaults to snake_case of the field name                       |
| `caddydecl:"foo"`       | Use `foo` as the block key                                         |
| `caddydecl:"arg=0"`     | Map the 0th positional argument to this field                      |
| `caddydecl:"foo,arg=0"` | Use `foo` as the block key **and** map the 0th positional argument |

## Positional Arguments

Tokens that appear on the same line as the directive (before the opening `{`)
are treated as positional arguments. Use `arg=N` to bind argument `N` to a
field:

```go
type Gate struct {
    Name string `caddydecl:"arg=0"`
}
```

```caddyfile
gate example.com
```

```go
Name == "example.com"
```

Positional arguments can be combined with block properties:

```caddyfile
gate example.com {
    user naitik
}
```
