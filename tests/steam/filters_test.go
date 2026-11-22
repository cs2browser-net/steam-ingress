package steam_test

import (
	"encoding/json"
	"testing"

	"github.com/cs2browser-net/steam-ingress/config"
	"github.com/cs2browser-net/steam-ingress/steam"
)

func TestBuildFiltersKeepsOrder(t *testing.T) {
	var f config.Ordered
	if err := json.Unmarshal([]byte(`{"appid":730,"gamedir":"csgo","nor":{"name_match":["Valve*","Other*"],"gametype":"empty"}}`), &f); err != nil {
		t.Fatal(err)
	}
	got, err := steam.BuildFilters(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `\appid\730\gamedir\csgo\nor\3\name_match\Valve*\name_match\Other*\gametype\empty`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
