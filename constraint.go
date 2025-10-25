package gozdd

import (
	"context"
	"fmt"
)

// Constraint represents a single constraint that can be evaluated during ZDD construction.
//
// Constraints are composed together to form complete problem specifications.
// Each constraint can validate state transitions and provide early termination
// hints to improve construction performance.
type Constraint interface {
	// Validate checks if a state transition satisfies this constraint.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - state: Current constraint state
	//   - level: Variable level being assigned (1-based)
	//   - take: true if variable is selected, false otherwise
	//
	// Returns an error if the transition violates this constraint,
	// indicating the branch should be pruned.
	Validate(ctx context.Context, state State, level int, take bool) error
	
	// CanPrune provides an early termination hint for optimization.
	//
	// Returns true if the current state cannot lead to any feasible
	// solutions, allowing early pruning of the search tree.
	//
	// This is optional - returning false is always safe but may be less efficient.
	CanPrune(state State, level int) bool
}

// BasicState provides a simple State implementation for common constraint types.
//
// Applications can embed BasicState and add domain-specific fields,
// or implement State directly for full control.
type BasicState struct {
	// Counters tracks counts for different constraint types
	Counters []int
	
	// Flags tracks boolean state for constraints
	Flags []bool
	
	// Sum tracks weighted sums for linear constraints
	Sum float64
}

// Clone creates a deep copy of the BasicState
func (s BasicState) Clone() State {
	counters := make([]int, len(s.Counters))
	copy(counters, s.Counters)
	
	flags := make([]bool, len(s.Flags))
	copy(flags, s.Flags)
	
	return BasicState{
		Counters: counters,
		Flags:    flags,
		Sum:      s.Sum,
	}
}

// Hash computes a hash value for state deduplication
func (s BasicState) Hash() uint64 {
	hash := uint64(0)
	
	// Hash counters
	for i, c := range s.Counters {
		hash = hash*31 + uint64(c)*uint64(i+1)
	}
	
	// Hash flags
	for i, f := range s.Flags {
		if f {
			hash = hash*31 + uint64(i+1)
		}
	}
	
	// Hash sum (convert to int64 for hashing)
	hash = hash*31 + uint64(int64(s.Sum*1000)) // 3 decimal precision
	
	return hash
}

// Equal checks equality with another BasicState
func (s BasicState) Equal(other State) bool {
	o, ok := other.(BasicState)
	if !ok {
		return false
	}
	
	if len(s.Counters) != len(o.Counters) || len(s.Flags) != len(o.Flags) {
		return false
	}
	
	for i, c := range s.Counters {
		if c != o.Counters[i] {
			return false
		}
	}
	
	for i, f := range s.Flags {
		if f != o.Flags[i] {
			return false
		}
	}
	
	// Compare sum with small tolerance for floating point
	diff := s.Sum - o.Sum
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}

// CountConstraint enforces minimum and maximum selection counts.
//
// This constraint tracks how many variables have been selected and
// ensures the total count falls within the specified range.
type CountConstraint struct {
	// Min is the minimum number of variables that must be selected
	Min int
	
	// Max is the maximum number of variables that can be selected
	Max int
	
	// CounterIndex specifies which counter in BasicState to use
	CounterIndex int
}

// Validate checks if the selection count constraint is satisfied
func (c CountConstraint) Validate(ctx context.Context, state State, level int, take bool) error {
	s, ok := state.(BasicState)
	if !ok {
		return fmt.Errorf("%w: CountConstraint requires BasicState", ErrInvalidConstraint)
	}
	
	if c.CounterIndex >= len(s.Counters) {
		return fmt.Errorf("%w: counter index %d out of bounds", ErrInvalidConstraint, c.CounterIndex)
	}
	
	count := s.Counters[c.CounterIndex]
	if take {
		count++
	}
	
	// Check if count exceeds maximum
	if count > c.Max {
		return fmt.Errorf("count %d exceeds maximum %d", count, c.Max)
	}
	
	return nil
}

// CanPrune checks if the current state can still satisfy the minimum count
func (c CountConstraint) CanPrune(state State, level int) bool {
	s, ok := state.(BasicState)
	if !ok {
		return false // Conservative: don't prune if we can't analyze
	}
	
	if c.CounterIndex >= len(s.Counters) {
		return false
	}
	
	count := s.Counters[c.CounterIndex]
	remainingLevels := level
	
	// Check if it's impossible to reach minimum count
	if count+remainingLevels < c.Min {
		return true // Prune: can't reach minimum even if all remaining are selected
	}
	
	return false
}

// SumConstraint enforces minimum and maximum weighted sums.
//
// This constraint is useful for knapsack problems, resource allocation,
// and other optimization problems with linear constraints.
type SumConstraint struct {
	// Weights specifies the weight of each variable (1-based indexing)
	// Weights[0] is ignored, Weights[i] is the weight of variable i
	Weights []float64
	
	// Min is the minimum required sum
	Min float64
	
	// Max is the maximum allowed sum
	Max float64
}

