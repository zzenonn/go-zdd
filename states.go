package gozdd

import (
	"fmt"
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

// MapState provides a flexible State implementation using key-value pairs.
//
// This state type is suitable for complex problems where the constraint state
// has heterogeneous data or dynamic structure that doesn't fit fixed arrays.
// Use this for problems with variable relationships, mixed data types, or
// when you need key-based lookups during constraint evaluation.
type MapState struct {
	Data map[string]interface{}
}

// NewMapState creates a new MapState with optional initial key-value pairs.
//
// Parameters should be provided as alternating key-value pairs:
//   state := NewMapState("count", 0, "weight", 15.5, "active", true)
func NewMapState(pairs ...interface{}) *MapState {
	data := make(map[string]interface{})
	
	// Accept pairs as key, value, key, value, ...
	for i := 0; i < len(pairs)-1; i += 2 {
		if key, ok := pairs[i].(string); ok {
			data[key] = pairs[i+1]
		}
	}
	
	return &MapState{Data: data}
}

// Clone creates a deep copy of the MapState
func (s *MapState) Clone() State {
	data := make(map[string]interface{})
	for k, v := range s.Data {
		// Shallow copy of values - applications should use immutable values
		// or implement custom deep copying for complex nested structures
		data[k] = v
	}
	return &MapState{Data: data}
}

// Hash computes a hash value for state deduplication
func (s *MapState) Hash() uint64 {
	h := fnv.New64a()
	
	// Sort keys for consistent hashing
	keys := make([]string, 0, len(s.Data))
	for k := range s.Data {
		keys = append(keys, k)
	}
	
	// Simple sort for minimal implementation
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	
	// Hash key-value pairs in sorted order
	for _, k := range keys {
		h.Write([]byte(k))
		
		// Hash value based on type
		switch v := s.Data[k].(type) {
		case int:
			h.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
		case float64:
			intVal := int64(v * 1000000)
			h.Write([]byte{
				byte(intVal), byte(intVal >> 8), byte(intVal >> 16), byte(intVal >> 24),
				byte(intVal >> 32), byte(intVal >> 40), byte(intVal >> 48), byte(intVal >> 56),
			})
		case string:
			h.Write([]byte(v))
		case bool:
			if v {
				h.Write([]byte{1})
			} else {
				h.Write([]byte{0})
			}
		default:
			// For other types, convert to string and hash
			h.Write([]byte(fmt.Sprintf("%v", v)))
		}
	}
	
	return h.Sum64()
}

// Equal checks equality with another MapState
func (s *MapState) Equal(other State) bool {
	o, ok := other.(*MapState)
	if !ok {
		return false
	}
	
	if len(s.Data) != len(o.Data) {
		return false
	}
	
	for k, v := range s.Data {
		otherV, exists := o.Data[k]
		if !exists {
			return false
		}
		
		// Type-specific equality
		switch val := v.(type) {
		case int:
			if otherVal, ok := otherV.(int); !ok || val != otherVal {
				return false
			}
		case float64:
			if otherVal, ok := otherV.(float64); !ok {
				return false
			} else {
				diff := val - otherVal
				if diff < 0 {
					diff = -diff
				}
				if diff > 1e-9 {
					return false
				}
			}
		case string:
			if otherVal, ok := otherV.(string); !ok || val != otherVal {
				return false
			}
		case bool:
			if otherVal, ok := otherV.(bool); !ok || val != otherVal {
				return false
			}
		default:
			// For other types, use interface{} equality
			if v != otherV {
				return false
			}
		}
	}
	
	return true
}

// SkipState wraps a state and indicates ZDD construction should skip to a specific level.
//
// This optimization is critical for problems with logical dependencies where certain
// variable assignments make subsequent variables irrelevant. For example, in the TripS
// data center problem, when P_{kt} = 0 (don't place storage), all related T_{jkt} and
// B_{ijkt} variables must be 0, so construction can skip those levels entirely.
type SkipState struct {
	State  State // The actual constraint state
	SkipTo int   // 1-based level to skip to (must be < current level)
}

// NewSkipState creates a SkipState that will cause construction to jump to the specified level.
func NewSkipState(state State, skipTo int) *SkipState {
	return &SkipState{State: state, SkipTo: skipTo}
}

// Clone creates a deep copy of the SkipState
func (s *SkipState) Clone() State {
	return &SkipState{State: s.State.Clone(), SkipTo: s.SkipTo}
}

// Hash delegates to the wrapped state's hash
func (s *SkipState) Hash() uint64 {
	return s.State.Hash()
}

// Equal checks equality with another State, handling SkipState comparison
func (s *SkipState) Equal(other State) bool {
	if otherSkip, ok := other.(*SkipState); ok {
		return s.SkipTo == otherSkip.SkipTo && s.State.Equal(otherSkip.State)
	}
	return false
}
