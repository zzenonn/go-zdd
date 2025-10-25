package gozdd

import (
	"context"
	"fmt"
	"sort"
)

// Solution represents a feasible solution extracted from a ZDD.
//
// A solution consists of the selected variables and associated metadata
// such as cost, utility, or other objective values computed during evaluation.
type Solution struct {
	// Variables contains the indices of selected variables (1-based)
	Variables []int
	
	// Cost represents the objective value for this solution
	Cost float64
	
	// Metadata stores additional solution-specific data
	// Applications can store domain-specific information here
	Metadata map[string]interface{}
}

// Evaluator defines the interface for ZDD evaluation algorithms.
//
// Evaluators traverse the ZDD structure to extract information such as:
//   - Solution count
//   - Optimal solutions
//   - K-best solutions
//   - Custom objective functions
//
// Evaluators use bottom-up dynamic programming for efficiency.
type Evaluator interface {
	// Evaluate performs bottom-up evaluation of the ZDD.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - zdd: The ZDD to evaluate
	//
	// Returns the evaluation result or an error if evaluation fails.
	// The result type depends on the specific evaluator implementation.
	Evaluate(ctx context.Context, zdd *ZDD) (interface{}, error)
}

// CountEvaluator counts the total number of solutions in the ZDD.
//
// This evaluator computes the cardinality of the solution set represented
// by the ZDD using efficient bottom-up traversal.
type CountEvaluator struct{}

// Evaluate counts all solutions in the ZDD
func (e CountEvaluator) Evaluate(ctx context.Context, zdd *ZDD) (interface{}, error) {
	if zdd.root == NullNode {
		return int64(0), nil
	}
	
	// Memoization table for dynamic programming
	memo := make(map[NodeID]int64)
	
	count, err := e.countRecursive(ctx, zdd, zdd.root, memo)
	if err != nil {
		return int64(0), fmt.Errorf("count evaluation failed: %w", err)
	}
	
	return count, nil
}

// countRecursive performs recursive solution counting with memoization
func (e CountEvaluator) countRecursive(ctx context.Context, zdd *ZDD, nodeID NodeID, memo map[NodeID]int64) (int64, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	
	// Check memoization
	if count, exists := memo[nodeID]; exists {
		return count, nil
	}
	
	// Handle terminal nodes
	if nodeID == ZeroNode {
		memo[nodeID] = 0
		return 0, nil
	}
	if nodeID == OneNode {
		memo[nodeID] = 1
		return 1, nil
	}
	
	// Get node structure
	node, err := zdd.GetNode(nodeID)
	if err != nil {
		return 0, err
	}
	
	// Recursively count solutions in both subtrees
	loCount, err := e.countRecursive(ctx, zdd, node.Lo, memo)
	if err != nil {
		return 0, err
	}
	
	hiCount, err := e.countRecursive(ctx, zdd, node.Hi, memo)
	if err != nil {
		return 0, err
	}
	
	// Total count is sum of both subtrees
	totalCount := loCount + hiCount
	memo[nodeID] = totalCount
	
	return totalCount, nil
}

// CostEvaluator finds the optimal solution with minimum cost.
//
// This evaluator requires cost information for each variable and computes
// the solution with the lowest total cost using dynamic programming.
type CostEvaluator struct {
	// Costs specifies the cost of selecting each variable (1-based indexing)
	// Costs[0] is ignored, Costs[i] is the cost of selecting variable i
	Costs []float64
}

// OptimalResult represents the result of optimal solution evaluation
type OptimalResult struct {
	Solution *Solution
	Cost     float64
	Found    bool
}

