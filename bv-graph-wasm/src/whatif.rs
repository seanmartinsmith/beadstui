//! What-If simulation for issue unblock cascade analysis.
//!
//! What-If analysis answers "If I close issue X, what happens?"
//! It computes direct unblocks, transitive cascades, and impact metrics.

use crate::graph::DiGraph;
use crate::reachability::{actionable_nodes, is_actionable};
use serde::Serialize;
use std::collections::VecDeque;

/// Result of a what-if simulation for closing a single node.
#[derive(Debug, Clone, Serialize)]
pub struct WhatIfResult {
    /// Number of issues directly unblocked (immediate dependents with all blockers satisfied)
    pub direct_unblocks: usize,
    /// Total issues transitively unblocked (full cascade)
    pub transitive_unblocks: usize,
    /// Indices of directly unblocked issues
    pub unblocked_ids: Vec<usize>,
    /// Indices of all transitively unblocked issues (includes direct)
    pub cascade_ids: Vec<usize>,
    /// Parallelization gain (new parallel opportunities created)
    pub parallel_gain: i32,
}

impl WhatIfResult {
    /// Create an empty result (no impact).
    pub fn empty() -> Self {
        WhatIfResult {
            direct_unblocks: 0,
            transitive_unblocks: 0,
            unblocked_ids: Vec::new(),
            cascade_ids: Vec::new(),
            parallel_gain: 0,
        }
    }
}

/// Compute what happens if a node is "closed" (removed from blocking consideration).
///
/// # Arguments
/// * `graph` - The dependency graph
/// * `node` - The node to simulate closing
/// * `closed_set` - Boolean array indicating which nodes are already closed
///
/// # Returns
/// WhatIfResult with direct unblocks, transitive cascade, and impact metrics.
pub fn what_if_close(graph: &DiGraph, node: usize, closed_set: &[bool]) -> WhatIfResult {
    let n = graph.len();
    if node >= n || closed_set.get(node).copied().unwrap_or(false) {
        // Node doesn't exist or is already closed
        return WhatIfResult::empty();
    }

    // Create new closed set with this node added
    let mut new_closed = closed_set.to_vec();
    new_closed.resize(n, false);
    new_closed[node] = true;

    // Find issues that become actionable (directly unblocked)
    // These are successors of node that had all other blockers already closed
    let mut direct_unblocks = Vec::new();

    for &successor in graph.successors_slice(node) {
        if new_closed[successor] {
            continue;
        }

        // Was this successor blocked before?
        let was_blocked = !is_actionable(graph, successor, closed_set);

        // Is it unblocked now?
        let now_unblocked = is_actionable(graph, successor, &new_closed);

        if was_blocked && now_unblocked {
            direct_unblocks.push(successor);
        }
    }

    // Count transitive unblocks (cascade effect)
    // BFS from direct unblocks, adding nodes as they become actionable
    let cascade_ids = count_cascade(graph, &direct_unblocks, &new_closed);

    let transitive_count = cascade_ids.len();
    let direct_count = direct_unblocks.len();

    WhatIfResult {
        direct_unblocks: direct_count,
        transitive_unblocks: transitive_count,
        unblocked_ids: direct_unblocks,
        cascade_ids,
        parallel_gain: direct_count.saturating_sub(1) as i32,
    }
}

/// Count the cascade of nodes that become actionable starting from roots.
///
/// Uses BFS simulation where we "close" each unblocked node and check
/// what else becomes actionable.
fn count_cascade(graph: &DiGraph, roots: &[usize], initial_closed: &[bool]) -> Vec<usize> {
    let n = graph.len();
    if n == 0 || roots.is_empty() {
        return roots.to_vec();
    }

    let mut closed = initial_closed.to_vec();
    closed.resize(n, false);

    let mut visited = vec![false; n];
    let mut cascade = Vec::new();
    let mut queue: VecDeque<usize> = VecDeque::new();

    // Initialize with roots
    for &root in roots {
        if root < n && !visited[root] && !closed[root] {
            visited[root] = true;
            cascade.push(root);
            queue.push_back(root);
        }
    }

    // BFS: simulate completing each node and check what unblocks
    while let Some(v) = queue.pop_front() {
        // Mark this node as "completed" for cascade purposes
        closed[v] = true;

        // Check successors
        for &w in graph.successors_slice(v) {
            if visited[w] || closed[w] {
                continue;
            }

            // Check if all predecessors of w are now resolved
            let all_resolved = graph
                .predecessors_slice(w)
                .iter()
                .all(|&p| closed[p] || visited[p]);

            if all_resolved {
                visited[w] = true;
                cascade.push(w);
                queue.push_back(w);
            }
        }
    }

    cascade
}

