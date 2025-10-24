# Go ZDD

A Go-native Zero-suppressed Decision Diagram (ZDD) library for constraint-based optimization problems.

## Features

- Unified ZDD construction and evaluation
- Context-aware operations with cancellation support
- Concurrent processing with configurable parallelism
- Functional options for configuration
- Interface-based constraint framework
- Built-in multi-datacenter optimization domain

## Installation

```bash
go get github.com/zenon/go-zdd
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/zenon/go-zdd"
)

func main() {
    // Create ZDD with 5 variables
    zdd := gozdd.NewZDD(5, gozdd.WithParallel(4))
    
    // Define constraints
    spec := &gozdd.BasicSpec{
        Variables: 5,
        Constraints: []gozdd.Constraint{
            &gozdd.CountConstraint{Min: 2, Max: 3},
        },
    }
    
    // Build ZDD
    ctx := context.Background()
    if err := zdd.Build(ctx, spec); err != nil {
        log.Fatal(err)
    }
    
    // Count solutions
    count, err := zdd.Evaluate(ctx, &gozdd.CountEvaluator{})
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Total solutions: %d\n", count)
}
```

## Documentation

See [pkg.go.dev](https://pkg.go.dev/github.com/zenon/go-zdd) for full API documentation.

## License

MIT License
