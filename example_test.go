package gozdd_test

import (
	"context"
	"fmt"
	"log"

	"github.com/zzenonn/go-zdd"
)

// ExampleNewZDD demonstrates basic ZDD creation and usage.
func ExampleNewZDD() {
	zdd := gozdd.NewZDD(3, gozdd.WithParallel(2))
	
	fmt.Printf("Variables: %d\n", zdd.Variables())
	fmt.Printf("Size: %d\n", zdd.Size())
	
	// Output:
	// Variables: 3
	// Size: 2
}

// ExampleIntState demonstrates using IntState for simple problems.
func ExampleIntState() {
	state := gozdd.NewIntState(0, 0) // selections, count
	
	newState := state.Clone().(*gozdd.IntState)
	newState.Values[0] = 1
	newState.Values[1] = 5
	
	fmt.Printf("Original: %v\n", state.Values)
	fmt.Printf("Modified: %v\n", newState.Values)
	
	// Output:
	// Original: [0 0]
	// Modified: [1 5]
}

// ExampleFloatState demonstrates using FloatState for knapsack problems.
func ExampleFloatState() {
	state := gozdd.NewFloatState(0.0, 0.0) // weight, value
	
	newState := state.Clone().(*gozdd.FloatState)
	newState.Values[0] += 2.5
	newState.Values[1] += 10.0
	
	fmt.Printf("Weight: %.1f, Value: %.1f\n", newState.Values[0], newState.Values[1])
	
	// Output:
	// Weight: 2.5, Value: 10.0
}

// ExampleMapState demonstrates using MapState for complex problems.
func ExampleMapState() {
	state := gozdd.NewMapState(
		"count", 0,
		"weight", 15.5,
		"active", true,
	)
	
	count := state.Data["count"].(int)
	weight := state.Data["weight"].(float64)
	
	fmt.Printf("Count: %d, Weight: %.1f\n", count, weight)
	
	// Output:
	// Count: 0, Weight: 15.5
}

// ExampleZDD_Count demonstrates counting solutions.
func ExampleZDD_Count() {
	spec := &SimpleSpec{vars: 2, maxCount: 1}
	
	zdd := gozdd.NewZDD(2)
	ctx := context.Background()
	
	if err := zdd.Build(ctx, spec); err != nil {
		log.Fatal(err)
	}
	
	count, err := zdd.Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Solutions: %d\n", count)
	
	// Output:
	// Solutions: 3
}

// ExampleZDD_FindKBest demonstrates finding optimal solutions.
func ExampleZDD_FindKBest() {
	spec := &SimpleSpec{vars: 2, maxCount: 2}
	
	zdd := gozdd.NewZDD(2)
	ctx := context.Background()
	
	if err := zdd.Build(ctx, spec); err != nil {
		log.Fatal(err)
	}
	
	costs := []float64{0, 1, 2} // Prefer variable 1 over 2
	solutions, err := zdd.FindKBest(ctx, 2, costs)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Found %d solutions\n", len(solutions))
	if len(solutions) > 0 {
		fmt.Printf("Best cost: %.0f\n", solutions[0].Cost)
	}
	
	// Output:
	// Found 2 solutions
	// Best cost: 0
}

// ExampleCustomConstraint demonstrates custom constraint implementation.
func ExampleCustomConstraint() {
	constraint := &gozdd.CustomConstraint{
		Name: "Max 2 selections",
		ValidateFunc: func(ctx context.Context, state gozdd.State, level int, take bool) error {
			s := state.(*gozdd.IntState)
			if take && s.Values[0] >= 2 {
				return fmt.Errorf("too many selections")
			}
			return nil
		},
	}
	
	state := gozdd.NewIntState(2) // Already at limit
	err := constraint.Validate(context.Background(), state, 1, true)
	
	fmt.Printf("Validation: %v\n", err != nil)
	
	// Output:
	// Validation: true
}

// Helper type for examples
type SimpleSpec struct {
	vars     int
	maxCount int
}

func (s *SimpleSpec) Variables() int {
	return s.vars
}

func (s *SimpleSpec) InitialState() gozdd.State {
	return gozdd.NewIntState(0) // selection count
}

func (s *SimpleSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
	st := state.(*gozdd.IntState)
	newState := st.Clone().(*gozdd.IntState)
	
	if take {
		newState.Values[0]++
		if newState.Values[0] > s.maxCount {
			return nil, fmt.Errorf("too many selections")
		}
	}
	
	return newState, nil
}

func (s *SimpleSpec) IsValid(state gozdd.State) bool {
	return true
}
