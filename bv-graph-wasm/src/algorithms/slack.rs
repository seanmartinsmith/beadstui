//! Slack computation for critical path analysis.
//!
//! Slack measures how much a task can be delayed without affecting
//! the overall project completion time (critical path length).
//! Nodes with zero slack are on the critical path.

use crate::algorithms::topo::topological_sort;
use crate::graph::DiGraph;

/// Compute slack for each node in a DAG.
///
/// Slack = (critical path length) - (longest path through this node)
///
/// Where "longest path through node v" = dist_from_start[v] + dist_to_end[v]
///
/// # Algorithm
/// 1. Topological sort
/// 2. Forward pass: compute longest distance from any start node
/// 3. Backward pass: compute longest distance to any end node
/// 4. Slack = max_path_length - (forward + backward distances)
///
/// # Returns
/// Vector of slack values indexed by node. Returns zeros for cyclic graphs.
pub fn slack(graph: &DiGraph) -> Vec<f64> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    // Get topological order (None if cyclic)
    let order = match topological_sort(graph) {
        Some(o) => o,
        None => return vec![0.0; n], // Return zeros for cyclic graphs
    };

    // Forward pass: longest distance from any start (nodes with no predecessors)
    // dist_from_start[v] = length of longest path from any root to v
    let mut dist_from_start = vec![0usize; n];
    for &v in &order {
        let max_pred = graph
            .predecessors_slice(v)
            .iter()
            .map(|&u| dist_from_start[u])
            .max()
            .unwrap_or(0);
        dist_from_start[v] = max_pred + 1;
    }

    // Backward pass: longest distance to any end (nodes with no successors)
    // dist_to_end[v] = length of longest path from v to any leaf
    let mut dist_to_end = vec![0usize; n];
    for &v in order.iter().rev() {
        let max_succ = graph
            .successors_slice(v)
            .iter()
            .map(|&w| dist_to_end[w])
            .max()
            .unwrap_or(0);
        dist_to_end[v] = max_succ + 1;
    }

    // Find the longest path length in the entire graph
    // longest_path_length = max(dist_from_start[i] + dist_to_end[i] - 1) for all i
    // (we subtract 1 because node v is counted in both distances)
    let longest_path: usize = (0..n)
        .map(|i| dist_from_start[i] + dist_to_end[i] - 1)
        .max()
        .unwrap_or(0);

    // Slack = longest_path - (dist_from_start + dist_to_end - 1)
    (0..n)
        .map(|i| {
            let path_through_i = dist_from_start[i] + dist_to_end[i] - 1;
            (longest_path - path_through_i) as f64
        })
        .collect()
}

/// Get nodes with zero slack (on the critical path).
pub fn zero_slack_nodes(graph: &DiGraph) -> Vec<usize> {
    let slacks = slack(graph);
    slacks
        .iter()
        .enumerate()
        .filter_map(|(i, &s)| if s < 0.001 { Some(i) } else { None })
        .collect()
}

