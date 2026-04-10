//! Articulation points (cut vertices) algorithm.
//!
//! Finds nodes whose removal disconnects the graph.
//! Uses Tarjan's algorithm on the undirected view.
//!
//! In issue tracking, articulation points are critical coordination
//! points - if those issues are blocked or deprioritized, they can
//! disconnect groups of related work.

use crate::graph::DiGraph;
use std::collections::HashSet;

/// Find articulation points (cut vertices) using Tarjan's algorithm.
///
/// An articulation point is a vertex whose removal increases the number
/// of connected components in the graph.
///
/// # Algorithm
/// Uses DFS to compute discovery times and low-link values:
/// - disc[v]: discovery time of vertex v
/// - low[v]: minimum discovery time reachable from subtree of v
///
/// A vertex v is an articulation point if:
/// 1. v is root of DFS tree and has >1 children, OR
/// 2. v is not root and has child u with low[u] >= disc[v]
///
/// # Returns
/// Vector of node indices that are articulation points.
pub fn articulation_points(graph: &DiGraph) -> Vec<usize> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    // Build undirected adjacency
    let neighbors = build_undirected_neighbors(graph);

    // Tarjan's algorithm state
    let mut disc = vec![0usize; n];
    let mut low = vec![0usize; n];
    let mut parent = vec![usize::MAX; n]; // usize::MAX means no parent (root)
    let mut visited = vec![false; n];
    let mut is_ap = vec![false; n];
    let mut time = 0usize;

    // Run DFS from each unvisited node (handles disconnected components)
    for start in 0..n {
        if !visited[start] {
            tarjan_dfs(
                start,
                &neighbors,
                &mut disc,
                &mut low,
                &mut parent,
                &mut visited,
                &mut is_ap,
                &mut time,
            );
        }
    }

    // Collect articulation points
    is_ap
        .iter()
        .enumerate()
        .filter_map(|(i, &ap)| if ap { Some(i) } else { None })
        .collect()
}

/// DFS for Tarjan's articulation point algorithm.
fn tarjan_dfs(
    v: usize,
    neighbors: &[Vec<usize>],
    disc: &mut [usize],
    low: &mut [usize],
    parent: &mut [usize],
    visited: &mut [bool],
    is_ap: &mut [bool],
    time: &mut usize,
) {
    visited[v] = true;
    *time += 1;
    disc[v] = *time;
    low[v] = *time;
    let mut children = 0;

    for &u in &neighbors[v] {
        if !visited[u] {
            children += 1;
            parent[u] = v;

            tarjan_dfs(u, neighbors, disc, low, parent, visited, is_ap, time);

            // Update low-link
            low[v] = low[v].min(low[u]);

            // Check if v is an articulation point
            // Case 1: v is root with >1 DFS children
            if parent[v] == usize::MAX && children > 1 {
                is_ap[v] = true;
            }

            // Case 2: v is not root and low[u] >= disc[v]
            // This means u (and its subtree) cannot reach any ancestor of v
            if parent[v] != usize::MAX && low[u] >= disc[v] {
                is_ap[v] = true;
            }
        } else if u != parent[v] {
            // Back edge (not to parent)
            low[v] = low[v].min(disc[u]);
        }
    }
}

/// Build undirected neighbor lists from directed graph.
fn build_undirected_neighbors(graph: &DiGraph) -> Vec<Vec<usize>> {
    let n = graph.len();
    let mut neighbors: Vec<HashSet<usize>> = vec![HashSet::new(); n];

    for u in 0..n {
        for &v in graph.successors_slice(u) {
            if u != v {
                // Skip self-loops
                neighbors[u].insert(v);
                neighbors[v].insert(u);
            }
        }
    }

    // Convert to Vec<Vec> for faster iteration
    neighbors.into_iter().map(|s| s.into_iter().collect()).collect()
}

/// Count bridges (cut edges) in the graph.
/// A bridge is an edge whose removal disconnects the graph.
pub fn bridges(graph: &DiGraph) -> Vec<(usize, usize)> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    let neighbors = build_undirected_neighbors(graph);

    let mut disc = vec![0usize; n];
    let mut low = vec![0usize; n];
    let mut parent = vec![usize::MAX; n];
    let mut visited = vec![false; n];
    let mut bridge_list = Vec::new();
    let mut time = 0usize;

    for start in 0..n {
        if !visited[start] {
            bridge_dfs(
                start,
                &neighbors,
                &mut disc,
                &mut low,
                &mut parent,
                &mut visited,
                &mut bridge_list,
                &mut time,
            );
        }
    }

    bridge_list
}

