# Go ZDD

A Go-native Zero-suppressed Decision Diagram (ZDD) library for constraint-based optimization problems.

## Features

- Unified ZDD construction and evaluation in a single library
- Context-aware operations with timeout and cancellation support
- Configurable parallelism for large-scale problems
- Automatic node deduplication and ZDD reduction rules
- Interface-based constraint framework for domain flexibility
- Built-in state types to eliminate boilerplate code
- Type-safe evaluation methods

## Installation

```bash
go get github.com/zzenonn/go-zdd
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/zzenonn/go-zdd"
)

func main() {
    // Create ZDD with 5 variables
    zdd := gozdd.NewZDD(5, gozdd.WithParallel(4))
    
    // Define constraints using built-in types
    spec := gozdd.NewCompositeSpec(5, 
        gozdd.NewFloatState(0, 0), // weight, value
        &gozdd.CountConstraint{Min: 2, Max: 3}, // Select 2-3 items
    )
    
    // Build ZDD
    ctx := context.Background()
    if err := zdd.Build(ctx, spec); err != nil {
        log.Fatal(err)
    }
    
    // Count solutions
    count, err := zdd.Count(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Total solutions: %d\n", count)
}
```

## Built-in State Types

### IntState - For Integer Problems
```go
// Resource allocation: [cpu_used, memory_used, disk_used]
state := gozdd.NewIntState(0, 0, 0)

// Task scheduling: [current_time, completed_tasks]
state := gozdd.NewIntState(0, 0)
```

### FloatState - For Continuous Problems
```go
// Knapsack: [total_weight, total_value]
state := gozdd.NewFloatState(0.0, 0.0)

// Portfolio optimization: [total_risk, total_return]
state := gozdd.NewFloatState(0.0, 0.0)
```

## Constraint Examples

### 1. Knapsack Problem
**Use Case**: Packing items with weight/value constraints

```go
type KnapsackSpec struct {
    items    []Item
    capacity float64
}

func (ks *KnapsackSpec) Variables() int { return len(ks.items) }

func (ks *KnapsackSpec) InitialState() gozdd.State {
    return gozdd.NewFloatState(0, 0) // weight, value
}

func (ks *KnapsackSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
    s := state.(*gozdd.FloatState)
    newState := s.Clone().(*gozdd.FloatState)
    
    if take {
        item := ks.items[level-1]
        newWeight := newState.Values[0] + item.Weight
        
        if newWeight > ks.capacity {
            return nil, fmt.Errorf("capacity exceeded")
        }
        
        newState.Values[0] = newWeight
        newState.Values[1] += item.Value
    }
    
    return newState, nil
}

func (ks *KnapsackSpec) IsValid(state gozdd.State) bool {
    return true // Validation done in GetChild
}
```

### 2. Resource Allocation
**Use Case**: Assigning tasks to servers with CPU/memory limits

```go
type ResourceSpec struct {
    tasks     []Task
    cpuLimit  int
    memLimit  int
}

func (rs *ResourceSpec) InitialState() gozdd.State {
    return gozdd.NewIntState(0, 0) // cpu_used, mem_used
}

func (rs *ResourceSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
    s := state.(*gozdd.IntState)
    newState := s.Clone().(*gozdd.IntState)
    
    if take {
        task := rs.tasks[level-1]
        newCPU := newState.Values[0] + task.CPU
        newMem := newState.Values[1] + task.Memory
        
        if newCPU > rs.cpuLimit || newMem > rs.memLimit {
            return nil, fmt.Errorf("resource limit exceeded")
        }
        
        newState.Values[0] = newCPU
        newState.Values[1] = newMem
    }
    
    return newState, nil
}
```

### 3. Team Selection
**Use Case**: Selecting team members with skill requirements

```go
type TeamSpec struct {
    candidates    []Person
    minDevelopers int
    minDesigners  int
    maxTeamSize   int
}

func (ts *TeamSpec) InitialState() gozdd.State {
    return gozdd.NewIntState(0, 0, 0) // developers, designers, total
}

func (ts *TeamSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
    s := state.(*gozdd.IntState)
    newState := s.Clone().(*gozdd.IntState)
    
    if take {
        person := ts.candidates[level-1]
        
        if person.Role == "Developer" {
            newState.Values[0]++
        } else if person.Role == "Designer" {
            newState.Values[1]++
        }
        newState.Values[2]++
        
        if newState.Values[2] > ts.maxTeamSize {
            return nil, fmt.Errorf("team too large")
        }
    }
    
    return newState, nil
}

func (ts *TeamSpec) IsValid(state gozdd.State) bool {
    s := state.(*gozdd.IntState)
    return s.Values[0] >= ts.minDevelopers && s.Values[1] >= ts.minDesigners
}
```

