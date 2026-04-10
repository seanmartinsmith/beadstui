//! Parallel Cut analysis algorithm.
//!
//! Identifies nodes whose completion would increase opportunities for
//! parallel work by unblocking multiple dependents.

use crate::graph::DiGraph;
use serde::Serialize;

/// A node that could increase parallelization when completed.
#[derive(Debug, Clone, Serialize)]
pub struct ParallelCutItem {
    /// Node index in the graph
    pub node: usize,
    /// Net gain in parallelization (dependents newly unblocked - 1)
    pub parallel_gain: i32,
    /// Number of dependents that would become actionable
    pub new_actionable: usize,
}

/// Result of parallel cut analysis.
#[derive(Debug, Clone, Serialize)]
pub struct ParallelCutResult {
    /// Nodes sorted by parallel gain descending
    pub items: Vec<ParallelCutItem>,
    /// Total open (non-closed) nodes considered
    pub open_nodes: usize,
    /// Current number of actionable nodes (before any changes)
    pub current_actionable: usize,
}

/// Find nodes that would increase parallelization when completed.
///
/// A node has positive parallel gain if completing it would unblock
/// more than one dependent (hence increasing parallel work opportunities).
///
/// # Arguments
/// * `graph` - The directed graph
/// * `closed_set` - Boolean array where true means the node is closed/completed
/// * `limit` - Maximum number of suggestions to return
///
/// # Returns
/// ParallelCutResult with nodes sorted by parallel gain.
pub fn parallel_cut_suggestions(
    graph: &DiGraph,
    closed_set: &[bool],
    limit: usize,
) -> ParallelCutResult {
    let n = graph.node_count();

    // Count current actionable nodes
    let current_actionable = (0..n)
        .filter(|&v| {
            !closed_set.get(v).copied().unwrap_or(false)
                && graph
                    .predecessors_slice(v)
                    .iter()
                    .all(|&p| closed_set.get(p).copied().unwrap_or(false))
        })
        .count();

    // Count open nodes
    let open_nodes = (0..n)
        .filter(|&v| !closed_set.get(v).copied().unwrap_or(false))
        .count();

    // Calculate parallel gain for each open node
    let mut suggestions: Vec<ParallelCutItem> = (0..n)
        .filter(|&v| !closed_set.get(v).copied().unwrap_or(false))
        .map(|v| {
            // Count how many dependents would become actionable if v is closed
            let new_actionable = graph
                .successors_slice(v)
                .iter()
                .filter(|&&w| {
                    // w must be open
                    !closed_set.get(w).copied().unwrap_or(false)
                    // All of w's other predecessors must be closed
                    && graph.predecessors_slice(w).iter()
                        .filter(|&&p| p != v)
                        .all(|&p| closed_set.get(p).copied().unwrap_or(false))
                })
                .count();

            // Parallel gain = newly actionable - 1 (the node itself becomes unavailable)
            // Positive means net increase in parallel work
            ParallelCutItem {
                node: v,
                parallel_gain: new_actionable as i32 - 1,
                new_actionable,
            }
        })
        .filter(|item| item.parallel_gain > 0)
        .collect();

    // Sort by parallel gain descending
    suggestions.sort_by(|a, b| b.parallel_gain.cmp(&a.parallel_gain));
    suggestions.truncate(limit);

    ParallelCutResult {
        items: suggestions,
        open_nodes,
        current_actionable,
    }
}

/// Find parallel cut suggestions with default limit of 10.
pub fn parallel_cut_default(graph: &DiGraph, closed_set: &[bool]) -> ParallelCutResult {
    parallel_cut_suggestions(graph, closed_set, 10)
}

