package model

import (
	"fmt"
	"strings"
)

func collectInt64s(ids map[int64]bool) []int64 {
	keys := make([]int64, 0, len(ids))
	for k := range ids {
		keys = append(keys, k)
	}
	return keys
}

func buildPlaceholders(n int) string {
	ps := make([]string, n)
	for i := range ps {
		ps[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(ps, ", ")
}

func int64sToArgs(keys []int64) []interface{} {
	args := make([]interface{}, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	return args
}
