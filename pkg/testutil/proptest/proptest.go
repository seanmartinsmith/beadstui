// Package proptest provides property-based testing utilities for verifying
// that optimized implementations produce identical results to original implementations.
//
// This package uses pgregory.net/rapid for property-based testing, enabling
// automatic generation of test inputs and shrinking of failing cases.
//
// Example usage:
//
//	func TestOptimization_Isomorphic(t *testing.T) {
//		proptest.CompareImplementations(t,
//			"GetActionableIssues",
//			func(t *rapid.T) []model.Issue { return genIssues(t) },
//			oldGetActionableIssues,
//			newGetActionableIssues,
//			issuesEqual,
//		)
//	}
package proptest

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"pgregory.net/rapid"
)

// CompareImplementations tests that two implementations produce identical results
// for randomly generated inputs. This is the primary tool for verifying that an
// optimization is "isomorphic" (produces the same outputs for the same inputs).
//
// Parameters:
//   - t: testing.T instance
//   - name: descriptive name for the comparison (appears in failure messages)
//   - genInput: rapid generator that produces test inputs
//   - oldImpl: the original implementation (known-good reference)
//   - newImpl: the new/optimized implementation (under test)
//   - equal: comparison function (returns true if outputs are equivalent)
//
// The test will run many iterations with different generated inputs. If any
// input produces different outputs from the two implementations, the test fails
// and rapid will attempt to shrink the input to a minimal failing case.
func CompareImplementations[I, O any](
	t *testing.T,
	name string,
	genInput func(*rapid.T) I,
	oldImpl func(I) O,
	newImpl func(I) O,
	equal func(O, O) bool,
) {
	t.Helper()
	rapid.Check(t, func(rt *rapid.T) {
		input := genInput(rt)
		oldOut := oldImpl(input)
		newOut := newImpl(input)
		if !equal(oldOut, newOut) {
			t.Fatalf("%s: implementations differ\ninput: %+v\nold: %+v\nnew: %+v",
				name, input, oldOut, newOut)
		}
	})
}

// CompareImplementationsWithError tests implementations that return (result, error).
// Both the result and error must match for the test to pass.
func CompareImplementationsWithError[I, O any](
	t *testing.T,
	name string,
	genInput func(*rapid.T) I,
	oldImpl func(I) (O, error),
	newImpl func(I) (O, error),
	equal func(O, O) bool,
) {
	t.Helper()
	rapid.Check(t, func(rt *rapid.T) {
		input := genInput(rt)
		oldOut, oldErr := oldImpl(input)
		newOut, newErr := newImpl(input)

		// Check error consistency
		if (oldErr == nil) != (newErr == nil) {
			t.Fatalf("%s: error behavior differs\ninput: %+v\nold error: %v\nnew error: %v",
				name, input, oldErr, newErr)
		}

		// If both errored, we're done (don't compare outputs)
		if oldErr != nil {
			return
		}

		// Compare outputs
		if !equal(oldOut, newOut) {
			t.Fatalf("%s: implementations differ\ninput: %+v\nold: %+v\nnew: %+v",
				name, input, oldOut, newOut)
		}
	})
}

// CompareJSON tests that two implementations produce JSON-equivalent outputs.
// This is useful when the outputs may have different Go representations but
// should serialize to identical JSON.
func CompareJSON[I, O any](
	t *testing.T,
	name string,
	genInput func(*rapid.T) I,
	oldImpl func(I) O,
	newImpl func(I) O,
) {
	t.Helper()
	CompareImplementations(t, name, genInput, oldImpl, newImpl, JSONEqual[O])
}

// ============================================================================
// Comparison Functions
// ============================================================================

// DeepEqual uses reflect.DeepEqual for comparison.
// Suitable for most struct comparisons.
func DeepEqual[T any](a, b T) bool {
	return reflect.DeepEqual(a, b)
}

// JSONEqual compares values by their JSON serialization.
// Useful when order of map keys or slice elements doesn't matter.
func JSONEqual[T any](a, b T) bool {
	ja, err := json.Marshal(a)
	if err != nil {
		return false
	}
	jb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ja) == string(jb)
}

