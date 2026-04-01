// Adapted from github.com/zjrosen/perles (MIT License). See LICENSE in this directory.

package bql

// Node is the interface for all AST nodes.
type Node interface {
	node()
}

// Expr is the interface for expression nodes.
type Expr interface {
	Node
	expr()
}

// Query represents a complete BQL query.
type Query struct {
	Filter  Expr          // The filter expression (may be nil for ORDER BY only queries)
	Expand  *ExpandClause // Expansion config (may be nil for no expansion)
	OrderBy []OrderTerm   // ORDER BY terms (may be empty)
}

func (q *Query) node() {}

// HasExpand returns true if the query has an expansion clause.
func (q *Query) HasExpand() bool {
	return q.Expand != nil && q.Expand.Type != ExpandNone
}

// BinaryExpr represents "expr AND/OR expr".
type BinaryExpr struct {
	Left  Expr
	Op    TokenType // TokenAnd or TokenOr
	Right Expr
}

func (b *BinaryExpr) node() {}
func (b *BinaryExpr) expr() {}

// NotExpr represents "NOT expr".
type NotExpr struct {
	Expr Expr
}

func (n *NotExpr) node() {}
func (n *NotExpr) expr() {}

// CompareExpr represents "field op value".
type CompareExpr struct {
	Field string
	Op    TokenType
	Value Value
}

func (c *CompareExpr) node() {}
func (c *CompareExpr) expr() {}

// InExpr represents "field IN (values)" or "field NOT IN (values)".
type InExpr struct {
	Field  string
	Values []Value
	Not    bool // true for "NOT IN"
}

func (i *InExpr) node() {}
func (i *InExpr) expr() {}

// ValueType indicates the type of a Value.
type ValueType int

const (
	ValueString ValueType = iota
	ValueInt
	ValueBool
	ValuePriority // P0, P1, P2, P3, P4
	ValueDate     // today, yesterday, -7d, ISO dates
)

// Value represents a literal value in a query.
type Value struct {
	Type   ValueType
	Raw    string // Original string representation
	String string // String value (for ValueString, ValuePriority, ValueDate)
	Int    int    // Integer value (for ValueInt, also priority level for ValuePriority)
	Bool   bool   // Boolean value (for ValueBool)
}

// OrderTerm represents a single ORDER BY term.
type OrderTerm struct {
	Field string
	Desc  bool // true for DESC, false for ASC (default)
}

// ExpandType specifies which direction to expand relationships.
type ExpandType int

const (
	ExpandNone ExpandType = iota
	ExpandUp              // Traverse toward dependencies (parent + blockers)
	ExpandDown            // Traverse toward dependents (children + blocks)
	ExpandAll             // Traverse both directions
)

// String returns the string representation of the ExpandType.
func (e ExpandType) String() string {
	switch e {
	case ExpandUp:
		return "up"
	case ExpandDown:
		return "down"
	case ExpandAll:
		return "all"
	default:
		return "none"
	}
}

// ExpandDepth represents how deep to expand relationships.
type ExpandDepth int

const (
	DepthDefault   ExpandDepth = 1  // Default: expand one level
	DepthUnlimited ExpandDepth = -1 // Unlimited: expand until no more found
	DepthMax       ExpandDepth = 10 // Maximum allowed explicit depth
)

// ExpandClause represents the EXPAND clause configuration.
type ExpandClause struct {
	Type  ExpandType  // Which relationships to expand
	Depth ExpandDepth // How many levels to expand (1 = default, -1 = unlimited)
}
