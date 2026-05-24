package tasks

import (
	"os"
	"strings"
)

// mergeEnv merges the task's env overrides over the parent process's
// env, returning the merged slice in `KEY=VALUE` form. Later writes win
// on duplicate keys.
func mergeEnv(over map[string]string) []string {
	parent := os.Environ()
	merged := make([]string, 0, len(parent)+len(over))
	seen := make(map[string]int, len(parent))
	for _, kv := range parent {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			merged = append(merged, kv)
			continue
		}
		key := kv[:i]
		seen[key] = len(merged)
		merged = append(merged, kv)
	}
	for k, v := range over {
		entry := k + "=" + v
		if idx, ok := seen[k]; ok {
			merged[idx] = entry
		} else {
			merged = append(merged, entry)
		}
	}
	return merged
}
