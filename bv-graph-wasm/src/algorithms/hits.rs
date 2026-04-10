//! HITS (Hyperlink-Induced Topic Search) algorithm.
//!
//! Computes hub and authority scores for nodes.
//! - Hubs: nodes that point to many good authorities
//! - Authorities: nodes pointed to by many good hubs
//!
//! Useful for identifying key "hub" issues that coordinate work
//! and "authority" issues that many others depend on.

use crate::graph::DiGraph;
use serde::Serialize;

/// Configuration for HITS computation.
pub struct HITSConfig {
    /// Convergence tolerance
    pub tolerance: f64,
    /// Maximum iterations
    pub max_iterations: u32,
}

impl Default for HITSConfig {
    fn default() -> Self {
        HITSConfig {
            tolerance: 1e-6,
            max_iterations: 100,
        }
    }
}

/// Result of HITS computation.
#[derive(Serialize)]
pub struct HITSResult {
    /// Hub scores (nodes that point to authorities)
    pub hubs: Vec<f64>,
    /// Authority scores (nodes pointed to by hubs)
    pub authorities: Vec<f64>,
    /// Number of iterations until convergence
    pub iterations: u32,
}

/// Compute HITS hub and authority scores.
///
/// HITS is an iterative algorithm:
/// 1. Authority(v) = sum of Hub(u) for all u → v
/// 2. Hub(u) = sum of Authority(v) for all u → v
/// 3. Normalize both vectors
/// 4. Repeat until convergence
///
/// # Arguments
/// * `graph` - The directed graph
/// * `config` - HITS configuration parameters
///
/// # Returns
/// HITSResult containing hub and authority scores
pub fn hits(graph: &DiGraph, config: &HITSConfig) -> HITSResult {
    let n = graph.len();
    if n == 0 {
        return HITSResult {
            hubs: Vec::new(),
            authorities: Vec::new(),
            iterations: 0,
        };
    }

    // Initialize with uniform scores
    let mut hubs = vec![1.0 / (n as f64); n];
    let mut auth = vec![1.0 / (n as f64); n];

    let mut iterations = 0;

    for iter in 0..config.max_iterations {
        iterations = iter + 1;

        let mut new_auth = vec![0.0; n];
        let mut new_hubs = vec![0.0; n];

        // Authority update: auth(v) = sum of hub(u) for all u → v
        for v in 0..n {
            for &u in graph.predecessors_slice(v) {
                new_auth[v] += hubs[u];
            }
        }

        // Hub update: hub(u) = sum of auth(v) for all u → v
        for u in 0..n {
            for &v in graph.successors_slice(u) {
                new_hubs[u] += new_auth[v];
            }
        }

        // Normalize both vectors (L2 norm for stability)
        normalize_l2(&mut new_auth);
        normalize_l2(&mut new_hubs);

        // Check convergence
        let auth_diff: f64 = auth
            .iter()
            .zip(new_auth.iter())
            .map(|(a, b)| (a - b).abs())
            .sum();
        let hub_diff: f64 = hubs
            .iter()
            .zip(new_hubs.iter())
            .map(|(a, b)| (a - b).abs())
            .sum();

        auth = new_auth;
        hubs = new_hubs;

        if auth_diff + hub_diff < config.tolerance {
            break;
        }
    }

    HITSResult {
        hubs,
        authorities: auth,
        iterations,
    }
}

/// Compute HITS with default parameters (tolerance=1e-6, max_iterations=100).
pub fn hits_default(graph: &DiGraph) -> HITSResult {
    hits(graph, &HITSConfig::default())
}