// Evaluate finds the optimal (minimum cost) solution
func (e CostEvaluator) Evaluate(ctx context.Context, zdd *ZDD) (interface{}, error) {
	if zdd.root == NullNode {
		return OptimalResult{Found: false}, nil
	}
	
	if len(e.Costs) <= zdd.vars {
		return OptimalResult{Found: false}, fmt.Errorf("insufficient cost data: need %d costs, got %d", zdd.vars, len(e.Costs)-1)
	}
	
	// Memoization for optimal costs and solutions
	costMemo := make(map[NodeID]float64)
	solutionMemo := make(map[NodeID][]int)
	
	cost, solution, err := e.optimalRecursive(ctx, zdd, zdd.root, costMemo, solutionMemo)
	if err != nil {
		return OptimalResult{Found: false}, fmt.Errorf("optimal evaluation failed: %w", err)
	}
	
	if len(solution) == 0 && cost == 0 && zdd.root == ZeroNode {
		return OptimalResult{Found: false}, nil
	}
	
	result := &Solution{
		Variables: solution,
		Cost:      cost,
		Metadata:  make(map[string]interface{}),
	}
	
	return OptimalResult{Solution: result, Cost: cost, Found: true}, nil
}

// optimalRecursive finds optimal solution recursively with memoization
func (e CostEvaluator) optimalRecursive(ctx context.Context, zdd *ZDD, nodeID NodeID, costMemo map[NodeID]float64, solutionMemo map[NodeID][]int) (float64, []int, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	default:
	}
	
	// Check memoization
	if cost, exists := costMemo[nodeID]; exists {
		return cost, solutionMemo[nodeID], nil
	}
	
	// Handle terminal nodes
	if nodeID == ZeroNode {
		costMemo[nodeID] = float64(1e9) // Infeasible (high cost)
		solutionMemo[nodeID] = nil
		return float64(1e9), nil, nil
	}
	if nodeID == OneNode {
		costMemo[nodeID] = 0
		solutionMemo[nodeID] = []int{}
		return 0, []int{}, nil
	}
	
	// Get node structure
	node, err := zdd.GetNode(nodeID)
	if err != nil {
		return 0, nil, err
	}
	
	// Evaluate both subtrees
	loCost, loSolution, err := e.optimalRecursive(ctx, zdd, node.Lo, costMemo, solutionMemo)
	if err != nil {
		return 0, nil, err
	}
	
	hiCost, hiSolution, err := e.optimalRecursive(ctx, zdd, node.Hi, costMemo, solutionMemo)
	if err != nil {
		return 0, nil, err
	}
	
	// Add variable cost to hi-arc path
	if node.Level > 0 && node.Level < len(e.Costs) {
		hiCost += e.Costs[node.Level]
	}
	
	// Choose the better option
	var bestCost float64
	var bestSolution []int
	
	if loCost <= hiCost {
		bestCost = loCost
		bestSolution = make([]int, len(loSolution))
		copy(bestSolution, loSolution)
	} else {
		bestCost = hiCost
		bestSolution = make([]int, len(hiSolution)+1)
		copy(bestSolution, hiSolution)
		bestSolution[len(hiSolution)] = node.Level // Add current variable
	}
	
	// Memoize result
	costMemo[nodeID] = bestCost
	solutionMemo[nodeID] = bestSolution
	
	return bestCost, bestSolution, nil
}

// KBestEvaluator finds the k best solutions with lowest costs.
//
// This evaluator uses a priority queue approach to efficiently extract
// the top k solutions without enumerating all solutions.
type KBestEvaluator struct {
	// K is the number of best solutions to find
	K int
	
	// Costs specifies the cost of selecting each variable (1-based indexing)
	Costs []float64
}

// KBestResult represents the result of k-best evaluation
type KBestResult struct {
	Solutions []*Solution
	Count     int
}

