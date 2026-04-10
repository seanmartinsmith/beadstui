//! Subgraph extraction and operations.
//!
//! Creates new graphs containing only specified nodes and their interconnections.
//! Essential for filtered-view analysis where you want to run algorithms on
//! a subset of issues (e.g., "PageRank for just 'auth' label issues").

use crate::graph::DiGraph;
use std::collections::HashMap;

/// Extract a subgraph containing only the specified node indices.
///
/// Creates a new DiGraph with:
/// - Only the nodes at the given indices
/// - Only edges that connect nodes within the subset
///
/// # Arguments
/// * `graph` - The source graph
/// * `node_indices` - Indices of nodes to include in the subgraph
///
/// # Returns
/// New DiGraph containing only the specified nodes and their interconnecting edges.
/// Node indices in the new graph are renumbered 0..n.
pub fn extract_subgraph(graph: &DiGraph, node_indices: &[usize]) -> DiGraph {
    let n = graph.len();
    if node_indices.is_empty() || n == 0 {
        return DiGraph::new();
    }

    // Create mapping: old index -> new index
    let mut index_map: HashMap<usize, usize> = HashMap::with_capacity(node_indices.len());
    let mut new_graph = DiGraph::with_capacity(node_indices.len(), node_indices.len() * 2);

    // Add nodes to new graph
    for &old_idx in node_indices {
        if old_idx < n {
            if let Some(id) = graph.node_id(old_idx) {
                let new_idx = new_graph.add_node(&id);
                index_map.insert(old_idx, new_idx);
            }
        }
    }

    // Add edges between retained nodes
    for &old_from in node_indices {
        if let Some(&new_from) = index_map.get(&old_from) {
            for &old_to in graph.successors_slice(old_from) {
                if let Some(&new_to) = index_map.get(&old_to) {
                    new_graph.add_edge(new_from, new_to);
                }
            }
        }
    }

    new_graph
}

/// Extract a subgraph by node IDs (string lookup).
///
/// Convenience wrapper that looks up indices by ID string first.
pub fn extract_subgraph_by_ids(graph: &DiGraph, ids: &[&str]) -> DiGraph {
    let indices: Vec<usize> = ids.iter().filter_map(|id| graph.node_idx(id)).collect();
    extract_subgraph(graph, &indices)
}

/// Get the induced subgraph on reachable nodes from a source.
///
/// Returns a subgraph containing all nodes reachable from `source`
/// (via outgoing edges), plus the source itself.
pub fn reachable_subgraph_from(graph: &DiGraph, source: usize) -> DiGraph {
    let reachable = reachable_from(graph, source);
    extract_subgraph(graph, &reachable)
}

/// Get nodes reachable from a source node (outgoing direction).
///
/// Uses BFS to find all nodes that can be reached by following
/// outgoing edges from the source.
pub fn reachable_from(graph: &DiGraph, source: usize) -> Vec<usize> {
    let n = graph.len();
    if source >= n {
        return Vec::new();
    }

    let mut visited = vec![false; n];
    let mut result = Vec::new();
    let mut queue = std::collections::VecDeque::new();

    visited[source] = true;
    result.push(source);
    queue.push_back(source);

    while let Some(v) = queue.pop_front() {
        for &w in graph.successors_slice(v) {
            if !visited[w] {
                visited[w] = true;
                result.push(w);
                queue.push_back(w);
            }
        }
    }

    result
}

/// Get nodes that can reach a target node (incoming direction).
///
/// Uses BFS on reverse edges to find all nodes that can reach
/// the target by following outgoing edges.
pub fn reachable_to(graph: &DiGraph, target: usize) -> Vec<usize> {
    let n = graph.len();
    if target >= n {
        return Vec::new();
    }

    let mut visited = vec![false; n];
    let mut result = Vec::new();
    let mut queue = std::collections::VecDeque::new();

    visited[target] = true;
    result.push(target);
    queue.push_back(target);

    while let Some(v) = queue.pop_front() {
        for &w in graph.predecessors_slice(v) {
            if !visited[w] {
                visited[w] = true;
                result.push(w);
                queue.push_back(w);
            }
        }
    }

    result
}