/// Get nodes sorted by how many dependents they unblock.
/// Unlike parallel_cut_suggestions, this includes nodes with gain <= 0.
pub fn unblock_ranking(graph: &DiGraph, closed_set: &[bool], limit: usize) -> Vec<(usize, usize)> {
    let n = graph.node_count();

    let mut ranking: Vec<(usize, usize)> = (0..n)
        .filter(|&v| !closed_set.get(v).copied().unwrap_or(false))
        .map(|v| {
            let unblocks = graph
                .successors_slice(v)
                .iter()
                .filter(|&&w| {
                    !closed_set.get(w).copied().unwrap_or(false)
                        && graph
                            .predecessors_slice(w)
                            .iter()
                            .filter(|&&p| p != v)
                            .all(|&p| closed_set.get(p).copied().unwrap_or(false))
                })
                .count();
            (v, unblocks)
        })
        .collect();

    ranking.sort_by(|a, b| b.1.cmp(&a.1));
    ranking.truncate(limit);
    ranking
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
        let closed: Vec<bool> = vec![];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        assert!(result.items.is_empty());
        assert_eq!(result.open_nodes, 0);
    }

    #[test]
    fn test_single_node() {
        let mut g = DiGraph::new();
        g.add_node("a");
        let closed = vec![false];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        // Single node has no dependents, so no parallel gain
        assert!(result.items.is_empty());
        assert_eq!(result.open_nodes, 1);
    }

    #[test]
    fn test_chain_no_gain() {
        // Chain: 0 -> 1 -> 2 -> 3
        // Completing any node only unblocks 1 dependent (gain = 0)
        let g = make_graph(&[(0, 1), (1, 2), (2, 3)]);
        let closed = vec![false; 4];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        // No node has gain > 0 in a simple chain
        assert!(result.items.is_empty());
    }

    #[test]
    fn test_fork_has_gain() {
        // Fork: 0 -> 1, 0 -> 2, 0 -> 3
        // Completing 0 unblocks 3 nodes, gain = 3 - 1 = 2
        let g = make_graph(&[(0, 1), (0, 2), (0, 3)]);
        let closed = vec![false; 4];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        assert_eq!(result.items.len(), 1);
        assert_eq!(result.items[0].node, 0);
        assert_eq!(result.items[0].parallel_gain, 2); // 3 unblocked - 1 = 2
        assert_eq!(result.items[0].new_actionable, 3);
    }

    #[test]
    fn test_partially_closed() {
        // Diamond: 0 -> 1, 0 -> 2, 1 -> 3, 2 -> 3
        // If 0 is closed, then 1 and 2 are actionable
        // Completing 1 unblocks 0 (3 still blocked by 2)
        // Completing 2 unblocks 0 (3 still blocked by 1)
        let g = make_graph(&[(0, 1), (0, 2), (1, 3), (2, 3)]);
        let closed = vec![true, false, false, false];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        // Neither 1 nor 2 alone unblocks 3 (needs both)
        // So no positive gain
        assert!(result.items.is_empty());
    }

    #[test]
    fn test_diamond_with_one_closed() {
        // Diamond: 0 -> 1, 0 -> 2, 1 -> 3, 2 -> 3
        // If 0 and 1 are closed, completing 2 unblocks 3
        let g = make_graph(&[(0, 1), (0, 2), (1, 3), (2, 3)]);
        let closed = vec![true, true, false, false];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        // Node 2 is actionable and completing it unblocks 3
        // But gain = 1 - 1 = 0 (not positive)
        assert!(result.items.is_empty());
    }

    #[test]
    fn test_multiple_forks() {
        // Two forks: 0 -> {1,2}, 3 -> {4,5,6}
        // Node 0 has gain = 2 - 1 = 1
        // Node 3 has gain = 3 - 1 = 2
        let g = make_graph(&[(0, 1), (0, 2), (3, 4), (3, 5), (3, 6)]);
        let closed = vec![false; 7];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        assert!(!result.items.is_empty());
        // Node 3 should be first (higher gain)
        assert_eq!(result.items[0].node, 3);
        assert_eq!(result.items[0].parallel_gain, 2);
    }

    #[test]
    fn test_limit_respected() {
        // Many forks
        let g = make_graph(&[
            (0, 1),
            (0, 2),
            (3, 4),
            (3, 5),
            (6, 7),
            (6, 8),
            (9, 10),
            (9, 11),
        ]);
        let closed = vec![false; 12];
        let result = parallel_cut_suggestions(&g, &closed, 2);

        assert!(result.items.len() <= 2);
    }

    #[test]
    fn test_current_actionable_count() {
        // Fork: 0 -> 1, 0 -> 2
        // Initially only 0 is actionable
        let g = make_graph(&[(0, 1), (0, 2)]);
        let closed = vec![false; 3];
        let result = parallel_cut_suggestions(&g, &closed, 10);

        assert_eq!(result.current_actionable, 1); // Only node 0
        assert_eq!(result.open_nodes, 3);
    }
}