// Validate checks if the weighted sum constraint is satisfied
func (c SumConstraint) Validate(ctx context.Context, state State, level int, take bool) error {
	s, ok := state.(BasicState)
	if !ok {
		return fmt.Errorf("%w: SumConstraint requires BasicState", ErrInvalidConstraint)
	}
	
	if level <= 0 || level >= len(c.Weights) {
		return fmt.Errorf("%w: level %d out of bounds for weights", ErrInvalidConstraint, level)
	}
	
	sum := s.Sum
	if take {
		sum += c.Weights[level]
	}
	
	// Check if sum exceeds maximum
	if sum > c.Max {
		return fmt.Errorf("sum %.3f exceeds maximum %.3f", sum, c.Max)
	}
	
	return nil
}

// CanPrune checks if the current state can still satisfy the minimum sum
func (c SumConstraint) CanPrune(state State, level int) bool {
	s, ok := state.(BasicState)
	if !ok {
		return false
	}
	
	sum := s.Sum
	
	// Calculate maximum possible sum from remaining variables
	maxRemaining := 0.0
	for i := 1; i < level && i < len(c.Weights); i++ {
		if c.Weights[i] > 0 {
			maxRemaining += c.Weights[i]
		}
	}
	
	// Check if it's impossible to reach minimum sum
	if sum+maxRemaining < c.Min {
		return true // Prune: can't reach minimum even with optimal remaining selections
	}
	
	return false
}

// CustomConstraint allows applications to define constraints using functions.
//
// This provides flexibility for constraints that don't fit the built-in types
// while maintaining the same interface.
type CustomConstraint struct {
	// ValidateFunc is called to validate state transitions
	ValidateFunc func(ctx context.Context, state State, level int, take bool) error
	
	// PruneFunc is called to check for early termination (optional)
	PruneFunc func(state State, level int) bool
	
	// Name provides a description for debugging and error messages
	Name string
}

// Validate delegates to the custom validation function
func (c CustomConstraint) Validate(ctx context.Context, state State, level int, take bool) error {
	if c.ValidateFunc == nil {
		return nil // No validation function means always valid
	}
	
	if err := c.ValidateFunc(ctx, state, level, take); err != nil {
		if c.Name != "" {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		return err
	}
	
	return nil
}

// CanPrune delegates to the custom pruning function
func (c CustomConstraint) CanPrune(state State, level int) bool {
	if c.PruneFunc == nil {
		return false // No pruning function means never prune
	}
	
	return c.PruneFunc(state, level)
}

// CompositeConstraintSpec combines multiple constraints into a single specification.
//
// This allows building complex constraint problems by composing simpler constraints.
// All constraints must be satisfied for a solution to be feasible.
type CompositeConstraintSpec struct {
	vars        int
	constraints []Constraint
	initialState State
}

// NewCompositeSpec creates a new composite constraint specification.
//
// Parameters:
//   - vars: Number of decision variables
//   - initialState: Starting state for constraint evaluation
//   - constraints: List of constraints that must all be satisfied
//
// The initialState is cloned for each ZDD construction, so it's safe to reuse
// the same spec for multiple ZDD builds.
func NewCompositeSpec(vars int, initialState State, constraints ...Constraint) *CompositeConstraintSpec {
	return &CompositeConstraintSpec{
		vars:         vars,
		constraints:  constraints,
		initialState: initialState,
	}
}

// Variables returns the number of decision variables
func (c *CompositeConstraintSpec) Variables() int {
	return c.vars
}

// InitialState returns a clone of the initial state
func (c *CompositeConstraintSpec) InitialState() State {
	return c.initialState.Clone()
}

// GetChild applies all constraints to compute the new state after variable assignment.
//
// The method:
//   1. Clones the current state
//   2. Updates the state based on the variable assignment
//   3. Validates the transition against all constraints
//   4. Returns the new state or an error if any constraint is violated
func (c *CompositeConstraintSpec) GetChild(ctx context.Context, state State, level int, take bool) (State, error) {
	// Clone state for the new branch
	newState := state.Clone()
	
	// Update state based on assignment (for BasicState)
	if bs, ok := newState.(BasicState); ok {
		// Update counters and sum for built-in constraints
		if take && len(bs.Counters) > 0 {
			bs.Counters[0]++ // Default counter for selections
		}
		
		// Applications can extend this logic or use CustomConstraint
		// for more complex state updates
		newState = bs
	}
	
	// Validate against all constraints
	for i, constraint := range c.constraints {
		if err := constraint.Validate(ctx, newState, level, take); err != nil {
			return nil, fmt.Errorf("constraint %d: %w", i, err)
		}
		
		// Check for early pruning
		if constraint.CanPrune(newState, level-1) {
			return nil, fmt.Errorf("constraint %d: branch pruned", i)
		}
	}
	
	return newState, nil
}

// IsValid checks if the final state satisfies all constraints.
//
// This is called when ZDD construction reaches a terminal state.
// For most constraints, validation during GetChild is sufficient,
// but some constraints may need final validation (e.g., minimum counts).
func (c *CompositeConstraintSpec) IsValid(state State) bool {
	// For BasicState, check minimum count constraints
	if bs, ok := state.(BasicState); ok {
		// This is a simplified check - applications should implement
		// proper final validation in their constraints
		if len(bs.Counters) > 0 {
			// Example: ensure at least one variable was selected
			return bs.Counters[0] > 0
		}
	}
	
	return true // Default: assume valid if no specific validation needed
}
