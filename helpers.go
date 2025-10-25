package gozdd

import (
	"hash/fnv"
)

// IntState provides a ready-to-use State implementation for integer-based problems.
//
// This state type is suitable for problems where the constraint state can be
// represented as a slice of integers (counters, indices, flags as 0/1, etc.).
type IntState struct {
	Values []int
}

// NewIntState creates a new IntState with the specified initial values.
func NewIntState(values ...int) *IntState {
	vals := make([]int, len(values))
	copy(vals, values)
	return &IntState{Values: vals}
}

// Clone creates a deep copy of the IntState
func (s *IntState) Clone() State {
	values := make([]int, len(s.Values))
	copy(values, s.Values)
	return &IntState{Values: values}
}

// Hash computes a hash value for state deduplication
func (s *IntState) Hash() uint64 {
	h := fnv.New64a()
	for _, v := range s.Values {
		h.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
	}
	return h.Sum64()
}

// Equal checks equality with another IntState
func (s *IntState) Equal(other State) bool {
	o, ok := other.(*IntState)
	if !ok {
		return false
	}
	
	if len(s.Values) != len(o.Values) {
		return false
	}
	
	for i, v := range s.Values {
		if v != o.Values[i] {
			return false
		}
	}
	
	return true
}

// FloatState provides a ready-to-use State implementation for floating-point problems.
//
// This state type is suitable for problems involving weights, costs, capacities,
// or other continuous values that need to be tracked during constraint evaluation.
type FloatState struct {
	Values []float64
}

// NewFloatState creates a new FloatState with the specified initial values.
func NewFloatState(values ...float64) *FloatState {
	vals := make([]float64, len(values))
	copy(vals, values)
	return &FloatState{Values: vals}
}

// Clone creates a deep copy of the FloatState
func (s *FloatState) Clone() State {
	values := make([]float64, len(s.Values))
	copy(values, s.Values)
	return &FloatState{Values: values}
}

// Hash computes a hash value for state deduplication
func (s *FloatState) Hash() uint64 {
	h := fnv.New64a()
	for _, v := range s.Values {
		// Convert to int64 with precision for hashing
		intVal := int64(v * 1000000) // 6 decimal precision
		h.Write([]byte{
			byte(intVal), byte(intVal >> 8), byte(intVal >> 16), byte(intVal >> 24),
			byte(intVal >> 32), byte(intVal >> 40), byte(intVal >> 48), byte(intVal >> 56),
		})
	}
	return h.Sum64()
}

// Equal checks equality with another FloatState
func (s *FloatState) Equal(other State) bool {
	o, ok := other.(*FloatState)
	if !ok {
		return false
	}
	
	if len(s.Values) != len(o.Values) {
		return false
	}
	
	for i, v := range s.Values {
		// Compare with small tolerance for floating point
		diff := v - o.Values[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 1e-9 {
			return false
		}
	}
	
	return true
}