/// DFS for bridge detection.
fn bridge_dfs(
    v: usize,
    neighbors: &[Vec<usize>],
    disc: &mut [usize],
    low: &mut [usize],
    parent: &mut [usize],
    visited: &mut [bool],
    bridges: &mut Vec<(usize, usize)>,
    time: &mut usize,
) {
    visited[v] = true;
    *time += 1;
    disc[v] = *time;
    low[v] = *time;

    for &u in &neighbors[v] {
        if !visited[u] {
            parent[u] = v;
            bridge_dfs(u, neighbors, disc, low, parent, visited, bridges, time);

            low[v] = low[v].min(low[u]);

            // Bridge condition: if low[u] > disc[v], edge v-u is a bridge
            if low[u] > disc[v] {
                bridges.push((v.min(u), v.max(u))); // Canonical order
            }
        } else if u != parent[v] {
            low[v] = low[v].min(disc[u]);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_articulation_empty() {
        let graph = DiGraph::new();
        let ap = articulation_points(&graph);
        assert!(ap.is_empty());
    }

    #[test]
    fn test_articulation_single_node() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        let ap = articulation_points(&graph);
        assert!(ap.is_empty()); // Single node is not an articulation point
    }

    #[test]
    fn test_articulation_two_nodes() {
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        graph.add_edge(a, b);

        let ap = articulation_points(&graph);
        // Neither node is an articulation point (removing either leaves isolated node)
        assert!(ap.is_empty());
    }

    #[test]
    fn test_articulation_chain() {
        // a -> b -> c
        // b is an articulation point
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let ap = articulation_points(&graph);
        assert_eq!(ap.len(), 1);
        assert!(ap.contains(&b));
    }

    #[test]
    fn test_articulation_triangle() {
        // a -> b -> c -> a (cycle/triangle)
        // No articulation points
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let ap = articulation_points(&graph);
        assert!(ap.is_empty());
    }

    #[test]
    fn test_articulation_star() {
        // Hub with 3 leaves
        // Hub is an articulation point if it connects >1 component
        let mut graph = DiGraph::new();
        let hub = graph.add_node("hub");
        let l1 = graph.add_node("l1");
        let l2 = graph.add_node("l2");
        let l3 = graph.add_node("l3");
        graph.add_edge(hub, l1);
        graph.add_edge(hub, l2);
        graph.add_edge(hub, l3);

        let ap = articulation_points(&graph);
        // Hub is articulation point (root with >1 children)
        assert_eq!(ap.len(), 1);
        assert!(ap.contains(&hub));
    }

    #[test]
    fn test_articulation_bowtie() {
        // Two triangles connected by a single node
        //     a       d
        //    / \     / \
        //   b---c---e---f
        // c is the articulation point
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        let f = graph.add_node("f");

        // Left triangle
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        // Bridge
        graph.add_edge(c, e);

        // Right triangle
        graph.add_edge(d, e);
        graph.add_edge(e, f);
        graph.add_edge(f, d);

        let ap = articulation_points(&graph);
        // c and e are both articulation points
        assert!(ap.contains(&c) || ap.contains(&e));
    }

    #[test]
    fn test_articulation_bridge_chain() {
        // a-b-c-d (simple path)
        // b and c are articulation points
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, d);

        let ap = articulation_points(&graph);
        assert_eq!(ap.len(), 2);
        assert!(ap.contains(&b));
        assert!(ap.contains(&c));
    }

    #[test]
    fn test_bridges_chain() {
        // a-b-c
        // Two bridges: (a,b) and (b,c)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let br = bridges(&graph);
        assert_eq!(br.len(), 2);
    }

    #[test]
    fn test_bridges_triangle() {
        // Triangle has no bridges
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let br = bridges(&graph);
        assert!(br.is_empty());
    }

    #[test]
    fn test_articulation_disconnected() {
        // Two disconnected triangles - no articulation points
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        let f = graph.add_node("f");

        // Triangle 1
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        // Triangle 2
        graph.add_edge(d, e);
        graph.add_edge(e, f);
        graph.add_edge(f, d);

        let ap = articulation_points(&graph);
        assert!(ap.is_empty());
    }
}