/// Normalize vector to unit L2 norm.
fn normalize_l2(vec: &mut [f64]) {
    let norm: f64 = vec.iter().map(|v| v * v).sum::<f64>().sqrt();
    if norm > 0.0 {
        for v in vec.iter_mut() {
            *v /= norm;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hits_empty() {
        let graph = DiGraph::new();
        let result = hits_default(&graph);
        assert!(result.hubs.is_empty());
        assert!(result.authorities.is_empty());
    }

    #[test]
    fn test_hits_single_node() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        let result = hits_default(&graph);
        assert_eq!(result.hubs.len(), 1);
        assert_eq!(result.authorities.len(), 1);
    }

    #[test]
    fn test_hits_chain() {
        // a -> b -> c
        // a is a hub (points to things)
        // c is an authority (pointed to)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let result = hits_default(&graph);

        // In a chain: a has highest hub (points to b), c has highest authority (final target)
        // b is in the middle, moderate both
        assert!(
            result.authorities[c] > result.authorities[a],
            "c should have higher authority"
        );
        assert!(result.hubs[a] > result.hubs[c], "a should have higher hub");
    }

    #[test]
    fn test_hits_star_hub() {
        // hub -> a, hub -> b, hub -> c
        // hub is a pure hub, a/b/c are pure authorities
        let mut graph = DiGraph::new();
        let hub = graph.add_node("hub");
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(hub, a);
        graph.add_edge(hub, b);
        graph.add_edge(hub, c);

        let result = hits_default(&graph);

        // Hub should have high hub score
        assert!(
            result.hubs[hub] > result.hubs[a],
            "hub should have higher hub score"
        );
        // a, b, c should have equal authority scores
        let auth_diff = (result.authorities[a] - result.authorities[b]).abs()
            + (result.authorities[b] - result.authorities[c]).abs();
        assert!(auth_diff < 0.01, "a, b, c should have equal authority");
    }

    #[test]
    fn test_hits_star_authority() {
        // a -> auth, b -> auth, c -> auth
        // auth is a pure authority, a/b/c are pure hubs
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let auth = graph.add_node("auth");
        graph.add_edge(a, auth);
        graph.add_edge(b, auth);
        graph.add_edge(c, auth);

        let result = hits_default(&graph);

        // auth should have highest authority score
        assert!(
            result.authorities[auth] > result.authorities[a],
            "auth should have higher authority"
        );
        // a, b, c should have equal hub scores
        let hub_diff = (result.hubs[a] - result.hubs[b]).abs()
            + (result.hubs[b] - result.hubs[c]).abs();
        assert!(hub_diff < 0.01, "a, b, c should have equal hub scores");
    }

    #[test]
    fn test_hits_bipartite() {
        // Hubs: h1, h2 -> Authorities: a1, a2
        // h1 -> a1, h1 -> a2
        // h2 -> a1, h2 -> a2
        let mut graph = DiGraph::new();
        let h1 = graph.add_node("h1");
        let h2 = graph.add_node("h2");
        let a1 = graph.add_node("a1");
        let a2 = graph.add_node("a2");
        graph.add_edge(h1, a1);
        graph.add_edge(h1, a2);
        graph.add_edge(h2, a1);
        graph.add_edge(h2, a2);

        let result = hits_default(&graph);

        // h1, h2 should have high hub scores
        // a1, a2 should have high authority scores
        assert!(result.hubs[h1] > result.authorities[h1]);
        assert!(result.hubs[h2] > result.authorities[h2]);
        assert!(result.authorities[a1] > result.hubs[a1]);
        assert!(result.authorities[a2] > result.hubs[a2]);
    }

    #[test]
    fn test_hits_cycle() {
        // a -> b -> c -> a
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let result = hits_default(&graph);

        // In a symmetric cycle, all nodes should have similar scores
        let hub_diff = (result.hubs[a] - result.hubs[b]).abs()
            + (result.hubs[b] - result.hubs[c]).abs();
        let auth_diff = (result.authorities[a] - result.authorities[b]).abs()
            + (result.authorities[b] - result.authorities[c]).abs();
        assert!(hub_diff < 0.01, "Cycle nodes should have similar hub scores");
        assert!(
            auth_diff < 0.01,
            "Cycle nodes should have similar authority scores"
        );
    }

    #[test]
    fn test_hits_scores_normalized() {
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let result = hits_default(&graph);

        // L2 norm should be 1
        let hub_norm: f64 = result.hubs.iter().map(|v| v * v).sum::<f64>().sqrt();
        let auth_norm: f64 = result
            .authorities
            .iter()
            .map(|v| v * v)
            .sum::<f64>()
            .sqrt();

        assert!(
            (hub_norm - 1.0).abs() < 0.001,
            "Hub scores should have unit L2 norm"
        );
        assert!(
            (auth_norm - 1.0).abs() < 0.001,
            "Authority scores should have unit L2 norm"
        );
    }

    #[test]
    fn test_hits_convergence() {
        // Create a non-trivial graph
        let mut graph = DiGraph::new();
        for i in 0..10 {
            graph.add_node(&format!("n{}", i));
        }
        // Add various edges
        for i in 0..5 {
            for j in 5..10 {
                graph.add_edge(i, j);
            }
        }

        let result = hits_default(&graph);

        // Should converge within max_iterations
        assert!(result.iterations <= 100);
        // Hubs (0-4) should have higher hub scores
        // Authorities (5-9) should have higher authority scores
        let avg_hub_hub: f64 = (0..5).map(|i| result.hubs[i]).sum::<f64>() / 5.0;
        let avg_auth_hub: f64 = (5..10).map(|i| result.hubs[i]).sum::<f64>() / 5.0;
        assert!(
            avg_hub_hub > avg_auth_hub,
            "Hub nodes should have higher hub scores"
        );
    }
}
