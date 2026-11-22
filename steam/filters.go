package steam

import (
	"encoding/json"
	"fmt"
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
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case nil:
		return "null"
	default:
		return fmt.Sprint(x)
	}
}