/// Get nodes in the dependency cone of a target (all ancestors + target + all descendants).
pub fn dependency_cone(graph: &DiGraph, node: usize) -> Vec<usize> {
    let from = reachable_to(graph, node);
    let to = reachable_from(graph, node);

    let mut all: Vec<usize> = from;
    for v in to {
        if !all.contains(&v) {
            all.push(v);
        }
    }
    all
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_subgraph_empty() {
        let graph = DiGraph::new();
        let sub = extract_subgraph(&graph, &[]);
        assert_eq!(sub.node_count(), 0);
        assert_eq!(sub.edge_count(), 0);
    }

    #[test]
    fn test_subgraph_single_node() {
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        graph.add_node("b");

        let sub = extract_subgraph(&graph, &[a]);
        assert_eq!(sub.node_count(), 1);
        assert_eq!(sub.edge_count(), 0);
        assert_eq!(sub.node_id(0), Some("a".to_string()));
    }

    #[test]
    fn test_subgraph_preserves_edges() {
        // a -> b -> c -> d
        // Subgraph [b, c] should have edge b->c
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, d);

        let sub = extract_subgraph(&graph, &[b, c]);
        assert_eq!(sub.node_count(), 2);
        assert_eq!(sub.edge_count(), 1);
    }

    #[test]
    fn test_subgraph_drops_external_edges() {
        // a -> b -> c
        // Subgraph [a, c] should have no edges (b is dropped)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let sub = extract_subgraph(&graph, &[a, c]);
        assert_eq!(sub.node_count(), 2);
        assert_eq!(sub.edge_count(), 0);
    }

    #[test]
    fn test_subgraph_diamond() {
        //     a
        //    / \
        //   b   c
        //    \ /
        //     d
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(b, d);
        graph.add_edge(c, d);

        // Full subgraph should preserve structure
        let sub = extract_subgraph(&graph, &[a, b, c, d]);
        assert_eq!(sub.node_count(), 4);
        assert_eq!(sub.edge_count(), 4);

        // Partial subgraph [a, b, d] loses a->c and c->d
        let sub2 = extract_subgraph(&graph, &[a, b, d]);
        assert_eq!(sub2.node_count(), 3);
        assert_eq!(sub2.edge_count(), 2); // a->b and b->d
    }

    #[test]
    fn test_subgraph_by_ids() {
        let mut graph = DiGraph::new();
        graph.add_node("bv-1");
        graph.add_node("bv-2");
        graph.add_node("bv-3");
        graph.add_edge(0, 1);
        graph.add_edge(1, 2);

        let sub = extract_subgraph_by_ids(&graph, &["bv-1", "bv-2"]);
        assert_eq!(sub.node_count(), 2);
        assert_eq!(sub.edge_count(), 1);
    }

    #[test]
    fn test_reachable_from() {
        // a -> b -> c
        //      |
        //      v
        //      d
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(b, d);

        let from_a = reachable_from(&graph, a);
        assert_eq!(from_a.len(), 4); // a, b, c, d

        let from_b = reachable_from(&graph, b);
        assert_eq!(from_b.len(), 3); // b, c, d
        assert!(from_b.contains(&b));
        assert!(from_b.contains(&c));
        assert!(from_b.contains(&d));

        let from_c = reachable_from(&graph, c);
        assert_eq!(from_c.len(), 1); // just c
    }

    #[test]
    fn test_reachable_to() {
        // a -> b -> c
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let to_c = reachable_to(&graph, c);
        assert_eq!(to_c.len(), 3); // c, b, a

        let to_b = reachable_to(&graph, b);
        assert_eq!(to_b.len(), 2); // b, a

        let to_a = reachable_to(&graph, a);
        assert_eq!(to_a.len(), 1); // just a
    }

    #[test]
    fn test_dependency_cone() {
        //     a
        //     |
        //     b (target)
        //    / \
        //   c   d
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(b, d);

        let cone = dependency_cone(&graph, b);
        assert_eq!(cone.len(), 4); // a, b, c, d
    }

    #[test]
    fn test_reachable_subgraph() {
        // a -> b -> c
        //      |
        //      v
        //      d
        // e (disconnected)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_node("e");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(b, d);

        let sub = reachable_subgraph_from(&graph, a);
        assert_eq!(sub.node_count(), 4); // a, b, c, d (not e)
        assert_eq!(sub.edge_count(), 3); // a->b, b->c, b->d
    }

    #[test]
    fn test_subgraph_runs_algorithms() {
        // Verify that algorithms work on extracted subgraphs
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let sub = extract_subgraph(&graph, &[a, b, c]);

        // Should be able to run pagerank on subgraph
        use crate::algorithms::pagerank::pagerank_default;
        let pr = pagerank_default(&sub);
        assert_eq!(pr.len(), 3);

        // Should be a DAG
        use crate::algorithms::topo::is_dag;
        assert!(is_dag(&sub));
    }

    #[test]
    fn test_subgraph_invalid_indices() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        graph.add_node("b");

        // Invalid indices should be ignored
        let sub = extract_subgraph(&graph, &[0, 999, 1]);
        assert_eq!(sub.node_count(), 2);
    }

    #[test]
    fn test_subgraph_duplicate_indices() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        graph.add_node("b");
        graph.add_edge(0, 1);

        // Duplicate indices should be handled
        let sub = extract_subgraph(&graph, &[0, 0, 1, 1]);
        // add_node is idempotent, so we should still get 2 nodes
        assert_eq!(sub.node_count(), 2);
        assert_eq!(sub.edge_count(), 1);
    }
}
