package gozdd

import (
	"runtime"
	"time"
)

// Config holds ZDD construction configuration parameters.
// All fields are exported to allow inspection after construction.
type Config struct {
	// Workers specifies the number of goroutines to use for parallel construction.
	// A value of 1 disables parallelism.
	Workers int
	
	// MemoryLimit sets the maximum memory usage in bytes.
	// A value of 0 means no limit is enforced.
	MemoryLimit int64
	
	// Timeout specifies the maximum duration for ZDD construction.
	// A value of 0 means no timeout is enforced.
	Timeout time.Duration
}

// Option configures ZDD construction parameters using the functional options pattern.
// Options are applied in the order they are provided to NewZDD.
type Option func(*Config)

// WithParallel sets the number of worker goroutines for parallel construction.
// 
// If workers <= 0, defaults to runtime.NumCPU() for optimal CPU utilization.
// If workers == 1, construction runs sequentially without goroutine overhead.
// If workers > 1, construction uses parallel algorithms where applicable.
//
// Note: Not all construction phases can be parallelized. The actual speedup
// depends on the problem structure and constraint complexity.
func WithParallel(workers int) Option {
	return func(c *Config) {
		if workers <= 0 {
			c.Workers = runtime.NumCPU()
		} else {
			c.Workers = workers
		}
	}
}

// WithMemoryLimit sets the memory limit in bytes for ZDD construction.
//
// If bytes <= 0, no memory limit is enforced (unlimited memory usage).
// If bytes > 0, construction will fail with ErrMemoryLimit when exceeded.
//
// The memory limit applies to the node table and internal data structures.
// It does not include memory used by application-defined State objects.
func WithMemoryLimit(bytes int64) Option {
	return func(c *Config) {
		c.MemoryLimit = bytes
	}
}

// WithTimeout sets the maximum duration for ZDD construction operations.
//
// If duration <= 0, no timeout is enforced (operations may run indefinitely).
// If duration > 0, construction will fail with context.DeadlineExceeded when exceeded.
//
// The timeout applies to the entire Build operation. Individual constraint
// evaluations may still block if they don't respect context cancellation.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// newConfig creates a new configuration with sensible defaults and applies
// the provided options in order.
//
// Default values:
//   - Workers: 1 (sequential construction)
//   - MemoryLimit: 1GB (1 << 30 bytes)
//   - Timeout: 0 (no timeout)
func newConfig(opts ...Option) *Config {
	cfg := &Config{
		Workers:     1,
		MemoryLimit: 1 << 30, // 1GB default
		Timeout:     0,       // No timeout by default
	}
	
	for _, opt := range opts {
		opt(cfg)
	}
	
	return cfg
}