/// Result entry for top what-if ranking.
#[derive(Debug, Clone, Serialize)]
pub struct TopWhatIfEntry {
    /// Node index
    pub node: usize,
    /// What-if result for this node
    pub result: WhatIfResult,
}

/// Find top N issues with highest cascade impact.
///
/// # Arguments
/// * `graph` - The dependency graph
/// * `closed_set` - Boolean array indicating which nodes are already closed
/// * `limit` - Maximum number of results to return
///
/// # Returns
/// Vector of (node, WhatIfResult) sorted by transitive_unblocks descending.
pub fn top_what_if(graph: &DiGraph, closed_set: &[bool], limit: usize) -> Vec<TopWhatIfEntry> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    // Get currently actionable nodes (candidates for closing)
    let candidates = actionable_nodes(graph, closed_set);

    let mut results: Vec<TopWhatIfEntry> = candidates
        .into_iter()
        .map(|node| {
            let result = what_if_close(graph, node, closed_set);
            TopWhatIfEntry { node, result }
        })
        .filter(|e| e.result.transitive_unblocks > 0)
        .collect();

    // Sort by transitive impact (descending)
    results.sort_by(|a, b| {
        b.result
            .transitive_unblocks
            .cmp(&a.result.transitive_unblocks)
    });

    results.truncate(limit);
    results
}

/// Find all issues with any unblock potential.
///
/// Similar to top_what_if but returns all issues (not just actionable ones)
/// sorted by their cascade impact. Useful for identifying high-impact blocked items.
pub fn all_what_if(graph: &DiGraph, closed_set: &[bool], limit: usize) -> Vec<TopWhatIfEntry> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    let mut closed = closed_set.to_vec();
    closed.resize(n, false);

    let mut results: Vec<TopWhatIfEntry> = (0..n)
        .filter(|&i| !closed[i])
        .map(|node| {
            let result = what_if_close(graph, node, &closed);
            TopWhatIfEntry { node, result }
        })
        .filter(|e| e.result.transitive_unblocks > 0)
        .collect();

    results.sort_by(|a, b| {
        b.result
            .transitive_unblocks
            .cmp(&a.result.transitive_unblocks)
    });

    results.truncate(limit);
    results
}