/// Get total float (maximum slack in the graph).
pub fn total_float(graph: &DiGraph) -> f64 {
    slack(graph).into_iter().fold(0.0, f64::max)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_slack_empty() {
        let graph = DiGraph::new();
        let s = slack(&graph);
        assert!(s.is_empty());
    }

    #[test]
    fn test_slack_single_node() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        let s = slack(&graph);
        assert_eq!(s.len(), 1);
        assert_eq!(s[0], 0.0); // Single node is on critical path
    }

    #[test]
    fn test_slack_chain() {
        // a -> b -> c
        // All nodes on critical path, all slack = 0
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let s = slack(&graph);
        assert_eq!(s[a], 0.0);
        assert_eq!(s[b], 0.0);
        assert_eq!(s[c], 0.0);
    }

    #[test]
    fn test_slack_parallel_chains() {
        // a -> b -> c (length 3)
        // d -> e      (length 2)
        // d and e have slack 1
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(d, e);

        let s = slack(&graph);
        // Critical path is a->b->c (length 3)
        assert_eq!(s[a], 0.0);
        assert_eq!(s[b], 0.0);
        assert_eq!(s[c], 0.0);
        // Shorter chain has slack
        assert_eq!(s[d], 1.0);
        assert_eq!(s[e], 1.0);
    }

    #[test]
    fn test_slack_diamond() {
        //     a (slack 0)
        //    / \
        //   b   c  (both slack 0)
        //    \ /
        //     d (slack 0)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(b, d);
        graph.add_edge(c, d);

        let s = slack(&graph);
        // Both paths a->b->d and a->c->d have same length
        // All nodes on critical path
        assert_eq!(s[a], 0.0);
        assert_eq!(s[b], 0.0);
        assert_eq!(s[c], 0.0);
        assert_eq!(s[d], 0.0);
    }

    #[test]
    fn test_slack_with_shortcut() {
        //     a
        //    / \
        //   b   |
        //    \ /
        //     c
        // Path a->b->c is longer than a->c
        // a->c direct edge has slack 1
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(a, c); // Shortcut

        let s = slack(&graph);
        // Critical path is a->b->c (length 3)
        assert_eq!(s[a], 0.0);
        assert_eq!(s[b], 0.0);
        assert_eq!(s[c], 0.0);
        // The direct edge a->c creates a shorter path, but slack is per-node
        // All nodes are still on critical path here
    }

    #[test]
    fn test_slack_branch_with_different_lengths() {
        //     a
        //    / \
        //   b   c -> d -> e
        //   |
        //   f
        // Left: a->b->f (length 3)
        // Right: a->c->d->e (length 4)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        let f = graph.add_node("f");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(b, f);
        graph.add_edge(c, d);
        graph.add_edge(d, e);

        let s = slack(&graph);
        // Critical path: a->c->d->e (length 4)
        assert_eq!(s[a], 0.0);
        assert_eq!(s[c], 0.0);
        assert_eq!(s[d], 0.0);
        assert_eq!(s[e], 0.0);
        // Shorter path: a->b->f (length 3) has slack 1
        assert_eq!(s[b], 1.0);
        assert_eq!(s[f], 1.0);
    }

    #[test]
    fn test_slack_cyclic() {
        // a -> b -> c -> a
        // Should return zeros for cyclic graphs
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let s = slack(&graph);
        assert_eq!(s, vec![0.0, 0.0, 0.0]);
    }

    #[test]
    fn test_zero_slack_nodes() {
        // a -> b -> c
        // d -> e
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(d, e);

        let critical = zero_slack_nodes(&graph);
        assert_eq!(critical.len(), 3);
        assert!(critical.contains(&a));
        assert!(critical.contains(&b));
        assert!(critical.contains(&c));
    }

    #[test]
    fn test_total_float() {
        // a -> b -> c (length 3)
        // d (isolated, length 1)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let _d = graph.add_node("d"); // isolated node for testing slack
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let tf = total_float(&graph);
        // d has slack = 3-1 = 2
        assert_eq!(tf, 2.0);
    }

    #[test]
    fn test_slack_non_negative() {
        // Various graph structures should never produce negative slack
        let mut graph = DiGraph::new();
        for i in 0..10 {
            graph.add_node(&format!("n{}", i));
        }
        // Add some edges
        graph.add_edge(0, 1);
        graph.add_edge(1, 2);
        graph.add_edge(2, 3);
        graph.add_edge(0, 4);
        graph.add_edge(4, 5);
        graph.add_edge(5, 6);
        graph.add_edge(6, 3);

        let s = slack(&graph);
        for (i, &slack_val) in s.iter().enumerate() {
            assert!(
                slack_val >= 0.0,
                "Node {} has negative slack: {}",
                i,
                slack_val
            );
        }
    }
}
