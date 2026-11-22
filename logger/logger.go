package logger

import (
	"fmt"
	"os"
	"time"
)

func Printf(format string, args ...any) {
	fmt.Printf("[%s] %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	Printf(format, args...)
	os.Exit(1)
}
