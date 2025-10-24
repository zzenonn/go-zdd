#!/bin/bash

# Go ZDD Project Setup Script
# Creates the initial directory structure and placeholder files

set -e

echo "Setting up Go ZDD project structure..."

# Create go.mod
cat > go.mod << 'EOF'
module github.com/zenon/go-zdd

go 1.21

require ()
EOF

# Create main package files
touch doc.go zdd.go node.go constraint.go solution.go options.go errors.go datacenter.go

# Create test files
touch zdd_test.go constraint_test.go solution_test.go datacenter_test.go example_test.go bench_test.go

# Create internal directory
mkdir -p internal
touch internal/pool.go

# Create ignore directory for development files
mkdir -p ignore

# Create basic README
cat > README.md << 'EOF'
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
EOF

# Create .gitignore
cat > .gitignore << 'EOF'
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with `go test -c`
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Planning and development files
TODO.md
*PLAN*.md
*CHECKLIST*.md
ignore/
EOF

echo "✅ Project structure created successfully!"
echo ""
echo "Next steps:"
echo "1. cd $(pwd)"
echo "2. git init"
echo "3. Start implementing according to GO_ZDD_IMPLEMENTATION_PLAN.md"
echo "4. Run 'go mod tidy' after adding dependencies"