/// Batch what-if: compute impact of closing multiple nodes at once.
///
/// # Arguments
/// * `graph` - The dependency graph
/// * `nodes` - Nodes to simulate closing together
/// * `closed_set` - Boolean array indicating which nodes are already closed
///
/// # Returns
/// Combined WhatIfResult for closing all specified nodes.
pub fn what_if_close_batch(
    graph: &DiGraph,
    nodes: &[usize],
    closed_set: &[bool],
) -> WhatIfResult {
    let n = graph.len();
    if n == 0 || nodes.is_empty() {
        return WhatIfResult::empty();
    }

    // Create closed set with all specified nodes added
    let mut new_closed = closed_set.to_vec();
    new_closed.resize(n, false);
    for &node in nodes {
        if node < n {
            new_closed[node] = true;
        }
    }

    // Find all issues that become directly actionable
    let mut direct_unblocks = Vec::new();
    let mut seen = vec![false; n];

    for &node in nodes {
        if node >= n {
            continue;
        }
        for &successor in graph.successors_slice(node) {
            if seen[successor] || new_closed[successor] {
                continue;
            }
            seen[successor] = true;

            let was_blocked = !is_actionable(graph, successor, closed_set);
            let now_unblocked = is_actionable(graph, successor, &new_closed);

            if was_blocked && now_unblocked {
                direct_unblocks.push(successor);
            }
        }
    }

    let cascade_ids = count_cascade(graph, &direct_unblocks, &new_closed);
    let transitive_count = cascade_ids.len();
    let direct_count = direct_unblocks.len();

    WhatIfResult {
        direct_unblocks: direct_count,
        transitive_unblocks: transitive_count,
        unblocked_ids: direct_unblocks,
        cascade_ids,
        parallel_gain: direct_count.saturating_sub(1) as i32,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_what_if_empty() {
        let graph = DiGraph::new();
        let result = what_if_close(&graph, 0, &[]);
        assert_eq!(result.direct_unblocks, 0);
        assert_eq!(result.transitive_unblocks, 0);
    }

    #[test]
    fn test_what_if_single_node() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        let result = what_if_close(&graph, 0, &[false]);
        assert_eq!(result.direct_unblocks, 0);
        assert_eq!(result.transitive_unblocks, 0);
    }

    #[test]
    fn test_what_if_simple_chain() {
        // a -> b -> c
        // Closing a should unblock b, then c transitively
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let closed = vec![false, false, false];
        let result = what_if_close(&graph, a, &closed);

        assert_eq!(result.direct_unblocks, 1); // b
        assert_eq!(result.transitive_unblocks, 2); // b and c
        assert!(result.unblocked_ids.contains(&b));
        assert!(result.cascade_ids.contains(&b));
        assert!(result.cascade_ids.contains(&c));
    }

    #[test]
    fn test_what_if_diamond() {
        //     a
        //    / \
        //   b   c
        //    \ /
        //     d
        // Closing a should unblock b and c directly, then d transitively
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(b, d);
        graph.add_edge(c, d);

        let closed = vec![false, false, false, false];
        let result = what_if_close(&graph, a, &closed);

        assert_eq!(result.direct_unblocks, 2); // b and c
        assert_eq!(result.transitive_unblocks, 3); // b, c, and d
        assert_eq!(result.parallel_gain, 1); // 2 - 1 = 1
    }

    #[test]
    fn test_what_if_partial_close() {
        // a -> c, b -> c
        // If a is already closed, closing b unblocks c
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, c);
        graph.add_edge(b, c);

        // a is closed
        let closed = vec![true, false, false];
        let result = what_if_close(&graph, b, &closed);

        assert_eq!(result.direct_unblocks, 1); // c
        assert_eq!(result.transitive_unblocks, 1); // just c
    }

    #[test]
    fn test_what_if_multi_blocker_not_ready() {
        // a -> c, b -> c
        // Neither closed: closing a doesn't unblock c (b still blocks it)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, c);
        graph.add_edge(b, c);

        let closed = vec![false, false, false];
        let result = what_if_close(&graph, a, &closed);

        assert_eq!(result.direct_unblocks, 0); // c still blocked by b
        assert_eq!(result.transitive_unblocks, 0);
    }

    #[test]
    fn test_what_if_already_closed() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        graph.add_node("b");

        let closed = vec![true, false];
        let result = what_if_close(&graph, 0, &closed);

        assert_eq!(result.direct_unblocks, 0);
        assert_eq!(result.transitive_unblocks, 0);
    }

    #[test]
    fn test_what_if_wide_fanout() {
        // a -> b1, b2, b3, b4, b5
        // Closing a unblocks all 5
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        for i in 0..5 {
            let b = graph.add_node(&format!("b{}", i));
            graph.add_edge(a, b);
        }

        let closed = vec![false; 6];
        let result = what_if_close(&graph, a, &closed);

        assert_eq!(result.direct_unblocks, 5);
        assert_eq!(result.transitive_unblocks, 5);
        assert_eq!(result.parallel_gain, 4); // 5 - 1
    }

    #[test]
    fn test_what_if_deep_cascade() {
        // a -> b -> c -> d -> e -> f (chain of 6)
        let mut graph = DiGraph::new();
        let mut prev = graph.add_node("a");
        for i in 1..6 {
            let node = graph.add_node(&format!("n{}", i));
            graph.add_edge(prev, node);
            prev = node;
        }

        let closed = vec![false; 6];
        let result = what_if_close(&graph, 0, &closed);

        assert_eq!(result.direct_unblocks, 1);
        assert_eq!(result.transitive_unblocks, 5); // All 5 subsequent nodes
    }

    #[test]
    fn test_top_what_if() {
        //     a       e
        //    /|\      |
        //   b c d     f
        // a has more impact than e
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        let f = graph.add_node("f");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(a, d);
        graph.add_edge(e, f);

        let closed = vec![false; 6];
        let top = top_what_if(&graph, &closed, 10);

        assert!(!top.is_empty());
        // a should be first (unblocks 3)
        assert_eq!(top[0].node, a);
        assert_eq!(top[0].result.transitive_unblocks, 3);

        // e should be second (unblocks 1)
        assert!(top.len() >= 2);
        assert_eq!(top[1].node, e);
        assert_eq!(top[1].result.transitive_unblocks, 1);
    }

    #[test]
    fn test_top_what_if_limit() {
        let mut graph = DiGraph::new();
        for i in 0..10 {
            let a = graph.add_node(&format!("a{}", i));
            let b = graph.add_node(&format!("b{}", i));
            graph.add_edge(a, b);
        }

        let closed = vec![false; 20];
        let top = top_what_if(&graph, &closed, 3);

        assert_eq!(top.len(), 3);
    }

    #[test]
    fn test_what_if_batch_simple() {
        // a -> c, b -> c
        // Closing both a and b should unblock c
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, c);
        graph.add_edge(b, c);

        let closed = vec![false, false, false];
        let result = what_if_close_batch(&graph, &[a, b], &closed);

        assert_eq!(result.direct_unblocks, 1); // c
        assert_eq!(result.transitive_unblocks, 1);
    }

    #[test]
    fn test_what_if_batch_cascade() {
        //     a       b
        //     |       |
        //     c       d
        //      \     /
        //        e
        // Closing both a and b unblocks c, d, then e
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        graph.add_edge(a, c);
        graph.add_edge(b, d);
        graph.add_edge(c, e);
        graph.add_edge(d, e);

        let closed = vec![false; 5];
        let result = what_if_close_batch(&graph, &[a, b], &closed);

        assert_eq!(result.direct_unblocks, 2); // c and d
        assert_eq!(result.transitive_unblocks, 3); // c, d, e
    }

    #[test]
    fn test_all_what_if() {
        // a -> b, c (isolated)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let _c = graph.add_node("c");
        graph.add_edge(a, b);

        let closed = vec![false, false, false];
        let all = all_what_if(&graph, &closed, 10);

        // Only a has impact (c is isolated, b is blocked)
        assert_eq!(all.len(), 1);
        assert_eq!(all[0].node, a);
    }

    #[test]
    fn test_what_if_cycle_handling() {
        // a -> b -> c -> a (cycle)
        // Each node should only unblock its direct successor
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let closed = vec![false, false, false];

        // In a cycle, nothing is actionable, so closing any one
        // won't immediately unblock anything (all still blocked)
        let result = what_if_close(&graph, a, &closed);
        // b is unblocked by closing a, but c still needs b, and a needs c
        // So only b is directly unblocked
        assert!(result.direct_unblocks <= 1);
    }

    #[test]
    fn test_what_if_disconnected_components() {
        // Component 1: a -> b
        // Component 2: c -> d
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(c, d);

        let closed = vec![false; 4];

        // Closing a should only affect component 1
        let result_a = what_if_close(&graph, a, &closed);
        assert_eq!(result_a.transitive_unblocks, 1);
        assert!(result_a.cascade_ids.contains(&b));

        // Closing c should only affect component 2
        let result_c = what_if_close(&graph, c, &closed);
        assert_eq!(result_c.transitive_unblocks, 1);
        assert!(result_c.cascade_ids.contains(&d));
    }

    #[test]
    fn test_cascade_order() {
        // a -> b -> c -> d (deep chain)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, d);

        let closed = vec![false; 4];
        let result = what_if_close(&graph, a, &closed);

        // Cascade should include b, c, d in order
        assert_eq!(result.cascade_ids.len(), 3);
        assert_eq!(result.cascade_ids[0], b);
        assert_eq!(result.cascade_ids[1], c);
        assert_eq!(result.cascade_ids[2], d);
    }
}
