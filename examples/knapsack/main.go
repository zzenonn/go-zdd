// Package main demonstrates ZDD usage for the knapsack problem.
//
// This example validates the ZDD library against MILP solver results
// for the classic 0-1 knapsack optimization problem.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/zzenonn/go-zdd"
)

// Item represents a knapsack item
type Item struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Weight float64 `json:"weight"`
}

// KnapsackData represents the input data structure
type KnapsackData map[string]struct {
	Capacity float64 `json:"capacity"`
	Items    []Item  `json:"items"`
}

// ExpectedResult represents the MILP solver output
type ExpectedResult struct {
	Scenario            string  `json:"scenario"`
	Capacity            float64 `json:"capacity"`
	TotalItems          int     `json:"total_items"`
	Success             bool    `json:"success"`
	OptimalValue        float64 `json:"optimal_value"`
	OptimalWeight       float64 `json:"optimal_weight"`
	CapacityUtilization float64 `json:"capacity_utilization"`
	SelectedItems       []Item  `json:"selected_items"`
}

// KnapsackSpec implements gozdd.ConstraintSpec using helper functions
type KnapsackSpec struct {
	items    []Item
	capacity float64
}

func NewKnapsackSpec(items []Item, capacity float64) *KnapsackSpec {
	return &KnapsackSpec{items: items, capacity: capacity}
}

func (ks *KnapsackSpec) Variables() int {
	return len(ks.items)
}

func (ks *KnapsackSpec) InitialState() gozdd.State {
	return gozdd.NewFloatState(0, 0) // weight, value
}

