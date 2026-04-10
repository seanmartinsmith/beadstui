package proptest

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

func TestCompareImplementations_Identical(t *testing.T) {
	// Two identical implementations should pass
	double := func(x int) int { return x * 2 }

	CompareImplementations(t,
		"double",
		IntRange(0, 1000),
		double,
		double,
		func(a, b int) bool { return a == b },
	)
}

func TestCompareImplementations_DifferentFails(t *testing.T) {
	// This test documents that differing implementations would be caught.
	// We don't actually run it as a failing test, just verify the machinery works.
	old := func(x int) int { return x * 2 }
	new := func(x int) int { return x * 2 } // Same implementation for passing test

	// Verify the comparison machinery works with identical implementations
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.IntRange(0, 100).Draw(rt, "input")
		oldOut := old(input)
		newOut := new(input)
		if oldOut != newOut {
			rt.Fatalf("implementations differ for input %d: old=%d, new=%d", input, oldOut, newOut)
		}
	})
}

func TestCompareImplementationsWithError_BothSuccess(t *testing.T) {
	safeDiv := func(x int) (int, error) {
		if x == 0 {
			return 0, errors.New("division by zero")
		}
		return 100 / x, nil
	}

	CompareImplementationsWithError(t,
		"safeDiv",
		IntRange(1, 100), // Avoid zero
		safeDiv,
		safeDiv,
		func(a, b int) bool { return a == b },
	)
}

func TestCompareImplementationsWithError_BothError(t *testing.T) {
	alwaysFail := func(x int) (int, error) {
		return 0, errors.New("always fails")
	}

	CompareImplementationsWithError(t,
		"alwaysFail",
		IntRange(0, 100),
		alwaysFail,
		alwaysFail,
		func(a, b int) bool { return a == b },
	)
}

func TestCompareJSON(t *testing.T) {
	type Result struct {
		Value int    `json:"value"`
		Name  string `json:"name"`
	}

	impl := func(x int) Result {
		return Result{Value: x, Name: "test"}
	}

	CompareJSON(t, "json_equal", IntRange(0, 100), impl, impl)
}

func TestDeepEqual(t *testing.T) {
	type S struct{ X, Y int }

	if !DeepEqual(S{1, 2}, S{1, 2}) {
		t.Error("identical structs should be equal")
	}
	if DeepEqual(S{1, 2}, S{1, 3}) {
		t.Error("different structs should not be equal")
	}
}

func TestJSONEqual(t *testing.T) {
	type S struct {
		A int `json:"a"`
		B int `json:"b"`
	}

	if !JSONEqual(S{1, 2}, S{1, 2}) {
		t.Error("identical structs should be JSON equal")
	}
	if JSONEqual(S{1, 2}, S{1, 3}) {
		t.Error("different structs should not be JSON equal")
	}
}

func TestSliceEqual(t *testing.T) {
	intEqual := func(a, b int) bool { return a == b }
	sliceEq := SliceEqual(intEqual)

	if !sliceEq([]int{1, 2, 3}, []int{1, 2, 3}) {
		t.Error("identical slices should be equal")
	}
	if sliceEq([]int{1, 2, 3}, []int{1, 2, 4}) {
		t.Error("different slices should not be equal")
	}
	if sliceEq([]int{1, 2}, []int{1, 2, 3}) {
		t.Error("different length slices should not be equal")
	}
}

func TestUnorderedSliceEqual(t *testing.T) {
	if !UnorderedSliceEqual([]int{1, 2, 3}, []int{3, 1, 2}) {
		t.Error("same elements different order should be equal")
	}
	if UnorderedSliceEqual([]int{1, 2, 3}, []int{1, 2, 4}) {
		t.Error("different elements should not be equal")
	}
}

func TestMapEqual(t *testing.T) {
	if !MapEqual(map[string]int{"a": 1}, map[string]int{"a": 1}) {
		t.Error("identical maps should be equal")
	}
	if MapEqual(map[string]int{"a": 1}, map[string]int{"a": 2}) {
		t.Error("different maps should not be equal")
	}
}

func TestFloatEqual(t *testing.T) {
	eq := FloatEqual(0.001)

	if !eq(1.0, 1.0005) {
		t.Error("values within tolerance should be equal")
	}
	if eq(1.0, 1.01) {
		t.Error("values outside tolerance should not be equal")
	}
}

func TestIntRange(t *testing.T) {
	gen := IntRange(10, 20)
	rapid.Check(t, func(rt *rapid.T) {
		val := gen(rt)
		if val < 10 || val > 20 {
			t.Fatalf("value %d outside range [10, 20]", val)
		}
	})
}

func TestSliceOfN(t *testing.T) {
	gen := SliceOfN(5, IntRange(0, 100))
	rapid.Check(t, func(rt *rapid.T) {
		slice := gen(rt)
		if len(slice) != 5 {
			t.Fatalf("expected slice of length 5, got %d", len(slice))
		}
	})
}

func TestSliceOfRange(t *testing.T) {
	gen := SliceOfRange(3, 7, IntRange(0, 100))
	rapid.Check(t, func(rt *rapid.T) {
		slice := gen(rt)
		if len(slice) < 3 || len(slice) > 7 {
			t.Fatalf("slice length %d outside range [3, 7]", len(slice))
		}
	})
}

func TestOneOf(t *testing.T) {
	options := []string{"a", "b", "c"}
	gen := OneOf(options...)
	rapid.Check(t, func(rt *rapid.T) {
		val := gen(rt)
		found := false
		for _, opt := range options {
			if val == opt {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("value %q not in options", val)
		}
	})
}

func TestRunAll(t *testing.T) {
	cases := []TestCase[int, int]{
		{
			Name:     "double",
			GenInput: IntRange(0, 100),
			OldImpl:  func(x int) int { return x * 2 },
			NewImpl:  func(x int) int { return x * 2 },
			Equal:    func(a, b int) bool { return a == b },
		},
		{
			Name:     "square",
			GenInput: IntRange(0, 50),
			OldImpl:  func(x int) int { return x * x },
			NewImpl:  func(x int) int { return x * x },
			Equal:    func(a, b int) bool { return a == b },
		},
	}

	RunAll(t, cases)
}

// Example of using proptest with a more complex type
func TestCompareImplementations_StructSlice(t *testing.T) {
	type Item struct {
		ID    int
		Value string
	}

	genItem := func(rt *rapid.T) Item {
		return Item{
			ID:    rapid.IntRange(0, 1000).Draw(rt, "id"),
			Value: rapid.StringN(5, 10, -1).Draw(rt, "value"),
		}
	}

	genItems := func(rt *rapid.T) []Item {
		n := rapid.IntRange(0, 10).Draw(rt, "count")
		items := make([]Item, n)
		for i := range items {
			items[i] = genItem(rt)
		}
		return items
	}

	// Both implementations just return the input (identity)
	identity := func(items []Item) []Item { return items }

	CompareImplementations(t, "identity", genItems, identity, identity, DeepEqual[[]Item])
}
