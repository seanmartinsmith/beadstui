//! PageRank algorithm implementation.
//!
//! Computes centrality scores based on incoming link structure.
//! High PageRank issues are central bottlenecks that many other issues depend on.

use crate::graph::DiGraph;

/// PageRank configuration parameters.
pub struct PageRankConfig {
    /// Damping factor (typically 0.85)
    pub damping: f64,
    /// Convergence tolerance
    pub tolerance: f64,
    /// Maximum iterations
    pub max_iterations: u32,
}

impl Default for PageRankConfig {
    fn default() -> Self {
        PageRankConfig {
            damping: 0.85,
            tolerance: 1e-6,
            max_iterations: 100,
        }
    }
}

/// Compute PageRank scores for all nodes.
///
/// Algorithm: Power iteration method
/// PR(v) = (1-d)/n + d * Σ PR(u)/out_degree(u) for all u → v
///
/// Returns vector of scores in node index order.
pub fn pagerank(graph: &DiGraph, config: &PageRankConfig) -> Vec<f64> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    let d = config.damping;
    let base = (1.0 - d) / n as f64;

    // Initialize with uniform distribution
    let mut scores = vec![1.0 / n as f64; n];
    let mut new_scores = vec![0.0; n];

    // Pre-compute out-degrees
    let out_degrees: Vec<usize> = (0..n).map(|i| graph.out_degree(i)).collect();

    for _ in 0..config.max_iterations {
        // Reset new scores to base value
        for s in &mut new_scores {
            *s = base;
        }

        // Handle dangling nodes (no outgoing edges)
        // Their rank "leaks" and is distributed uniformly
        let dangling_sum: f64 = (0..n)
            .filter(|&i| out_degrees[i] == 0)
            .map(|i| scores[i])
            .sum();
        let dangling_contrib = d * dangling_sum / n as f64;

        // Add dangling contribution to all nodes
        for s in &mut new_scores {
            *s += dangling_contrib;
        }

        // Accumulate contributions from predecessors
        for v in 0..n {
            for &u in graph.predecessors_slice(v) {
                if out_degrees[u] > 0 {
                    new_scores[v] += d * scores[u] / out_degrees[u] as f64;
                }
            }
        }

        // Check convergence
        let diff: f64 = scores
            .iter()
            .zip(new_scores.iter())
            .map(|(a, b)| (a - b).abs())
            .sum();

        std::mem::swap(&mut scores, &mut new_scores);

        if diff < config.tolerance {
            break;
        }
    }

    scores
}

/// Compute PageRank with default parameters (damping=0.85, tolerance=1e-6).
pub fn pagerank_default(graph: &DiGraph) -> Vec<f64> {
    pagerank(graph, &PageRankConfig::default())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pagerank_empty() {
        let graph = DiGraph::new();
        let scores = pagerank(&graph, &PageRankConfig::default());
        assert!(scores.is_empty());
    }

    #[test]
    fn test_pagerank_single_node() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        let scores = pagerank(&graph, &PageRankConfig::default());
        assert_eq!(scores.len(), 1);
        // Single node should have all probability mass
        assert!((scores[0] - 1.0).abs() < 1e-6);
    }

    #[test]
    fn test_pagerank_chain() {
        // a -> b -> c
        // c should have highest rank (sink)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let scores = pagerank(&graph, &PageRankConfig::default());
        assert!(scores[c] > scores[b], "c should have higher rank than b");
        assert!(scores[b] > scores[a], "b should have higher rank than a");
    }

    #[test]
    fn test_pagerank_cycle() {
        // a -> b -> c -> a
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let scores = pagerank(&graph, &PageRankConfig::default());
        // Scores should be roughly equal in a cycle
        let diff = (scores[a] - scores[b]).abs() + (scores[b] - scores[c]).abs();
        assert!(diff < 0.01, "Scores in cycle should be roughly equal");
    }

    #[test]
    fn test_pagerank_star() {
        // a -> b, a -> c, a -> d (a is central)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(a, d);

        let scores = pagerank(&graph, &PageRankConfig::default());
        // b, c, d are sinks with equal incoming from a
        // They should have roughly equal scores
        let sink_diff = (scores[b] - scores[c]).abs() + (scores[c] - scores[d]).abs();
        assert!(sink_diff < 0.01, "Sink nodes should have equal scores");

        // Sinks should have higher rank than the source (due to damping)
        assert!(
            scores[b] > scores[a],
            "Sink should have higher rank than source"
        );
    }

    #[test]
    fn test_pagerank_two_chains() {
        // Chain 1: a -> b -> c
        // Chain 2: d -> e
        // Longer chain should have higher max score
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        let e = graph.add_node("e");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(d, e);

        let scores = pagerank(&graph, &PageRankConfig::default());
        // c is the deepest sink, should have highest score
        assert!(scores[c] > scores[e], "Deeper sink should have higher rank");
    }

    #[test]
    fn test_pagerank_sum_to_one() {
        // PageRank scores should sum to approximately 1.0
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let scores = pagerank(&graph, &PageRankConfig::default());
        let sum: f64 = scores.iter().sum();
        assert!(
            (sum - 1.0).abs() < 0.001,
            "PageRank should sum to 1.0, got {}",
            sum
        );
    }

    #[test]
    fn test_pagerank_dangling_nodes() {
        // a -> b, c is isolated (dangling)
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        // c has no edges - it's a dangling node

        let scores = pagerank(&graph, &PageRankConfig::default());
        assert!(scores[c] > 0.0, "Dangling node should have positive score");

        let sum: f64 = scores.iter().sum();
        assert!(
            (sum - 1.0).abs() < 0.001,
            "PageRank should sum to 1.0 with dangling nodes, got {}",
            sum
        );
    }

    #[test]
    fn test_pagerank_convergence() {
        // Large enough graph to test convergence
        let mut graph = DiGraph::new();
        for i in 0..20 {
            graph.add_node(&format!("node{}", i));
        }
        // Create some edges
        for i in 0..19 {
            graph.add_edge(i, i + 1);
        }
        // Add some cross edges
        graph.add_edge(0, 10);
        graph.add_edge(5, 15);
        graph.add_edge(10, 0);

        let scores = pagerank(&graph, &PageRankConfig::default());
        assert_eq!(scores.len(), 20);

        let sum: f64 = scores.iter().sum();
        assert!(
            (sum - 1.0).abs() < 0.001,
            "PageRank should sum to 1.0 in larger graph"
        );
    }

    #[test]
    fn test_pagerank_diamond() {
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

        let scores = pagerank(&graph, &PageRankConfig::default());
        // d should have highest score (most incoming paths)
        assert!(scores[d] > scores[b], "d should have higher rank than b");
        assert!(scores[d] > scores[c], "d should have higher rank than c");
        // b and c should have equal scores (symmetric)
        assert!(
            (scores[b] - scores[c]).abs() < 0.001,
            "b and c should have equal scores"
        );
    }
}
