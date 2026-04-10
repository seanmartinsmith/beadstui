//! High-performance graph algorithms for bv static viewer.
//!
//! This crate provides WASM-compiled graph algorithms that run in the browser,
//! enabling fast dependency analysis without server roundtrips.

use wasm_bindgen::prelude::*;

mod graph;
pub mod algorithms;
mod advanced;
mod whatif;
mod subgraph;
mod reachability;

pub use graph::DiGraph;

// Re-export key algorithm functions for testing
pub use algorithms::pagerank::{pagerank, pagerank_default, PageRankConfig};
pub use algorithms::betweenness::{betweenness, betweenness_approx};
pub use algorithms::eigenvector::{eigenvector, eigenvector_default, EigenvectorConfig};
pub use algorithms::critical_path::{critical_path_heights, critical_path_nodes, critical_path_length};
pub use algorithms::cycles::{has_cycles, tarjan_scc};
pub use algorithms::kcore::{kcore, degeneracy};
pub use algorithms::slack::{slack, total_float};
pub use algorithms::hits::{hits, hits_default, HITSConfig};

/// Initialize panic hook for better error messages in browser console.
#[wasm_bindgen(start)]
pub fn init() {
    #[cfg(feature = "console_error_panic_hook")]
    console_error_panic_hook::set_once();
}

/// Get the crate version.
#[wasm_bindgen]
pub fn version() -> String {
    env!("CARGO_PKG_VERSION").to_string()
}
