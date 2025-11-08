# SkipState Optimization Example

This example demonstrates how SkipState can dramatically improve ZDD performance by skipping irrelevant variables when logical dependencies make certain branches impossible.

## Problem: Server-Task Assignment

- **Servers**: 5 compute servers with different capacities
- **Tasks**: 20 computational tasks, each requiring a specific server
- **Constraint**: Tasks can only run on their designated server
- **Optimization**: When a server is not selected, skip all tasks that require that server

## Variable Ordering (Critical for SkipState)

ZDD processes variables from **high level to low level** (25 → 1), so:

- **High levels (21-25)**: Server selection variables
- **Low levels (1-20)**: Task assignment variables

This ordering ensures server decisions are made before task decisions, enabling SkipState to skip task variables when their required server is not selected.

## Key Results

The example shows:

1. **Skip Operations**: Thousands of skip operations performed
2. **Performance Improvement**: Faster build times and fewer ZDD nodes
3. **Correctness**: Identical solution counts between SkipState and non-SkipState versions
4. **Effective Variable Reduction**: Dramatic reduction in variables that need processing

## Running the Example

```bash
cd examples/skipstate
go run main.go
```

## Key Insight

SkipState optimization is most effective when:
1. Variables have logical dependencies (server → tasks)
2. Dependency variables are processed before dependent variables
3. Many branches can be pruned early based on dependency decisions

This pattern is common in:
- Resource allocation problems
- Hierarchical selection problems  
- Problems with prerequisite relationships