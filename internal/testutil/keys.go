// Package testutil provides shared test helpers used across collector test packages.
package testutil

// DataKeys returns the keys of a map[string]any, useful for error messages in tests.
func DataKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
