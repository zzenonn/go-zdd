// Package gozdd provides Zero-suppressed Decision Diagram (ZDD) functionality
// for constraint-based optimization problems.
//
// This package implements a unified ZDD engine that supports both top-down
// construction and bottom-up evaluation, eliminating the need for multiple
// libraries while providing Go-idiomatic interfaces for constraint specification
// and solution analysis.
package gozdd

import "errors"

// Core ZDD construction and validation errors.
// These errors can be wrapped with additional context using fmt.Errorf.
var (
	// ErrInvalidVariable indicates a variable index is out of bounds.
	ErrInvalidVariable = errors.New("invalid variable index")
	
	// ErrInvalidLevel indicates a level parameter is invalid (< 0 or > max).
	ErrInvalidLevel = errors.New("invalid level")
	
	// ErrInvalidNode indicates a node ID does not exist in the node table.
	ErrInvalidNode = errors.New("invalid node")
	
	// ErrMemoryLimit indicates the configured memory limit has been exceeded.
	ErrMemoryLimit = errors.New("memory limit exceeded")
	
	// ErrTimeout indicates a construction operation has timed out.
	ErrTimeout = errors.New("operation timeout")
	
	// ErrInfeasible indicates no valid solutions exist for the given constraints.
	ErrInfeasible = errors.New("no feasible solutions")
	
	// ErrInvalidConstraint indicates a constraint specification is malformed.
	ErrInvalidConstraint = errors.New("invalid constraint")
	
	// ErrNotReduced indicates an operation requires a reduced ZDD but the
	// ZDD has not been reduced yet.
	ErrNotReduced = errors.New("ZDD not reduced")
)
