// Package gozdd provides a Go-native Zero-suppressed Decision Diagram (ZDD) library
// for constraint-based optimization problems.
//
// # Overview
//
// Zero-suppressed Decision Diagrams (ZDDs) are a specialized form of Binary Decision
// Diagrams optimized for representing sparse sets and constraint satisfaction problems.
// This library provides a unified ZDD engine that eliminates the complexity of using
// multiple libraries while offering Go-idiomatic interfaces.
//
// # Key Features
//
//   - Unified ZDD construction and evaluation in a single library
//   - Context-aware operations with timeout and cancellation support
//   - Configurable parallelism for large-scale problems
//   - Automatic node deduplication and ZDD reduction rules
//   - Interface-based constraint framework for domain flexibility
//   - Memory-efficient node management with optional pooling
//
// # Basic Usage
//
// To use this library, implement the ConstraintSpec interface for your problem domain:
//
//	type MyConstraintSpec struct {
//	    vars int
//	    // ... problem-specific fields
//	}
//
//	func (s MyConstraintSpec) Variables() int { return s.vars }
//	func (s MyConstraintSpec) InitialState() State { /* ... */ }
//	func (s MyConstraintSpec) GetChild(ctx context.Context, state State, level int, take bool) (State, error) { /* ... */ }
//	func (s MyConstraintSpec) IsValid(state State) bool { /* ... */ }
//
// Then create and build a ZDD:
//
//	zdd := NewZDD(10, WithParallel(4), WithTimeout(time.Minute))
//	spec := MyConstraintSpec{vars: 10}
//	
//	ctx := context.Background()
//	if err := zdd.Build(ctx, spec); err != nil {
//	    log.Fatal(err)
//	}
//	
//	fmt.Printf("ZDD has %d nodes representing all feasible solutions\n", zdd.Size())
//
// # Performance Considerations
//
// For optimal performance:
//
//   - Implement efficient Hash() and Equal() methods for State objects
//   - Use WithParallel() for problems with independent constraint evaluation
//   - Consider memory pooling for frequently allocated State objects
//   - Order variables to minimize ZDD size (problem-dependent)
package gozdd

import (
	"context"
	"fmt"
)

// State represents the constraint state during ZDD construction.
//
// Applications must implement this interface to define their problem-specific
// state representation. The state tracks constraint satisfaction progress
// as the ZDD construction proceeds through variable assignments.
//
// Implementations should ensure:
//   - Clone() creates a deep copy for branching
//   - Hash() provides consistent hashing for deduplication
//   - Equal() implements proper equality semantics
//   - State objects are immutable after creation
type State interface {
	// Clone creates a deep copy of the state for branching during construction.
	// The returned state should be independent of the original.
	Clone() State
	
	// Hash returns a hash value for state deduplication.
	// States with identical hash values will be compared using Equal().
	// The hash should be consistent across multiple calls.
	Hash() uint64
	
	// Equal returns true if this state is equivalent to another state.
	// Equivalent states can be merged during ZDD construction.
	// This method should be symmetric and transitive.
	Equal(other State) bool
}

// ConstraintSpec defines the problem specification for ZDD construction.
//
// Applications implement this interface to specify:
//   - The number of decision variables
//   - The initial constraint state
//   - State transition logic for variable assignments
//   - Feasibility validation for terminal states
//
// The ZDD construction algorithm calls these methods to explore
// the solution space while respecting problem constraints.
type ConstraintSpec interface {
	// Variables returns the total number of decision variables.
	// Variables are numbered from 1 to Variables() inclusive.
	Variables() int
	
	// InitialState returns the starting state for ZDD construction.
	// This represents the constraint state before any variables are assigned.
	InitialState() State
	
	// GetChild computes the new state after assigning a variable.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout handling
	//   - state: Current constraint state
	//   - level: Variable level being assigned (1-based)
	//   - take: true if variable is selected, false if not selected
	//
	// Returns:
	//   - New state after the assignment
	//   - Error if the assignment violates constraints (prunes this branch)
	//
	// Returning an error indicates this assignment path is infeasible
	// and should be pruned from the ZDD.
	GetChild(ctx context.Context, state State, level int, take bool) (State, error)
	
	// IsValid checks if a state represents a feasible solution.
	// Called when construction reaches a terminal state (all variables assigned).
	//
	// Returns true if the state satisfies all problem constraints,
	// false if the solution is infeasible.
	IsValid(state State) bool
}

