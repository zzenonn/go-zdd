// Package main demonstrates SkipState optimization for server-task assignment.
//
// This example shows how SkipState can dramatically reduce the effective
// problem size by skipping irrelevant variables when logical dependencies
// make certain branches impossible.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zzenonn/go-zdd"
)

// Server represents a compute server
type Server struct {
	ID       int
	CPU      int
	Memory   int
	Location string
}

// Task represents a computational task
type Task struct {
	ID     int
	CPU    int
	Memory int
	Server int // Which server this task can be assigned to
}

// ServerTaskSpec implements server-task assignment with SkipState optimization
type ServerTaskSpec struct {
	servers []Server
	tasks   []Task
	
	// Variable mapping: first N variables are server selection,
	// remaining variables are task assignments
	serverVars int
	
	// Skip tracking
	SkipCount int
	SkippedVariables int
}

func NewServerTaskSpec(servers []Server, tasks []Task) *ServerTaskSpec {
	return &ServerTaskSpec{
		servers:    servers,
		tasks:      tasks,
		serverVars: len(servers),
	}
}

func (sts *ServerTaskSpec) Variables() int {
	return len(sts.servers) + len(sts.tasks)
}

func (sts *ServerTaskSpec) InitialState() gozdd.State {
	// Track: [selected_servers..., total_cpu, total_memory]
	initialValues := make([]int, len(sts.servers)+2)
	return gozdd.NewIntState(initialValues...)
}

func (sts *ServerTaskSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
	s := state.(*gozdd.IntState)
	newState := s.Clone().(*gozdd.IntState)
	
	// CRITICAL: ZDD processes variables from HIGH to LOW level
	// Variable mapping (reversed for SkipState to work):
	// High levels (len(servers)+len(tasks) down to len(tasks)+1): Server selection
	// Low levels (len(tasks) down to 1): Task assignment
	
	if level > len(sts.tasks) {
		// Server selection variable (high levels processed first)
		serverIndex := level - len(sts.tasks) - 1
		if serverIndex < 0 || serverIndex >= len(sts.servers) {
			return nil, fmt.Errorf("invalid server index")
		}
		
		if take {
			// Mark server as selected
			newState.Values[serverIndex] = 1
		} else {
			// Don't skip at server level - SkipState will be applied at task level
			// when we discover the server is not selected
		}
	} else {
		// Task assignment variable (low levels processed later)
		taskIndex := level - 1
		if taskIndex < 0 || taskIndex >= len(sts.tasks) {
			return nil, fmt.Errorf("invalid task index")
		}
		
		task := sts.tasks[taskIndex]
		
		if take {
			// Check if the required server is selected
			if newState.Values[task.Server] == 0 {
				return nil, fmt.Errorf("task requires unselected server")
			}
			
			// Add task resource usage
			newState.Values[len(sts.servers)] += task.CPU    // total_cpu
			newState.Values[len(sts.servers)+1] += task.Memory // total_memory
		} else {
			// SKIPSTATE OPTIMIZATION: If task not taken and its server is not selected,
			// we can skip remaining tasks for the same server
			if newState.Values[task.Server] == 0 {
				// Find next task that doesn't depend on this server
				nextLevel := 0
				for i := taskIndex - 1; i >= 0; i-- {
					if sts.tasks[i].Server != task.Server {
						nextLevel = i + 1
						break
					}
				}
				
				if nextLevel > 0 && nextLevel < level {
					skippedCount := level - nextLevel
					sts.SkipCount++
					sts.SkippedVariables += skippedCount
					return gozdd.NewSkipState(newState, nextLevel), nil
				}
			}
		}
	}
	
	return newState, nil
}

func (sts *ServerTaskSpec) IsValid(state gozdd.State) bool {
	s := state.(*gozdd.IntState)
	
	// Check server capacity constraints
	for i, server := range sts.servers {
		if s.Values[i] == 1 { // Server is selected
			serverCPU := 0
			serverMemory := 0
			
			// Calculate total load on this server
			for _, task := range sts.tasks {
				if task.Server == i {
					// This is a simplified check - in real implementation,
					// you'd track which tasks are actually assigned
					serverCPU += task.CPU
					serverMemory += task.Memory
				}
			}
			
			if serverCPU > server.CPU || serverMemory > server.Memory {
				return false
			}
		}
	}
	
	return true
}

