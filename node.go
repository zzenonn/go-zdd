package gozdd

import (
	"fmt"
	"sync"
)

// NodeID represents a unique identifier for ZDD nodes.
// NodeIDs are assigned sequentially during construction and remain
// valid for the lifetime of the NodeTable.
type NodeID uint32

// Special node IDs for ZDD terminals and invalid references.
const (
	// NullNode represents an invalid or uninitialized node reference.
	NullNode NodeID = 0
	
	// ZeroNode represents the 0-terminal (empty set, false).
	// All paths leading to ZeroNode represent infeasible solutions.
	ZeroNode NodeID = 1
	
	// OneNode represents the 1-terminal (base set, true).
	// All paths leading to OneNode represent feasible solutions.
	OneNode NodeID = 2
)

// Node represents a ZDD node with a variable level and two outgoing arcs.
//
// ZDD nodes follow these invariants:
//   - Level > 0 for non-terminal nodes, Level == 0 for terminals
//   - Lo arc represents the case where the variable is NOT selected
//   - Hi arc represents the case where the variable IS selected
//   - Hi arc never points to ZeroNode (ZDD reduction rule)
type Node struct {
	// Level indicates the variable level (1-based indexing).
	// Level 0 is reserved for terminal nodes.
	Level int
	
	// Lo is the 0-arc, representing the "variable not selected" branch.
	// Points to a node at a higher level or a terminal.
	Lo NodeID
	
	// Hi is the 1-arc, representing the "variable selected" branch.
	// Points to a node at a higher level or a terminal.
	// Never points to ZeroNode due to ZDD reduction rules.
	Hi NodeID
}

// IsTerminal returns true if this node is a terminal (0-terminal or 1-terminal).
// Terminal nodes have Level == 0 and represent the final decision outcomes.
func (n Node) IsTerminal() bool {
	return n.Level == 0
}

// NodeTable manages ZDD nodes with automatic deduplication and reduction.
//
// The NodeTable ensures that:
//   - Identical nodes are shared (structural sharing)
//   - ZDD reduction rules are applied automatically
//   - Thread-safe concurrent access to nodes
//   - Efficient lookup and insertion operations
//
// Memory usage grows with the number of unique node specifications.
// Nodes are never deleted once created to maintain NodeID validity.
type NodeTable struct {
	// mu protects concurrent access to nodes and hash map
	mu sync.RWMutex
	
	// nodes stores the actual node data indexed by NodeID
	nodes []Node
	
	// hash provides O(1) lookup for node deduplication
	// Maps node specification to existing NodeID
	hash map[Node]NodeID
	
	// next tracks the next available NodeID for assignment
	next NodeID
}

// NewNodeTable creates a new node table with pre-initialized terminal nodes.
//
// The table is initialized with:
//   - NullNode (ID 0): Invalid/uninitialized reference
//   - ZeroNode (ID 1): 0-terminal representing empty set
//   - OneNode (ID 2): 1-terminal representing base set
//
// Returns a thread-safe NodeTable ready for ZDD construction.
func NewNodeTable() *NodeTable {
	nt := &NodeTable{
		nodes: make([]Node, 3), // Reserve space for null, 0, 1 terminals
		hash:  make(map[Node]NodeID),
		next:  3, // Start assigning IDs from 3
	}
	
	// Initialize terminal nodes with Level 0
	// Terminal nodes have null arcs since they don't branch further
	nt.nodes[ZeroNode] = Node{Level: 0, Lo: NullNode, Hi: NullNode}
	nt.nodes[OneNode] = Node{Level: 0, Lo: NullNode, Hi: NullNode}
	
	return nt
}

// GetNode retrieves a node by its ID with bounds checking.
//
// Returns ErrInvalidNode if:
//   - id == NullNode (invalid reference)
//   - id >= number of allocated nodes (out of bounds)
//
// This method is thread-safe for concurrent access.
func (nt *NodeTable) GetNode(id NodeID) (Node, error) {
	nt.mu.RLock()
	defer nt.mu.RUnlock()
	
	if id == NullNode || int(id) >= len(nt.nodes) {
		return Node{}, fmt.Errorf("%w: node ID %d", ErrInvalidNode, id)
	}
	
	return nt.nodes[id], nil
}

// AddNode creates a new node or returns an existing equivalent node.
//
// This method implements ZDD reduction rules:
//   1. If hi == ZeroNode, return lo (eliminate redundant nodes)
//   2. If an identical node exists, return its ID (structural sharing)
//   3. Otherwise, create a new node with a fresh ID
//
// Parameters:
//   - level: Variable level (must be > 0 for non-terminals)
//   - lo: NodeID for the 0-arc (variable not selected)
//   - hi: NodeID for the 1-arc (variable selected)
//
// Returns the NodeID of the created or existing equivalent node.
// This method is thread-safe for concurrent construction.
func (nt *NodeTable) AddNode(level int, lo, hi NodeID) NodeID {
	// ZDD Reduction Rule: Eliminate nodes with hi-arc pointing to 0-terminal
	// This maintains the zero-suppressed property of the diagram
	if hi == ZeroNode {
		return lo
	}
	
	node := Node{Level: level, Lo: lo, Hi: hi}
	
	nt.mu.Lock()
	defer nt.mu.Unlock()
	
	// Check for existing equivalent node (structural sharing)
	if existing, exists := nt.hash[node]; exists {
		return existing
	}
	
	// Create new node with fresh ID
	id := nt.next
	nt.next++
	
	// Expand node storage if necessary
	if int(id) >= len(nt.nodes) {
		nt.nodes = append(nt.nodes, node)
	} else {
		nt.nodes[id] = node
	}
	
	// Register node for future deduplication
	nt.hash[node] = id
	return id
}

// Size returns the total number of nodes in the table, excluding NullNode.
//
// This count includes:
//   - Terminal nodes (ZeroNode, OneNode)
//   - All non-terminal nodes created during construction
//
// The size reflects the structural complexity of the ZDD.
// This method is thread-safe for concurrent access.
func (nt *NodeTable) Size() int {
	nt.mu.RLock()
	defer nt.mu.RUnlock()
	return int(nt.next) - 1 // Exclude null node from count
}
