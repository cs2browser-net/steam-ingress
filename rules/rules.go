package rules

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/cs2browser-net/steam-ingress/config"
	"github.com/cs2browser-net/steam-ingress/logger"
	"github.com/cs2browser-net/steam-ingress/steam"
)

type compiled struct {
	source  string
	status  int
	program *vm.Program
}

type Set struct{ exprs []compiled }

func jsNumber(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return math.NaN()
	default:
		return math.NaN()
	}
}

var (
	stringLiteral  = regexp.MustCompile(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`)
	startsWithCall = regexp.MustCompile(`\bstartsWith\s*\(`)
)

func renameStartsWithCalls(src string) string {
	var b strings.Builder
	last := 0
	for _, lit := range stringLiteral.FindAllStringIndex(src, -1) {
		b.WriteString(startsWithCall.ReplaceAllString(src[last:lit[0]], "_startsWith("))
		b.WriteString(src[lit[0]:lit[1]])
		last = lit[1]
	}
	b.WriteString(startsWithCall.ReplaceAllString(src[last:], "_startsWith("))
	return b.String()
}

var exprFunctions = []expr.Option{
	expr.Function("number", func(p ...any) (any, error) { return jsNumber(p[0]), nil },
		new(func(any) float64)),
	expr.DisableBuiltin("string"),
	expr.Function("string", func(p ...any) (any, error) { return steam.JSString(p[0]), nil },
		new(func(any) string)),
	expr.Function("_startsWith", func(p ...any) (any, error) {
		return strings.HasPrefix(steam.JSString(p[0]), steam.JSString(p[1])), nil
	}, new(func(any, any) bool)),
	expr.Function("includes", func(p ...any) (any, error) {
		arr, _ := p[0].([]string)
		return slices.Contains(arr, steam.JSString(p[1])), nil
	}, new(func(any, any) bool)),
}

func Compile(src config.Ordered) (*Set, error) {
	opts := append([]expr.Option{expr.Env(steam.ServerInfo{})}, exprFunctions...)
	out := make([]compiled, 0, len(src))

	for _, kv := range src {
		var status int
		if err := json.Unmarshal(kv.Value, &status); err != nil {
			return nil, fmt.Errorf("expression %q: status must be an integer", kv.Key)
		}
		program, err := expr.Compile(renameStartsWithCalls(kv.Key), opts...)
		if err != nil {
			return nil, fmt.Errorf("expression %q: %w", kv.Key, err)
		}
		out = append(out, compiled{source: kv.Key, status: status, program: program})
	}
	return &Set{exprs: out}, nil
}

func (s *Set) Evaluate(server *steam.ServerInfo) int {
	for _, e := range s.exprs {
		result, err := expr.Run(e.program, server)
		if err != nil {
			logger.Printf("Expression %q failed for %s: %v", e.source, server.Addr, err)
			continue
		}
		if truthy(result) {
			return e.status
		}
	}
	return 0
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int:
		return x != 0
	case float64:
		return x != 0 && !math.IsNaN(x)
	default:
		return true
	}
}