// ZDD represents a Zero-suppressed Decision Diagram for constraint optimization.
//
// A ZDD compactly represents all feasible solutions to a constraint satisfaction
// problem. It supports:
//   - Efficient construction from constraint specifications
//   - Automatic structural sharing and reduction
//   - Context-aware operations with timeout and cancellation
//   - Configurable parallelism and memory limits
//
// ZDDs are immutable after construction. To modify constraints,
// create a new ZDD instance.
type ZDD struct {
	// root is the NodeID of the root node
	root NodeID
	
	// nodes manages all ZDD nodes with deduplication
	nodes *NodeTable
	
	// vars is the number of decision variables
	vars int
	
	// reduced indicates if ZDD reduction has been applied
	reduced bool
	
	// config holds construction parameters
	config *Config
}

// NewZDD creates a new ZDD with the specified number of variables.
//
// The ZDD is initially empty (no constraints). Use Build() to construct
// the ZDD from a constraint specification.
//
// Parameters:
//   - vars: Number of decision variables (must be >= 0)
//   - opts: Configuration options (WithParallel, WithMemoryLimit, etc.)
//
// If vars < 0, it is treated as 0 (empty problem).
//
// Example:
//   zdd := NewZDD(10, WithParallel(4), WithTimeout(time.Minute))
func NewZDD(vars int, opts ...Option) *ZDD {
	if vars < 0 {
		vars = 0
	}
	
	return &ZDD{
		root:    NullNode,
		nodes:   NewNodeTable(),
		vars:    vars,
		reduced: false,
		config:  newConfig(opts...),
	}
}

// Build constructs the ZDD from a constraint specification using recursive
// top-down construction.
//
// This method implements the breadth-first ZDD construction algorithm by
// processing variables in order from highest to lowest level. The algorithm:
//   1. Starts with the initial state at the root
//   2. For each variable level, explores both assignment choices
//   3. Applies constraint transitions via GetChild()
//   4. Prunes infeasible branches automatically
//   5. Shares equivalent states for structural compression
//
// Parameters:
//   - ctx: Context for cancellation and timeout handling
//   - spec: Constraint specification defining the problem
//
// Returns an error if:
//   - spec.Variables() != ZDD variables (mismatched problem size)
//   - Construction times out (if WithTimeout was used)
//   - Context is cancelled
//   - Constraint evaluation fails
//
// After successful construction, the ZDD represents all feasible solutions
// to the constraint problem.
func (z *ZDD) Build(ctx context.Context, spec ConstraintSpec) error {
	if spec.Variables() != z.vars {
		return fmt.Errorf("spec variables (%d) != ZDD variables (%d)", spec.Variables(), z.vars)
	}
	
	// Apply timeout if configured
	if z.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, z.config.Timeout)
		defer cancel()
	}
	
	// Build ZDD recursively from top level down
	root, err := z.buildRecursive(ctx, spec, spec.InitialState(), z.vars)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	
	z.root = root
	return nil
}