// SliceEqual compares slices element-by-element using the provided comparison.
func SliceEqual[T any](equal func(T, T) bool) func([]T, []T) bool {
	return func(a, b []T) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if !equal(a[i], b[i]) {
				return false
			}
		}
		return true
	}
}

// UnorderedSliceEqual compares slices without regard to order.
// Elements must be JSON-serializable for comparison.
func UnorderedSliceEqual[T any](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	// Serialize and sort for comparison
	sa := serializeAndSort(a)
	sb := serializeAndSort(b)
	return reflect.DeepEqual(sa, sb)
}

func serializeAndSort[T any](items []T) []string {
	result := make([]string, len(items))
	for i, item := range items {
		data, _ := json.Marshal(item)
		result[i] = string(data)
	}
	sort.Strings(result)
	return result
}

// MapEqual compares maps using reflect.DeepEqual.
func MapEqual[K comparable, V any](a, b map[K]V) bool {
	return reflect.DeepEqual(a, b)
}

// FloatEqual compares floats with a tolerance for floating-point precision issues.
func FloatEqual(tolerance float64) func(float64, float64) bool {
	return func(a, b float64) bool {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff <= tolerance
	}
}

// ============================================================================
// Input Generators
// ============================================================================

// IntRange generates integers in the range [min, max].
func IntRange(min, max int) func(*rapid.T) int {
	return func(t *rapid.T) int {
		return rapid.IntRange(min, max).Draw(t, "int")
	}
}

// StringOfN generates strings of length n using alphanumeric characters.
func StringOfN(n int) func(*rapid.T) string {
	return func(t *rapid.T) string {
		return rapid.StringN(n, n, -1).Draw(t, "string")
	}
}

// SliceOfN generates slices of length n using the provided generator.
func SliceOfN[T any](n int, gen func(*rapid.T) T) func(*rapid.T) []T {
	return func(t *rapid.T) []T {
		result := make([]T, n)
		for i := range result {
			result[i] = gen(t)
		}
		return result
	}
}

// SliceOfRange generates slices with length in [min, max].
func SliceOfRange[T any](min, max int, gen func(*rapid.T) T) func(*rapid.T) []T {
	return func(t *rapid.T) []T {
		n := rapid.IntRange(min, max).Draw(t, "slice_len")
		result := make([]T, n)
		for i := range result {
			result[i] = gen(t)
		}
		return result
	}
}

// MapOfN generates maps with n entries.
func MapOfN[K comparable, V any](n int, genKey func(*rapid.T) K, genVal func(*rapid.T) V) func(*rapid.T) map[K]V {
	return func(t *rapid.T) map[K]V {
		result := make(map[K]V, n)
		for i := 0; i < n; i++ {
			k := genKey(t)
			v := genVal(t)
			result[k] = v
		}
		return result
	}
}

// OneOf returns a generator that picks from the provided options.
func OneOf[T any](options ...T) func(*rapid.T) T {
	return func(t *rapid.T) T {
		idx := rapid.IntRange(0, len(options)-1).Draw(t, "index")
		return options[idx]
	}
}

// ============================================================================
// Test Harness
// ============================================================================

// TestCase represents a single property test case.
type TestCase[I, O any] struct {
	Name     string
	GenInput func(*rapid.T) I
	OldImpl  func(I) O
	NewImpl  func(I) O
	Equal    func(O, O) bool
}

// RunAll runs multiple property test cases.
func RunAll[I, O any](t *testing.T, cases []TestCase[I, O]) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			CompareImplementations(t, tc.Name, tc.GenInput, tc.OldImpl, tc.NewImpl, tc.Equal)
		})
	}
}

// ============================================================================
// Benchmarking Support
// ============================================================================

// BenchmarkComparison benchmarks both implementations to verify the optimization
// actually improves performance.
func BenchmarkComparison[I, O any](
	b *testing.B,
	name string,
	input I,
	oldImpl func(I) O,
	newImpl func(I) O,
) {
	b.Run(name+"_old", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = oldImpl(input)
		}
	})
	b.Run(name+"_new", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = newImpl(input)
		}
	})
}
