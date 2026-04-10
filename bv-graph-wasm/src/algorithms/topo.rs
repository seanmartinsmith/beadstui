//! Topological Sort using Kahn's algorithm.
//!
//! Orders nodes such that for every edge u→v, u comes before v.
//! Essential for execution planning and critical path analysis.

use crate::graph::DiGraph;
use std::cmp::Reverse;
use std::collections::BinaryHeap;

/// Topological sort result.
pub struct TopoSortResult {
    /// Sorted node indices (valid only if is_dag is true)
    pub order: Vec<usize>,
    /// Whether the graph is a DAG
    pub is_dag: bool,
}

/// Topological sort using Kahn's algorithm with deterministic ordering.
///
/// Uses a min-heap to ensure consistent output across runs.
/// Returns None if the graph contains cycles.
///
/// # Arguments
/// * `graph` - The directed graph to sort
///
/// # Returns
/// * `Some(order)` - Vector of node indices in topological order
/// * `None` - If the graph contains cycles
pub fn topological_sort(graph: &DiGraph) -> Option<Vec<usize>> {
    let n = graph.len();
    if n == 0 {
        return Some(Vec::new());
    }

    // Compute in-degrees
    let mut in_degree: Vec<usize> = (0..n).map(|i| graph.in_degree(i)).collect();

    // Min-heap for deterministic ordering (process lowest index first)
    let mut heap: BinaryHeap<Reverse<usize>> = (0..n)
        .filter(|&i| in_degree[i] == 0)
        .map(Reverse)
        .collect();

    let mut order = Vec::with_capacity(n);

    while let Some(Reverse(u)) = heap.pop() {
        order.push(u);

        for &v in graph.successors_slice(u) {
            in_degree[v] -= 1;
            if in_degree[v] == 0 {
                heap.push(Reverse(v));
            }
        }
    }

    if order.len() == n {
        Some(order)
    } else {
        None // Cycle detected
    }
}

/// Check if the graph is a DAG (directed acyclic graph).
///
/// A graph is a DAG if and only if it has a valid topological order.
pub fn is_dag(graph: &DiGraph) -> bool {
    topological_sort(graph).is_some()
}

/// Compute topological sort with detailed result.
pub fn topological_sort_result(graph: &DiGraph) -> TopoSortResult {
    match topological_sort(graph) {
        Some(order) => TopoSortResult { order, is_dag: true },
        None => TopoSortResult {
            order: Vec::new(),
            is_dag: false,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_graph() {
        let g = DiGraph::new();
        let result = topological_sort(&g);
        assert_eq!(result, Some(vec![]));
    }

    #[test]
    fn test_single_node() {
        let mut g = DiGraph::new();
        g.add_node("a");
        let result = topological_sort(&g);
        assert_eq!(result, Some(vec![0]));
    }

    #[test]
    fn test_linear_chain() {
        // a -> b -> c
        let mut g = DiGraph::new();
        let a = g.add_node("a");
        let b = g.add_node("b");
        let c = g.add_node("c");
        g.add_edge(a, b);
        g.add_edge(b, c);

        let result = topological_sort(&g).unwrap();
        assert_eq!(result, vec![0, 1, 2]); // a, b, c
    }

    #[test]
    fn test_diamond() {
        //     a
        //    / \
        //   b   c
        //    \ /
        //     d
        let mut g = DiGraph::new();
        let a = g.add_node("a");
        let b = g.add_node("b");
        let c = g.add_node("c");
        let d = g.add_node("d");
        g.add_edge(a, b);
        g.add_edge(a, c);
        g.add_edge(b, d);
        g.add_edge(c, d);

        let result = topological_sort(&g).unwrap();
        // Valid orders: [a, b, c, d] or [a, c, b, d]
        // With min-heap, should be [a, b, c, d]
        assert_eq!(result[0], a); // a first
        assert_eq!(result[3], d); // d last
        // b before c due to min-heap ordering
        assert_eq!(result, vec![0, 1, 2, 3]);
    }

    #[test]
    fn test_cycle_detection() {
        // a -> b -> c -> a
        let mut g = DiGraph::new();
        let a = g.add_node("a");
        let b = g.add_node("b");
        let c = g.add_node("c");
        g.add_edge(a, b);
        g.add_edge(b, c);
        g.add_edge(c, a);

        let result = topological_sort(&g);
        assert!(result.is_none());
    }

    #[test]
    fn test_self_loop() {
        // a -> a
        let mut g = DiGraph::new();
        let a = g.add_node("a");
        g.add_edge(a, a);

        let result = topological_sort(&g);
        assert!(result.is_none());
    }

    #[test]
    fn test_disconnected() {
        // Two separate components: a->b, c->d
        let mut g = DiGraph::new();
        let a = g.add_node("a");
        let b = g.add_node("b");
        let c = g.add_node("c");
        let d = g.add_node("d");
        g.add_edge(a, b);
        g.add_edge(c, d);

        let result = topological_sort(&g).unwrap();
        // With min-heap: [0, 2, 1, 3] - process by index
        // a (0) and c (2) have in-degree 0, min-heap pops 0 first
        assert_eq!(result.len(), 4);
        // a comes before b
        let a_pos = result.iter().position(|&x| x == a).unwrap();
        let b_pos = result.iter().position(|&x| x == b).unwrap();
        assert!(a_pos < b_pos);
        // c comes before d
        let c_pos = result.iter().position(|&x| x == c).unwrap();
        let d_pos = result.iter().position(|&x| x == d).unwrap();
        assert!(c_pos < d_pos);
    }

    #[test]
    fn test_is_dag() {
        let mut dag = DiGraph::new();
        let a = dag.add_node("a");
        let b = dag.add_node("b");
        dag.add_edge(a, b);
        assert!(is_dag(&dag));

        let mut cyclic = DiGraph::new();
        let x = cyclic.add_node("x");
        let y = cyclic.add_node("y");
        cyclic.add_edge(x, y);
        cyclic.add_edge(y, x);
        assert!(!is_dag(&cyclic));
    }

    #[test]
    fn test_deterministic() {
        // Run multiple times, should always get same result
        let mut g = DiGraph::new();
        for i in 0..10 {
            g.add_node(&format!("node{}", i));
        }
        // Add some edges
        g.add_edge(0, 5);
        g.add_edge(1, 5);
        g.add_edge(2, 6);
        g.add_edge(3, 7);
        g.add_edge(5, 8);
        g.add_edge(6, 8);
        g.add_edge(7, 9);
        g.add_edge(8, 9);

        let result1 = topological_sort(&g).unwrap();
        let result2 = topological_sort(&g).unwrap();
        let result3 = topological_sort(&g).unwrap();

        assert_eq!(result1, result2);
        assert_eq!(result2, result3);
    }
}
