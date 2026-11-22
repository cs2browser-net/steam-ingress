package steam_test

import (
	"math"
	"testing"

	"github.com/cs2browser-net/steam-ingress/steam"
)

func TestJSStringFloat(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{12345, "12345"},
		{12.5, "12.5"},
		{-5, "-5"},
		{1e6, "1000000"},
		{1234567890, "1234567890"},
		{1e20, "100000000000000000000"},
		{1e21, "1e+21"},
		{1.5e21, "1.5e+21"},
		{0.00001, "0.00001"},
		{1e-6, "0.000001"},
		{1e-7, "1e-7"},
		{1.23e-18, "1.23e-18"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	} {
		if got := steam.JSString(c.in); got != c.want {
			t.Errorf("JSString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