### 4. Scheduling with Time Windows
**Use Case**: Scheduling meetings with time conflicts

```go
type ScheduleSpec struct {
    meetings []Meeting
}

type Meeting struct {
    Start, End int
    Priority   float64
}

func (ss *ScheduleSpec) InitialState() gozdd.State {
    return gozdd.NewIntState() // No initial conflicts
}

func (ss *ScheduleSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
    if !take {
        return state, nil
    }
    
    s := state.(*gozdd.IntState)
    newState := s.Clone().(*gozdd.IntState)
    
    currentMeeting := ss.meetings[level-1]
    
    // Check for conflicts with already scheduled meetings
    for _, scheduledEnd := range s.Values {
        if scheduledEnd > currentMeeting.Start && scheduledEnd < currentMeeting.End {
            return nil, fmt.Errorf("time conflict")
        }
    }
    
    // Add this meeting's end time
    newState.Values = append(newState.Values, currentMeeting.End)
    
    return newState, nil
}
```

### 5. Portfolio Optimization
**Use Case**: Selecting investments with risk/return constraints

```go
type PortfolioSpec struct {
    assets    []Asset
    maxRisk   float64
    minReturn float64
}

func (ps *PortfolioSpec) InitialState() gozdd.State {
    return gozdd.NewFloatState(0, 0) // total_risk, total_return
}

func (ps *PortfolioSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
    s := state.(*gozdd.FloatState)
    newState := s.Clone().(*gozdd.FloatState)
    
    if take {
        asset := ps.assets[level-1]
        newRisk := newState.Values[0] + asset.Risk
        
        if newRisk > ps.maxRisk {
            return nil, fmt.Errorf("risk limit exceeded")
        }
        
        newState.Values[0] = newRisk
        newState.Values[1] += asset.ExpectedReturn
    }
    
    return newState, nil
}

func (ps *PortfolioSpec) IsValid(state gozdd.State) bool {
    s := state.(*gozdd.FloatState)
    return s.Values[1] >= ps.minReturn
}
```

## Built-in Constraint Types

### CountConstraint
```go
// Select exactly 3 items
constraint := &gozdd.CountConstraint{Min: 3, Max: 3}

// Select 2-5 items
constraint := &gozdd.CountConstraint{Min: 2, Max: 5}
```

### SumConstraint
```go
// Weighted sum between 10 and 50
weights := []float64{0, 2.5, 1.0, 3.0, 1.5} // 1-based indexing
constraint := &gozdd.SumConstraint{
    Weights: weights,
    Min:     10.0,
    Max:     50.0,
}
```

### CustomConstraint
```go
// Custom business logic
constraint := &gozdd.CustomConstraint{
    Name: "No consecutive selection",
    ValidateFunc: func(ctx context.Context, state gozdd.State, level int, take bool) error {
        if take && level > 1 {
            // Check if previous item was also selected
            // (implementation depends on your state tracking)
        }
        return nil
    },
}
```

## Type-Safe Evaluation

### Counting Solutions
```go
count, err := zdd.Count(ctx)
fmt.Printf("Found %d solutions\n", count)
```

### Finding Optimal Solutions
```go
// Maximize value (use negative costs)
costs := []float64{0, -10, -20, -15, -30}
solutions, err := zdd.FindKBest(ctx, 1, costs)

if len(solutions) > 0 {
    optimal := solutions[0]
    fmt.Printf("Best solution: %v, value: %.0f\n", 
               optimal.Variables, -optimal.Cost)
}
```

### Finding Multiple Solutions
```go
// Get top 5 solutions
solutions, err := zdd.FindKBest(ctx, 5, costs)

for i, sol := range solutions {
    fmt.Printf("Solution %d: variables %v, cost %.2f\n", 
               i+1, sol.Variables, sol.Cost)
}
```

## Configuration Options

```go
zdd := gozdd.NewZDD(10,
    gozdd.WithParallel(4),                    // Use 4 goroutines
    gozdd.WithMemoryLimit(1<<30),             // 1GB memory limit
    gozdd.WithTimeout(time.Minute),           // 1 minute timeout
)
```

## Performance Tips

1. **Variable Ordering**: Order variables by constraint tightness (most constrained first)
2. **State Design**: Keep state minimal - only track what's necessary for constraints
3. **Early Pruning**: Implement `CanPrune()` in constraints for better performance
4. **Memory Management**: Use built-in state types to avoid allocation overhead

## Examples

- **[Knapsack Problem](examples/knapsack/)** - Complete example with validation against MILP solver
- **Resource Allocation** - Coming soon
- **Team Selection** - Coming soon

## Documentation

See [pkg.go.dev](https://pkg.go.dev/github.com/zzenonn/go-zdd) for full API documentation.

## License

MIT License
