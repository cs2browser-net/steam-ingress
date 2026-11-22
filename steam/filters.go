package steam

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cs2browser-net/steam-ingress/config"
)

func countInstructions(filters config.Ordered) int {
	count := 0
	for _, kv := range filters {
		var arr []json.RawMessage
		if json.Unmarshal(kv.Value, &arr) == nil {
			count += len(arr)
		} else {
			count++
		}
	}
	return count
}

func BuildFilters(filters config.Ordered) (string, error) {
	parts := make([]string, 0, len(filters))

	for _, kv := range filters {
		if kv.Key == "nand" || kv.Key == "nor" {
			var nested config.Ordered
			if err := json.Unmarshal(kv.Value, &nested); err != nil {
				return "", fmt.Errorf("filter %q: %w", kv.Key, err)
			}
			inner, err := BuildFilters(nested)
			if err != nil {
				return "", err
			}
			parts = append(parts, kv.Key+`\`+strconv.Itoa(countInstructions(nested))+inner)
			continue
		}

		var value any
		if err := json.Unmarshal(kv.Value, &value); err != nil {
			return "", fmt.Errorf("filter %q: %w", kv.Key, err)
		}
		if arr, ok := value.([]any); ok {
			for _, el := range arr {
				parts = append(parts, kv.Key+`\`+JSString(el))
			}
		} else {
			parts = append(parts, kv.Key+`\`+JSString(value))
		}
	}

	return `\` + strings.Join(parts, `\`), nil
}

func JSString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return jsNumberString(x)
	case int:
		return strconv.Itoa(x)
	case nil:
		return "null"
	default:
		return fmt.Sprint(x)
	}
}

func jsNumberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}

	mant, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(exp)
	k, n := len(digits), e+1

	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}

	s := digits[:1]
	if k > 1 {
		s += "." + digits[1:]
	}
	if e >= 0 {
		return sign + s + "e+" + strconv.Itoa(e)
	}
	return sign + s + "e-" + strconv.Itoa(-e)
}
