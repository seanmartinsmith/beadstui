//! Coverage Set (Vertex Cover) algorithm.
//!
//! Finds a minimal set of nodes that "covers" all edges - useful for understanding
//! which issues touch the most dependency relationships.
//!
//! Uses a greedy 2-approximation algorithm.

use crate::graph::DiGraph;
use serde::Serialize;
use std::collections::HashSet;

/// Single item in the coverage set with contribution info.
#[derive(Debug, Clone, Serialize)]
pub struct CoverageItem {
    /// Node index in the graph
    pub node: usize,
    /// Number of new edges covered by adding this node
    pub edges_added: usize,
}

/// Result of coverage set computation.
#[derive(Debug, Clone, Serialize)]
pub struct CoverageResult {
    /// Selected nodes in order of selection
    pub items: Vec<CoverageItem>,
    /// Total edges covered by selected nodes
    pub edges_covered: usize,
    /// Total edges in the graph
    pub total_edges: usize,
    /// Coverage ratio (edges_covered / total_edges)
    pub coverage_ratio: f64,
}

/// Compute a greedy vertex cover - 2-approximation algorithm.
///
/// This greedily selects nodes that cover the most uncovered edges,
/// continuing until the limit is reached or all edges are covered.
///
/// # Arguments
/// * `graph` - The directed graph
/// * `limit` - Maximum number of nodes to select
///
/// # Returns
/// A CoverageResult containing the selected nodes and coverage statistics.
pub fn coverage_set(graph: &DiGraph, limit: usize) -> CoverageResult {
    let n = graph.node_count();
    let total_edges = graph.edge_count();

    if n == 0 || total_edges == 0 {
        return CoverageResult {
            items: Vec::new(),
            edges_covered: 0,
            total_edges,
            coverage_ratio: if total_edges == 0 { 1.0 } else { 0.0 },
        };
    }

    // Track covered edges using a HashSet for O(E) memory
    // Instead of O(V^2) with a matrix
    let mut covered: HashSet<(usize, usize)> = HashSet::new();
    let mut selected: Vec<CoverageItem> = Vec::with_capacity(limit.min(n));
    let mut edges_covered = 0;

    for _ in 0..limit {
        // Find node covering most uncovered edges
        let mut best_node: Option<usize> = None;
        let mut best_count = 0;

        for v in 0..n {
            let mut count = 0;

            // Count uncovered outgoing edges (v -> w)
            for &w in graph.successors_slice(v) {
                if !covered.contains(&(v, w)) {
                    count += 1;
                }
            }

            // Count uncovered incoming edges (u -> v)
            for &u in graph.predecessors_slice(v) {
                if !covered.contains(&(u, v)) {
                    count += 1;
                }
            }

            if count > best_count {
                best_count = count;
                best_node = Some(v);
            }
        }

        match best_node {
            Some(node) if best_count > 0 => {
                // Mark edges as covered - outgoing
                for &w in graph.successors_slice(node) {
                    covered.insert((node, w));
                }
                // Mark edges as covered - incoming
                for &u in graph.predecessors_slice(node) {
                    covered.insert((u, node));
                }

                selected.push(CoverageItem {
                    node,
                    edges_added: best_count,
                });
                edges_covered += best_count;
            }
            _ => break, // No more uncovered edges or no more nodes
        }
    }

    let coverage_ratio = if total_edges > 0 {
        edges_covered as f64 / total_edges as f64
    } else {
        1.0
    };

    CoverageResult {
        items: selected,
        edges_covered,
        total_edges,
        coverage_ratio,
    }
}

/// Compute coverage set with default limit of 10.
pub fn coverage_set_default(graph: &DiGraph) -> CoverageResult {
    coverage_set(graph, 10)
}

/// Get just the node indices from a coverage set computation.
pub fn coverage_nodes(graph: &DiGraph, limit: usize) -> Vec<usize> {
    coverage_set(graph, limit)
        .items
        .into_iter()
        .map(|item| item.node)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_graph(edges: &[(usize, usize)]) -> DiGraph {
        let mut g = DiGraph::new();
        let max_node = edges
            .iter()
            .flat_map(|(a, b)| [*a, *b])
            .max()
            .unwrap_or(0);
        for i in 0..=max_node {
            g.add_node(&format!("n{}", i));
        }
        for (from, to) in edges {
            g.add_edge(*from, *to);
        }
        g
    }

    #[test]
    fn test_empty_graph() {
        let g = DiGraph::new();
        let result = coverage_set(&g, 5);
        assert!(result.items.is_empty());
        assert_eq!(result.edges_covered, 0);
        assert_eq!(result.total_edges, 0);
        assert_eq!(result.coverage_ratio, 1.0);
    }

    #[test]
    fn test_single_edge() {
        let g = make_graph(&[(0, 1)]);
        let result = coverage_set(&g, 5);

        // Either node 0 or 1 should be selected, covering the single edge
        assert_eq!(result.items.len(), 1);
        assert_eq!(result.edges_covered, 1);
        assert_eq!(result.total_edges, 1);
        assert_eq!(result.coverage_ratio, 1.0);
    }

    #[test]
    fn test_star_graph() {
        // Center node 0 with edges to 1, 2, 3
        let g = make_graph(&[(0, 1), (0, 2), (0, 3)]);
        let result = coverage_set(&g, 5);

        // Node 0 should be selected first as it covers all 3 edges
        assert!(!result.items.is_empty());
        assert_eq!(result.items[0].node, 0);
        assert_eq!(result.items[0].edges_added, 3);
        assert_eq!(result.edges_covered, 3);
        assert_eq!(result.coverage_ratio, 1.0);
    }

    #[test]
    fn test_chain_graph() {
        // Chain: 0 -> 1 -> 2 -> 3
        let g = make_graph(&[(0, 1), (1, 2), (2, 3)]);
        let result = coverage_set(&g, 5);

        // Node 1 or 2 should be selected first (each covers 2 edges)
        assert!(!result.items.is_empty());
        let first_node = result.items[0].node;
        assert!(first_node == 1 || first_node == 2);
        assert_eq!(result.items[0].edges_added, 2);
    }

    #[test]
    fn test_limit_respected() {
        // Larger graph
        let g = make_graph(&[(0, 1), (1, 2), (2, 3), (3, 4), (4, 5)]);
        let result = coverage_set(&g, 2);

        assert!(result.items.len() <= 2);
    }

    #[test]
    fn test_coverage_ratio() {
        let g = make_graph(&[(0, 1), (2, 3), (4, 5)]); // 3 disconnected edges
        let result = coverage_set(&g, 1);

        // With limit 1, we cover at most 1 edge
        assert_eq!(result.items.len(), 1);
        assert_eq!(result.edges_covered, 1);
        assert!((result.coverage_ratio - 1.0 / 3.0).abs() < 0.001);
    }

    #[test]
    fn test_bidirectional_edges() {
        // A <-> B (mutual dependency)
        let g = make_graph(&[(0, 1), (1, 0)]);
        let result = coverage_set(&g, 5);

        // One node should cover both edges
        assert_eq!(result.items.len(), 1);
        assert_eq!(result.edges_covered, 2);
        assert_eq!(result.total_edges, 2);
    }
}
