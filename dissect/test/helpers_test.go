// this file contains helper functions for *tests*

package test

// sliceDiff returns a list of items that are in a, but not in b
// with O(n) complexity
func sliceDiff[T comparable](a, b []T) (diff []T) {
	// map containing each value in a as key
	// value not needed, so we use struct{} as it takes no memory
	m := make(map[T]struct{}, len(b))

	// fill map
	for _, item := range b {
		m[item] = struct{}{}
	}

	// add to diff if in a, but not in b
	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}
	return
}
