package main

import (
	"sort"
	"testing"
)

type testset struct {
	base     []string
	value    string
	expected []string
}

// Test data of good and bad solutions
var tests = []testset{
	{[]string{"one", "two", "three"}, "one", []string{"one", "two", "three"}},
	{[]string{"one"}, "one", []string{"one"}},
	{[]string{"one"}, "two", []string{"one", "two"}},
	{[]string{"one", "two", "three"}, "four", []string{"one", "two", "three", "four"}},
}

// Tests go below here

func TestUniqAdd(t *testing.T) {
	// Pass in a messy list and a value, then test whether the array comes back
	// more or less with the unique string combo.
	for _, test := range tests {
		v := uniqAdd(test.base, test.value)
		if !equalSlices(v, test.expected) {
			t.Error(
				"For", test.base,
				"adding", test.value,
				"expected", test.expected,
				"got", v,
			)
		}
	}

}

// END Tests

// Utility functions below here

func equalSlices(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
