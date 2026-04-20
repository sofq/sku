package estimate

import (
	"fmt"
	"strconv"
	"strings"
)

// parseQuantity accepts plain numbers plus K/M/G/T/P suffixes (decimal, 1K=1000).
// Used by per-kind estimators for params like hours=730 or requests=1M.
func parseQuantity(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty quantity")
	}
	last := s[len(s)-1]
	mult := 1.0
	switch last {
	case 'k', 'K':
		mult = 1e3
	case 'm', 'M':
		mult = 1e6
	case 'g', 'G':
		mult = 1e9
	case 't', 'T':
		mult = 1e12
	case 'p', 'P':
		mult = 1e15
	}
	body := s
	if mult != 1 {
		body = strings.TrimSpace(s[:len(s)-1])
	}
	n, err := strconv.ParseFloat(body, 64)
	if err != nil {
		return 0, err
	}
	return n * mult, nil
}