// NoSkipServerTaskSpec is the same problem without SkipState optimization
type NoSkipServerTaskSpec struct {
	*ServerTaskSpec
}

func (nsts *NoSkipServerTaskSpec) GetChild(ctx context.Context, state gozdd.State, level int, take bool) (gozdd.State, error) {
	s := state.(*gozdd.IntState)
	newState := s.Clone().(*gozdd.IntState)
	
	// Same variable mapping as SkipState version but without skipping
	if level > len(nsts.tasks) {
		// Server selection variable (high levels)
		serverIndex := level - len(nsts.tasks) - 1
		if serverIndex < 0 || serverIndex >= len(nsts.servers) {
			return nil, fmt.Errorf("invalid server index")
		}
		
		if take {
			newState.Values[serverIndex] = 1
		}
		// NO SKIPSTATE - process all variables normally
	} else {
		// Task assignment variable (low levels)
		taskIndex := level - 1
		if taskIndex < 0 || taskIndex >= len(nsts.tasks) {
			return nil, fmt.Errorf("invalid task index")
		}
		
		task := nsts.tasks[taskIndex]
		
		if take {
			// Check if the required server is selected
			if newState.Values[task.Server] == 0 {
				return nil, fmt.Errorf("task requires unselected server")
			}
			
			// Add task resource usage
			newState.Values[len(nsts.servers)] += task.CPU
			newState.Values[len(nsts.servers)+1] += task.Memory
		}
	}
	
	return newState, nil
}

func createTestProblem() ([]Server, []Task) {
	// Create 5 servers
	servers := []Server{
		{ID: 0, CPU: 100, Memory: 200, Location: "US-East"},
		{ID: 1, CPU: 150, Memory: 300, Location: "US-West"},
		{ID: 2, CPU: 120, Memory: 250, Location: "EU-West"},
		{ID: 3, CPU: 80, Memory: 150, Location: "Asia"},
		{ID: 4, CPU: 200, Memory: 400, Location: "US-Central"},
	}
	
	// Create 20 tasks, each requiring a specific server
	tasks := make([]Task, 20)
	for i := 0; i < 20; i++ {
		tasks[i] = Task{
			ID:     i,
			CPU:    10 + (i%5)*5,    // CPU: 10, 15, 20, 25, 30
			Memory: 20 + (i%3)*10,   // Memory: 20, 30, 40
			Server: i % len(servers), // Distribute across servers
		}
	}
	
	return servers, tasks
}

type BenchmarkResult struct {
	BuildTime time.Duration
	Nodes     int
	Solutions int
	SkipCount int
	SkippedVars int
}

func runBenchmark(name string, spec gozdd.ConstraintSpec) (BenchmarkResult, error) {
	fmt.Printf("\n🔧 Testing %s\n", name)
	fmt.Printf("Variables: %d\n", spec.Variables())
	
	zdd := gozdd.NewZDD(spec.Variables())
	
	start := time.Now()
	ctx := context.Background()
	
	if err := zdd.Build(ctx, spec); err != nil {
		return BenchmarkResult{}, fmt.Errorf("build failed: %v", err)
	}
	
	buildTime := time.Since(start)
	
	count, err := zdd.Count(ctx)
	if err != nil {
		return BenchmarkResult{}, fmt.Errorf("count failed: %v", err)
	}
	
	result := BenchmarkResult{
		BuildTime: buildTime,
		Nodes:     zdd.Size(),
		Solutions: int(count),
	}
	
	// Extract skip statistics if available
	if skipSpec, ok := spec.(*ServerTaskSpec); ok {
		result.SkipCount = skipSpec.SkipCount
		result.SkippedVars = skipSpec.SkippedVariables
	}
	
	fmt.Printf("Build time: %v\n", buildTime)
	fmt.Printf("ZDD nodes: %d\n", zdd.Size())
	fmt.Printf("Solutions: %d\n", count)
	if result.SkipCount > 0 {
		fmt.Printf("Skip operations: %d\n", result.SkipCount)
		fmt.Printf("Variables skipped: %d\n", result.SkippedVars)
	}
	
	return result, nil
}

