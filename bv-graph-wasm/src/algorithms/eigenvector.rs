//! Eigenvector Centrality algorithm implementation.
//!
//! Measures node importance based on connections to other important nodes.
//! Uses the principal eigenvector of the adjacency matrix via power iteration.

use crate::graph::DiGraph;

/// Eigenvector centrality configuration.
pub struct EigenvectorConfig {
    /// Number of power iterations
    pub iterations: u32,
    /// Convergence tolerance (stop early if converged)
    pub tolerance: f64,
}

impl Default for EigenvectorConfig {
    fn default() -> Self {
        EigenvectorConfig {
            iterations: 50,
            tolerance: 1e-6,
        }
    }
}

/// Compute eigenvector centrality using power iteration.
///
/// Uses incoming edges: nodes pointed TO by important nodes are important.
/// This measures prestige - who receives attention from important sources.
///
/// Returns vector of scores in node index order, normalized to unit length.
pub fn eigenvector(graph: &DiGraph, config: &EigenvectorConfig) -> Vec<f64> {
    let n = graph.len();
    if n == 0 {
        return Vec::new();
    }

    // Initialize with uniform distribution
    let init_val = 1.0 / (n as f64).sqrt();
    let mut vec = vec![init_val; n];
    let mut work = vec![0.0; n];

    for _ in 0..config.iterations {
        // Reset work vector
        for w in &mut work {
            *w = 0.0;
        }

        // Multiply: work = A^T * vec (sum of predecessor scores)
        // A node's score = sum of scores of nodes that point to it
        for v in 0..n {
            for &u in graph.predecessors_slice(v) {
                work[v] += vec[u];
            }
        }

        // Normalize to unit length (L2 norm)
        let norm: f64 = work.iter().map(|x| x * x).sum::<f64>().sqrt();
        if norm < 1e-10 {
            // Graph has no edges or is disconnected - return uniform
            let uniform = 1.0 / (n as f64).sqrt();
            return vec![uniform; n];
        }

        for w in &mut work {
            *w /= norm;
        }

        // Check convergence
        let diff: f64 = vec
            .iter()
            .zip(work.iter())
            .map(|(a, b)| (a - b).abs())
            .sum();

        std::mem::swap(&mut vec, &mut work);

        if diff < config.tolerance {
            break;
        }
    }

    vec
}

/// Compute eigenvector centrality with default parameters (50 iterations).
pub fn eigenvector_default(graph: &DiGraph) -> Vec<f64> {
    eigenvector(graph, &EigenvectorConfig::default())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_eigenvector_empty() {
        let graph = DiGraph::new();
        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        assert!(scores.is_empty());
    }

    #[test]
    fn test_eigenvector_single_node() {
        let mut graph = DiGraph::new();
        graph.add_node("a");
        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        assert_eq!(scores.len(), 1);
        // Single node with no edges - uniform distribution
        assert!((scores[0] - 1.0).abs() < 1e-6);
    }

    #[test]
    fn test_eigenvector_chain() {
        // a -> b -> c (DAG - no cycles)
        // In a DAG without cycles, eigenvector centrality converges to uniform
        // because there's no feedback loop to establish dominance
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        // In a DAG, scores converge to uniform distribution
        let diff = (scores[a] - scores[b]).abs() + (scores[b] - scores[c]).abs();
        assert!(
            diff < 0.01,
            "DAG chain should converge to near-uniform: {:?}",
            scores
        );
    }

    #[test]
    fn test_eigenvector_cycle() {
        // a -> b -> c -> a
        // In a cycle, all nodes should have equal centrality
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        let diff = (scores[a] - scores[b]).abs() + (scores[b] - scores[c]).abs();
        assert!(
            diff < 0.001,
            "Scores in cycle should be equal: {:?}",
            scores
        );
    }

    #[test]
    fn test_eigenvector_star() {
        // a -> b, a -> c, a -> d (a is the source, b/c/d are sinks)
        // b, c, d receive from a, so they should have equal scores
        // a receives from nobody
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(a, d);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        // b, c, d should have equal scores
        let sink_diff = (scores[b] - scores[c]).abs() + (scores[c] - scores[d]).abs();
        assert!(sink_diff < 0.001, "Sink nodes should have equal scores");
    }

    #[test]
    fn test_eigenvector_hub_with_cycle() {
        // b -> a, c -> a, d -> a, a -> b (cycle through b)
        // a receives from many sources, should have high score
        // Adding cycle ensures non-trivial eigenvector
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(b, a);
        graph.add_edge(c, a);
        graph.add_edge(d, a);
        graph.add_edge(a, b); // Creates cycle a->b->a

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        // a receives from b, c, d - should have higher score than isolated c, d
        assert!(
            scores[a] > scores[c],
            "Hub with cycle should have higher score than isolated node: {} vs {}",
            scores[a],
            scores[c]
        );
    }

    #[test]
    fn test_eigenvector_diamond() {
        //     a
        //    / \
        //   b   c
        //    \ /
        //     d
        // DAG - eigenvector converges to uniform in pure DAGs
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(a, c);
        graph.add_edge(b, d);
        graph.add_edge(c, d);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        // b and c should be equal (symmetric structure)
        assert!(
            (scores[b] - scores[c]).abs() < 0.001,
            "b and c should have equal scores due to symmetry"
        );
        // All nodes present
        assert_eq!(scores.len(), 4);
    }

    #[test]
    fn test_eigenvector_unit_length() {
        // Eigenvector should be normalized to unit length
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        graph.add_edge(a, b);
        graph.add_edge(b, c);
        graph.add_edge(c, a);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        let length: f64 = scores.iter().map(|x| x * x).sum::<f64>().sqrt();
        assert!(
            (length - 1.0).abs() < 0.001,
            "Eigenvector should have unit length, got {}",
            length
        );
    }

    #[test]
    fn test_eigenvector_disconnected() {
        // Two disconnected components
        let mut graph = DiGraph::new();
        let a = graph.add_node("a");
        let b = graph.add_node("b");
        let c = graph.add_node("c");
        let d = graph.add_node("d");
        graph.add_edge(a, b);
        graph.add_edge(c, d);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        assert_eq!(scores.len(), 4);
        // All scores should be positive
        for s in &scores {
            assert!(*s >= 0.0, "All scores should be non-negative");
        }
    }

    #[test]
    fn test_eigenvector_convergence() {
        // Larger graph to test convergence
        let mut graph = DiGraph::new();
        for i in 0..20 {
            graph.add_node(&format!("node{}", i));
        }
        // Create chain with some cross edges
        for i in 0..19 {
            graph.add_edge(i, i + 1);
        }
        graph.add_edge(0, 10);
        graph.add_edge(5, 15);
        graph.add_edge(10, 0);

        let scores = eigenvector(&graph, &EigenvectorConfig::default());
        assert_eq!(scores.len(), 20);

        // Should be normalized
        let length: f64 = scores.iter().map(|x| x * x).sum::<f64>().sqrt();
        assert!(
            (length - 1.0).abs() < 0.001,
            "Should converge to unit length"
        );
    }
}