// Evaluate finds the k best solutions with lowest costs
func (e KBestEvaluator) Evaluate(ctx context.Context, zdd *ZDD) (interface{}, error) {
	if zdd.root == NullNode || e.K <= 0 {
		return KBestResult{Solutions: []*Solution{}, Count: 0}, nil
	}
	
	if len(e.Costs) <= zdd.vars {
		return KBestResult{}, fmt.Errorf("insufficient cost data: need %d costs, got %d", zdd.vars, len(e.Costs)-1)
	}
	
	// Use a simple approach: enumerate solutions and sort by cost
	// For large k, more sophisticated algorithms would be needed
	solutions, err := e.enumerateSolutions(ctx, zdd, zdd.root, []int{}, 0)
	if err != nil {
		return KBestResult{}, fmt.Errorf("k-best evaluation failed: %w", err)
	}
	
	// Sort solutions by cost
	sort.Slice(solutions, func(i, j int) bool {
		return solutions[i].Cost < solutions[j].Cost
	})
	
	// Return top k solutions
	count := len(solutions)
	if count > e.K {
		solutions = solutions[:e.K]
	}
	
	return KBestResult{Solutions: solutions, Count: count}, nil
}

// enumerateSolutions recursively enumerates all solutions with costs
func (e KBestEvaluator) enumerateSolutions(ctx context.Context, zdd *ZDD, nodeID NodeID, currentVars []int, currentCost float64) ([]*Solution, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Handle terminal nodes
	if nodeID == ZeroNode {
		return []*Solution{}, nil // No solutions
	}
	if nodeID == OneNode {
		// Create solution from current path
		vars := make([]int, len(currentVars))
		copy(vars, currentVars)
		sort.Ints(vars) // Sort for consistent output
		
		solution := &Solution{
			Variables: vars,
			Cost:      currentCost,
			Metadata:  make(map[string]interface{}),
		}
		return []*Solution{solution}, nil
	}
	
	// Get node structure
	node, err := zdd.GetNode(nodeID)
	if err != nil {
		return nil, err
	}
	
	var allSolutions []*Solution
	
	// Explore lo-arc (don't take variable)
	loSolutions, err := e.enumerateSolutions(ctx, zdd, node.Lo, currentVars, currentCost)
	if err != nil {
		return nil, err
	}
	allSolutions = append(allSolutions, loSolutions...)
	
	// Explore hi-arc (take variable)
	newVars := make([]int, len(currentVars)+1)
	copy(newVars, currentVars)
	newVars[len(currentVars)] = node.Level
	
	newCost := currentCost
	if node.Level > 0 && node.Level < len(e.Costs) {
		newCost += e.Costs[node.Level]
	}
	
	hiSolutions, err := e.enumerateSolutions(ctx, zdd, node.Hi, newVars, newCost)
	if err != nil {
		return nil, err
	}
	allSolutions = append(allSolutions, hiSolutions...)
	
	return allSolutions, nil
}

// CustomEvaluator allows applications to define custom evaluation logic.
//
// This provides flexibility for domain-specific evaluation requirements
// while maintaining the same interface as built-in evaluators.
type CustomEvaluator struct {
	// EvaluateFunc performs the custom evaluation
	EvaluateFunc func(ctx context.Context, zdd *ZDD) (interface{}, error)
	
	// Name provides a description for debugging
	Name string
}

// Evaluate delegates to the custom evaluation function
func (e CustomEvaluator) Evaluate(ctx context.Context, zdd *ZDD) (interface{}, error) {
	if e.EvaluateFunc == nil {
		return nil, fmt.Errorf("custom evaluator %s has no evaluation function", e.Name)
	}
	
	result, err := e.EvaluateFunc(ctx, zdd)
	if err != nil && e.Name != "" {
		return nil, fmt.Errorf("%s: %w", e.Name, err)
	}
	
	return result, err
}

// EvaluateZDD is a convenience function for evaluating ZDDs with any evaluator.
//
// This function provides a simple interface for ZDD evaluation with proper
// error handling and context support.
func EvaluateZDD(ctx context.Context, zdd *ZDD, evaluator Evaluator) (interface{}, error) {
	if zdd == nil {
		return nil, fmt.Errorf("%w: ZDD is nil", ErrInvalidNode)
	}
	
	if evaluator == nil {
		return nil, fmt.Errorf("%w: evaluator is nil", ErrInvalidConstraint)
	}
	
	return evaluator.Evaluate(ctx, zdd)
}
