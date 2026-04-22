package caddydecl

import (
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
			`gate {}`,
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
			"arg + block",
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
	}
	for _, c := range cases {
		var g Gate
		ensure.Nil(t, Unmarshal(&g, caddyfile.NewTestDispenser(c.input)))
		ensure.DeepEqual(t, g, c.expected)
	}
}
