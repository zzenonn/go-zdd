# Knapsack Problem with ZDD - Beginner's Guide

This example shows how to solve the classic **knapsack problem** using Zero-suppressed Decision Diagrams (ZDDs). Don't worry if you've never heard of ZDDs - this guide will walk you through everything!

## What is the Knapsack Problem?

Imagine you're packing for a trip and have a backpack with limited space. You have many items to choose from, each with:
- A **value** (how useful it is)
- A **weight** (how much space it takes)

**Goal**: Pick items that fit in your backpack and give you the maximum total value.

**Example**: Backpack capacity = 10kg
- Gaming Mouse: value=40, weight=1kg
- Gaming Monitor: value=100, weight=15kg (too heavy!)
- SSD Drive: value=90, weight=1kg (good choice)

## What is a ZDD?

A **Zero-suppressed Decision Diagram** is a smart data structure that:
1. **Represents ALL possible solutions** to your problem
2. **Eliminates impossible choices** automatically (like items too heavy for your backpack)
3. **Finds the best solution** without checking every possibility manually

Think of it as a decision tree that automatically prunes bad branches.

## How to Use the ZDD Library

### Step 1: Define Your Problem

```go
// Your items
items := []Item{
    {Name: "Gaming Mouse", Value: 40, Weight: 1},
    {Name: "SSD Drive", Value: 90, Weight: 1},
    {Name: "Gaming Monitor", Value: 100, Weight: 15},
}
capacity := 10.0 // Your backpack capacity
```

### Step 2: Create a Constraint Specification

This tells the ZDD what rules to follow:

```go
// Create the knapsack constraint
spec := NewKnapsackSpec(items, capacity)
```

The `KnapsackSpec` automatically handles:
- Rejecting items that exceed capacity
- Tracking total weight and value
- State management (you don't need to worry about this!)

### Step 3: Build the ZDD

```go
// Create ZDD with 3 variables (one per item)
zdd := gozdd.NewZDD(len(items))

// Build the ZDD (finds all valid combinations)
ctx := context.Background()
err := zdd.Build(ctx, spec)
```

The ZDD now contains **every possible valid combination** of items that fit in your backpack.

### Step 4: Find Solutions

```go
// Count how many valid combinations exist
count, err := zdd.Count(ctx)
fmt.Printf("Found %d valid combinations\n", count)

// Find the best combination
costs := []float64{0, -40, -90, -100} // Negative because we want to maximize
solutions, err := zdd.FindKBest(ctx, 1, costs)

if len(solutions) > 0 {
    best := solutions[0]
    fmt.Printf("Best solution: items %v, value: %.0f\n", 
               best.Variables, -best.Cost)
}
```

## Key Concepts Explained

### Variables
Each item is a **variable** that can be:
- `0` = don't take the item
- `1` = take the item

For 3 items, you have variables: `[item1, item2, item3]`

### State
The **state** tracks information as you make decisions:
- Current total weight
- Current total value

We use `gozdd.FloatState` to handle this automatically.

### Constraints
**Constraints** are rules that eliminate invalid choices:
- "Total weight must not exceed capacity"
- "Must select at least 2 items"
- "Cannot select both item A and item B"

### Solutions
A **solution** is a valid combination of items:
- `Variables: [1, 3]` means "take item 1 and item 3"
- `Cost: -130` means "total value is 130" (negative because we maximized)

## Running the Example

```bash
cd examples/knapsack
go run main.go
```

This runs the full validation against 3 different scenarios and shows that ZDD produces identical results to professional optimization solvers!

## Why Use ZDD Instead of Brute Force?

**Brute Force**: Check every possible combination
- 10 items = 1,024 combinations to check
- 20 items = 1,048,576 combinations to check
- 30 items = 1,073,741,824 combinations to check (very slow!)

**ZDD**: Smart elimination of impossible branches
- 10 items = ~50 nodes in the ZDD
- 20 items = ~200 nodes in the ZDD  
- 30 items = ~800 nodes in the ZDD (much faster!)

ZDDs scale **exponentially better** than brute force!

## Next Steps

1. **Modify the example**: Change item values/weights and see how solutions change
2. **Add constraints**: Try adding "must select at least 3 items" constraint
3. **Different problems**: Use ZDD for scheduling, routing, or resource allocation
4. **Learn more**: Read the main library documentation for advanced features

## Common Patterns

### Multiple Solutions
```go
// Get top 5 solutions instead of just the best
solutions, err := zdd.FindKBest(ctx, 5, costs)
```

### Different Objectives
```go
// Minimize weight instead of maximizing value
weights := []float64{0, 1, 1, 15} // Positive for minimization
solutions, err := zdd.FindKBest(ctx, 1, weights)
```

### Adding Constraints
```go
// Add minimum selection constraint
minItems := gozdd.CountConstraint{Min: 2, Max: len(items)}
// (This requires using the constraint framework - see constraint.go)
```
