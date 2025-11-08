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
// Optimized for cache-friendly access patterns and reduced memory overhead.
type NodeTable struct {
	mu sync.RWMutex
	
	// nodes stores the actual node data indexed by NodeID
	nodes []Node
	
	// Cache-friendly hash table using open addressing
	hashTable []hashEntry
	hashMask   uint32 // Always power of 2 minus 1
	
	next NodeID
}

// hashEntry represents a single entry in the hash table
type hashEntry struct {
	node Node
	id   NodeID
	used bool
}

// NewNodeTable creates a new node table with pre-initialized terminal nodes.
func NewNodeTable() *NodeTable {
	initialSize := uint32(1024) // Start with 1K entries
	nt := &NodeTable{
		nodes:     make([]Node, 3),
		hashTable: make([]hashEntry, initialSize),
		hashMask:  initialSize - 1,
		next:      3,
	}
	
	// Initialize terminal nodes
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
func (nt *NodeTable) AddNode(level int, lo, hi NodeID) NodeID {
	if hi == ZeroNode {
		return lo
	}
	
	node := Node{Level: level, Lo: lo, Hi: hi}
	
	nt.mu.Lock()
	defer nt.mu.Unlock()
	
	// Check for existing node using cache-friendly hash table
	if existing := nt.findNode(node); existing != NullNode {
		return existing
	}
	
	// Create new node
	id := nt.next
	nt.next++
	
	if int(id) >= len(nt.nodes) {
		nt.nodes = append(nt.nodes, node)
	} else {
		nt.nodes[id] = node
	}
	
	// Insert into hash table
	nt.insertNode(node, id)
	return id
}

// findNode searches for an existing node using open addressing
func (nt *NodeTable) findNode(node Node) NodeID {
	hash := nt.hashNode(node)
	for i := uint32(0); i < uint32(len(nt.hashTable)); i++ {
		idx := (hash + i) & nt.hashMask
		entry := &nt.hashTable[idx]
		
		if !entry.used {
			return NullNode // Not found
		}
		
		if nt.nodesEqual(entry.node, node) {
			return entry.id
		}
	}
	return NullNode
}

// insertNode adds a node to the hash table, resizing if needed
func (nt *NodeTable) insertNode(node Node, id NodeID) {
	// Resize if load factor > 0.75
	if nt.countUsed() > len(nt.hashTable)*3/4 {
		nt.resizeHashTable()
	}
	
	hash := nt.hashNode(node)
	for i := uint32(0); i < uint32(len(nt.hashTable)); i++ {
		idx := (hash + i) & nt.hashMask
		entry := &nt.hashTable[idx]
		
		if !entry.used {
			entry.node = node
			entry.id = id
			entry.used = true
			return
		}
	}
}

// hashNode computes hash for a node using fast integer operations
func (nt *NodeTable) hashNode(node Node) uint32 {
	hash := uint32(node.Level)
	hash = hash*31 + uint32(node.Lo)
	hash = hash*31 + uint32(node.Hi)
	return hash
}

// nodesEqual compares two nodes for equality
func (nt *NodeTable) nodesEqual(a, b Node) bool {
	return a.Level == b.Level && a.Lo == b.Lo && a.Hi == b.Hi
}

// countUsed counts used entries in hash table
func (nt *NodeTable) countUsed() int {
	count := 0
	for i := range nt.hashTable {
		if nt.hashTable[i].used {
			count++
		}
	}
	return count
}

// resizeHashTable doubles the hash table size
func (nt *NodeTable) resizeHashTable() {
	oldTable := nt.hashTable
	newSize := uint32(len(oldTable)) * 2
	
	nt.hashTable = make([]hashEntry, newSize)
	nt.hashMask = newSize - 1
	
	// Rehash all entries
	for i := range oldTable {
		if oldTable[i].used {
			nt.insertNode(oldTable[i].node, oldTable[i].id)
		}
	}
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