// buildRecursive implements the TdZdd-style ZDD construction algorithm.
// This matches the construction process used in TripS-ZDD for optimal performance.
func (z *ZDD) buildRecursive(ctx context.Context, spec ConstraintSpec, state State, level int) (NodeID, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return NullNode, ctx.Err()
	default:
	}
	
	// Terminal case: all variables processed
	if level == 0 {
		if spec.IsValid(state) {
			return OneNode, nil
		}
		return ZeroNode, nil
	}
	
	// Check for state deduplication using hash-based memoization
	if existingNode := z.nodes.LookupState(state, level); existingNode != NullNode {
		return existingNode, nil
	}
	
	// Explore 0-arc: variable NOT selected (lo branch)
	var lo NodeID
	loState, err := spec.GetChild(ctx, state, level, false)
	if err != nil {
		// Constraint violation - prune this branch
		lo = ZeroNode
	} else {
		// Handle level skipping optimization
		if skipState, ok := loState.(*SkipState); ok {
			// Skip directly to target level without recursive calls
			if skipState.SkipTo <= 0 {
				// Skip to terminal - check validity
				if spec.IsValid(skipState.State) {
					lo = OneNode
				} else {
					lo = ZeroNode
				}
			} else {
				// Skip to intermediate level
				lo, err = z.buildRecursive(ctx, spec, skipState.State, skipState.SkipTo)
				if err != nil {
					return NullNode, err
				}
			}
		} else {
			// Normal recursive descent
			lo, err = z.buildRecursive(ctx, spec, loState, level-1)
			if err != nil {
				return NullNode, err
			}
		}
	}
	
	// Explore 1-arc: variable IS selected (hi branch)
	var hi NodeID
	hiState, err := spec.GetChild(ctx, state, level, true)
	if err != nil {
		// Constraint violation - prune this branch
		hi = ZeroNode
	} else {
		// Handle level skipping optimization
		if skipState, ok := hiState.(*SkipState); ok {
			// Skip directly to target level without recursive calls
			if skipState.SkipTo <= 0 {
				// Skip to terminal - check validity
				if spec.IsValid(skipState.State) {
					hi = OneNode
				} else {
					hi = ZeroNode
				}
			} else {
				// Skip to intermediate level
				hi, err = z.buildRecursive(ctx, spec, skipState.State, skipState.SkipTo)
				if err != nil {
					return NullNode, err
				}
			}
		} else {
			// Normal recursive descent
			hi, err = z.buildRecursive(ctx, spec, hiState, level-1)
			if err != nil {
				return NullNode, err
			}
		}
	}
	
	// Create node with ZDD reduction rules
	node := z.nodes.AddNode(level, lo, hi)
	
	// Cache the result for state deduplication
	z.nodes.CacheState(state, level, node)
	
	return node, nil
}

// Root returns the NodeID of the ZDD root node.
//
// Returns NullNode if the ZDD has not been constructed yet.
// The root node represents the starting point for solution enumeration.
func (z *ZDD) Root() NodeID {
	return z.root
}

// Size returns the total number of nodes in the ZDD.
//
// This includes terminal nodes but excludes the null node.
// The size reflects the structural complexity of the constraint problem.
// Larger sizes indicate more complex solution spaces.
func (z *ZDD) Size() int {
	return z.nodes.Size()
}

// Variables returns the number of decision variables in the ZDD.
//
// This value is set during NewZDD() and cannot be changed.
// It must match the Variables() returned by the ConstraintSpec.
func (z *ZDD) Variables() int {
	return z.vars
}

// IsReduced returns true if the ZDD is in reduced canonical form.
//
// Currently always returns false since explicit reduction is not implemented.
// The ZDD construction automatically applies basic reduction rules during
// node creation, but full reduction requires additional algorithms.
func (z *ZDD) IsReduced() bool {
	return z.reduced
}

// GetNode retrieves a node by its ID with validation.
//
// This method provides safe access to ZDD nodes for traversal and analysis.
// Returns ErrInvalidNode if the ID is invalid or out of bounds.
//
// Example usage for ZDD traversal:
//   node, err := zdd.GetNode(zdd.Root())
//   if err != nil { /* handle error */ }
//   // Process node.Lo and node.Hi arcs
func (z *ZDD) GetNode(id NodeID) (Node, error) {
	return z.nodes.GetNode(id)
}

// Count returns the total number of solutions in the ZDD.
//
// This is a type-safe convenience method that eliminates the need for
// type assertions when counting solutions.
func (z *ZDD) Count(ctx context.Context) (int64, error) {
	result, err := EvaluateZDD(ctx, z, CountEvaluator{})
	if err != nil {
		return 0, err
	}
	return result.(int64), nil
}

// FindKBest finds the k best solutions with lowest costs.
//
// This is a type-safe convenience method that eliminates the need for
// type assertions when finding optimal solutions.
//
// For k=1, this finds the single optimal solution.
// For k>1, this finds the top k solutions ranked by cost.
func (z *ZDD) FindKBest(ctx context.Context, k int, costs []float64) ([]*Solution, error) {
	result, err := EvaluateZDD(ctx, z, KBestEvaluator{K: k, Costs: costs})
	if err != nil {
		return nil, err
	}
	
	kbest := result.(KBestResult)
	return kbest.Solutions, nil
}