func (ks *KnapsackSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
	s := state.(*gozdd.FloatState)
	newState := s.Clone().(*gozdd.FloatState)
	
	if take {
		itemIndex := level - 1
		if itemIndex < 0 || itemIndex >= len(ks.items) {
			return nil, fmt.Errorf("invalid item index %d", itemIndex)
		}
		
		item := ks.items[itemIndex]
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
	s := state.(*gozdd.FloatState)
	return s.Values[0] <= ks.capacity
}

// cleanItemName removes emoji characters from item names
func cleanItemName(name string) string {
	// Remove emoji characters (simplified approach)
	re := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]`)
	cleaned := re.ReplaceAllString(name, "")
	return strings.TrimSpace(cleaned)
}

func main() {
	// Load knapsack data
	itemsFile := "../../testdata/knapsack/items.json"
	itemsData, err := os.ReadFile(itemsFile)
	if err != nil {
		log.Fatalf("Failed to read items file: %v", err)
	}
	
	var knapsackData KnapsackData
	if err := json.Unmarshal(itemsData, &knapsackData); err != nil {
		log.Fatalf("Failed to parse items JSON: %v", err)
	}
	
	// Load expected results
	resultsFile := "../../testdata/knapsack/expected_results.json"
	resultsData, err := os.ReadFile(resultsFile)
	if err != nil {
		log.Fatalf("Failed to read results file: %v", err)
	}
	
	var expectedResults []ExpectedResult
	if err := json.Unmarshal(resultsData, &expectedResults); err != nil {
		log.Fatalf("Failed to parse results JSON: %v", err)
	}
	
	fmt.Printf("🎒 Knapsack Problem - ZDD vs MILP Validation\n")
	fmt.Printf("============================================\n\n")
	
	allPassed := true
	
	// Test each scenario
	for scenario, data := range knapsackData {
		// Find expected result for this scenario
		var expected ExpectedResult
		for _, result := range expectedResults {
			if result.Scenario == scenario {
				expected = result
				break
			}
		}
		
		if expected.Scenario == "" {
			fmt.Printf("❌ %s: No expected results found\n", scenario)
			allPassed = false
			continue
		}
		
		// Clean emoji characters from item names
		items := data.Items
		for i := range items {
			items[i].Name = cleanItemName(items[i].Name)
		}
		
		// Clean expected item names
		for i := range expected.SelectedItems {
			expected.SelectedItems[i].Name = cleanItemName(expected.SelectedItems[i].Name)
		}
		
		fmt.Printf("📦 Testing %s\n", scenario)
		fmt.Printf("Items: %d, Capacity: %.0f\n", len(items), data.Capacity)
		fmt.Printf("Expected optimal value: %.0f, weight: %.0f\n", expected.OptimalValue, expected.OptimalWeight)
		
		// Create ZDD specification
		spec := NewKnapsackSpec(items, data.Capacity)
		
		// Create ZDD with parallel construction
		zdd := gozdd.NewZDD(len(items), gozdd.WithParallel(4))
		
		// Build ZDD
		ctx := context.Background()
		if err := zdd.Build(ctx, spec); err != nil {
			fmt.Printf("❌ %s: ZDD construction failed: %v\n\n", scenario, err)
			allPassed = false
			continue
		}
		
		// Count total solutions
		totalSolutions, err := zdd.Count(ctx)
		if err != nil {
			fmt.Printf("❌ %s: Solution counting failed: %v\n\n", scenario, err)
			allPassed = false
			continue
		}
		
		// Find top 10 solutions
		costs := make([]float64, len(items)+1) // 1-based indexing
		for i, item := range items {
			costs[i+1] = -item.Value // Negative because we want to maximize value
		}
		
		solutions, err := zdd.FindKBest(ctx, 10, costs)
		if err != nil {
			fmt.Printf("❌ %s: Top 10 solution search failed: %v\n\n", scenario, err)
			allPassed = false
			continue
		}
		
		if len(solutions) == 0 {
			fmt.Printf("❌ %s: No solutions found\n\n", scenario)
			allPassed = false
			continue
		}
		
		optimal := solutions[0] // Best solution for validation
		
		// Calculate actual value and weight
		actualValue := 0.0
		actualWeight := 0.0
		selectedItems := make([]Item, 0)
		
		for _, varLevel := range optimal.Variables {
			itemIndex := varLevel - 1 // Convert back to 0-based index
			if itemIndex >= 0 && itemIndex < len(items) {
				item := items[itemIndex]
				actualValue += item.Value
				actualWeight += item.Weight
				selectedItems = append(selectedItems, item)
			}
		}
		
		// Sort selected items by name for comparison
		sort.Slice(selectedItems, func(i, j int) bool {
			return selectedItems[i].Name < selectedItems[j].Name
		})
		
		sort.Slice(expected.SelectedItems, func(i, j int) bool {
			return expected.SelectedItems[i].Name < expected.SelectedItems[j].Name
		})
		
		// Check if selected items match
		itemsMatch := len(selectedItems) == len(expected.SelectedItems)
		if itemsMatch {
			for i, item := range selectedItems {
				if i >= len(expected.SelectedItems) || 
				   item.Name != expected.SelectedItems[i].Name ||
				   item.Value != expected.SelectedItems[i].Value ||
				   item.Weight != expected.SelectedItems[i].Weight {
					itemsMatch = false
					break
				}
			}
		}
		
		// Validation results
		valueMatch := actualValue == expected.OptimalValue
		weightMatch := actualWeight == expected.OptimalWeight
		scenarioPassed := valueMatch && weightMatch && itemsMatch
		
		if scenarioPassed {
			fmt.Printf("✅ %s: PASSED (nodes: %d, solutions: %d)\n", scenario, zdd.Size(), totalSolutions)
			
			// Show top 10 solutions
			fmt.Printf("   Top %d solutions:\n", len(solutions))
			for i, sol := range solutions {
				// Calculate actual value and weight for this solution
				solValue := 0.0
				solWeight := 0.0
				solItems := make([]string, 0)
				
				for _, varLevel := range sol.Variables {
					itemIndex := varLevel - 1
					if itemIndex >= 0 && itemIndex < len(items) {
						item := items[itemIndex]
						solValue += item.Value
						solWeight += item.Weight
						solItems = append(solItems, item.Name)
					}
				}
				
				fmt.Printf("   %2d. Value: %4.0f, Weight: %4.0f, Items: %s\n", 
					i+1, solValue, solWeight, strings.Join(solItems, ", "))
			}
		} else {
			fmt.Printf("❌ %s: FAILED\n", scenario)
			fmt.Printf("   Value: %.0f vs %.0f (match: %t)\n", actualValue, expected.OptimalValue, valueMatch)
			fmt.Printf("   Weight: %.0f vs %.0f (match: %t)\n", actualWeight, expected.OptimalWeight, weightMatch)
			fmt.Printf("   Items: %d vs %d (match: %t)\n", len(selectedItems), len(expected.SelectedItems), itemsMatch)
			allPassed = false
		}
		
		fmt.Println()
	}
	
	if allPassed {
		fmt.Printf("🎉 ALL SCENARIOS PASSED: ZDD produces identical results to MILP solver!\n")
	} else {
		fmt.Printf("💥 SOME SCENARIOS FAILED: Results differ from MILP solver\n")
		os.Exit(1)
	}
}
