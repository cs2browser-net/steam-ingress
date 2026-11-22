package rules_test

import (
	"encoding/json"
	"testing"

	"github.com/cs2browser-net/steam-ingress/config"
	"github.com/cs2browser-net/steam-ingress/rules"
	"github.com/cs2browser-net/steam-ingress/steam"
)

func compile(t *testing.T, src string) *rules.Set {
	t.Helper()
	var o config.Ordered
	if err := json.Unmarshal([]byte(src), &o); err != nil {
		t.Fatal(err)
	}
	set, err := rules.Compile(o)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestFirstMatchWins(t *testing.T) {
	set := compile(t, `{
		"name == \"Fake Server\"": 2,
		"startsWith(name, \"Fake\") and includes(tags, \"secure\")": 3,
		"number(\"10\") < players or max_players == 0": 4,
		"map == \"de_dust2\" and not secure": 5
	}`)

	cases := []struct {
		s    steam.ServerInfo
		want int
	}{
		{steam.ServerInfo{Name: "Fake Server", MaxPlayers: 10}, 2},
		{steam.ServerInfo{Name: "Fake Server 2", Tags: []string{"secure"}, MaxPlayers: 10}, 3},
		{steam.ServerInfo{Name: "Real", Players: 11, MaxPlayers: 20}, 4},
		{steam.ServerInfo{Name: "Real", Players: 5, MaxPlayers: 20, Map: "de_dust2"}, 5},
		{steam.ServerInfo{Name: "Real", Players: 5, MaxPlayers: 20}, 0},
	}
	for _, c := range cases {
		if got := set.Evaluate(&c.s); got != c.want {
			t.Errorf("%+v: got %d, want %d", c.s, got, c.want)
		}
	}
}

// startsWith is an expr-lang keyword; calls get renamed, but never inside string literals.
func TestStartsWithCallSyntax(t *testing.T) {
	set := compile(t, `{
		"startsWith(name, \"startsWith(\")": 6,
		"startsWith (map, 'x')": 7
	}`)

	cases := []struct {
		s    steam.ServerInfo
		want int
	}{
		{steam.ServerInfo{Name: "startsWith(x"}, 6},
		{steam.ServerInfo{Name: "other", Map: "xyz"}, 7},
		{steam.ServerInfo{Name: "other", Map: "abc"}, 0},
	}
	for _, c := range cases {
		if got := set.Evaluate(&c.s); got != c.want {
			t.Errorf("%+v: got %d, want %d", c.s, got, c.want)
		}
	}
}

func TestCompileRejectsBadConfig(t *testing.T) {
	for _, src := range []string{
		`{"players == \"5\"": 1}`, // type error: caught at startup, not on every poll
		`{"nope > 1": 1}`,         // unknown variable
		`{"players > 1": "x"}`,    // status must be an integer
	} {
		var o config.Ordered
		if err := json.Unmarshal([]byte(src), &o); err != nil {
			t.Fatal(err)
		}
		if _, err := rules.Compile(o); err == nil {
			t.Errorf("%s: expected an error", src)
		}
	}
}

// Expressions from the production config.
func TestProductionExpressions(t *testing.T) {
	set := compile(t, `{
		"max_players > 64": 2,
		"gamedir != \"csgo\" or product != \"cs2\"": 3,
		"string(number(name)) == name": 3,
		"not startsWith(version, \"1.4\")": 4,
		"secure == true and includes(tags, \"insecure\")": 8,
		"secure == false and includes(tags, \"secure\")": 8,
		"includes(tags, \"cola\") or includes(tags, \"colateam\")": 8
	}`)

	ok := steam.ServerInfo{Name: "My Server", GameDir: "csgo", Product: "cs2", Version: "1.40.9.1", MaxPlayers: 32, Secure: true, Tags: []string{"secure"}}
	cases := []struct {
		edit func(*steam.ServerInfo)
		want int
	}{
		{func(s *steam.ServerInfo) {}, 0},
		{func(s *steam.ServerInfo) { s.MaxPlayers = 65 }, 2},
		{func(s *steam.ServerInfo) { s.Product = "csgo" }, 3},
		{func(s *steam.ServerInfo) { s.Name = "12345" }, 3},
		{func(s *steam.ServerInfo) { s.Name = "12.5" }, 3},
		{func(s *steam.ServerInfo) { s.Name = "1e3" }, 0},  // JS: String(1000) != "1e3"
		{func(s *steam.ServerInfo) { s.Name = " 42 " }, 0}, // JS: String(42) != " 42 "
		{func(s *steam.ServerInfo) { s.Version = "1.39.1" }, 4},
		{func(s *steam.ServerInfo) { s.Tags = []string{"insecure"} }, 8},
		{func(s *steam.ServerInfo) { s.Secure = false }, 8},
		{func(s *steam.ServerInfo) { s.Tags = []string{"secure", "colateam"} }, 8},
	}
	for i, c := range cases {
		s := ok
		c.edit(&s)
		if got := set.Evaluate(&s); got != c.want {
			t.Errorf("case %d (%+v): got %d, want %d", i, s, got, c.want)
		}
	}
}
