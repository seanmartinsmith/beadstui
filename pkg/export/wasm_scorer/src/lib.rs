use serde::{Deserialize, Serialize};
use wasm_bindgen::prelude::*;

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct Weights {
    pub text: f64,
    pub pagerank: f64,
    pub status: f64,
    pub impact: f64,
    pub priority: f64,
    pub recency: f64,
}

impl Default for Weights {
    fn default() -> Self {
        Self {
            text: 0.40,
            pagerank: 0.20,
            status: 0.15,
            impact: 0.10,
            priority: 0.10,
            recency: 0.05,
        }
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct IssueMetrics {
    pub id: String,
    pub text_score: f64,
    pub pagerank: f64,
    pub status: u8,
    pub priority: u8,
    pub blocker_count: u32,
    pub days_since_update: f64,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ScoredResult {
    pub id: String,
    pub score: f64,
    pub text_score: f64,
    pub components: [f64; 5],
}

#[wasm_bindgen]
pub struct HybridScorer {
    weights: Weights,
    max_blocker_count: u32,
}

#[wasm_bindgen]
impl HybridScorer {
    #[wasm_bindgen(constructor)]
    pub fn new(weights_json: &str) -> HybridScorer {
        let weights = serde_json::from_str(weights_json).unwrap_or_default();
        HybridScorer {
            weights,
            max_blocker_count: 0,
        }
    }

    #[wasm_bindgen]
    pub fn score_batch(&mut self, metrics_json: &str) -> String {
        let metrics: Vec<IssueMetrics> = match serde_json::from_str(metrics_json) {
            Ok(metrics) => metrics,
            Err(_) => return "[]".to_string(),
        };

        self.max_blocker_count = metrics
            .iter()
            .map(|m| m.blocker_count)
            .max()
            .unwrap_or(0);

        let mut results: Vec<ScoredResult> = metrics.iter().map(|m| self.score_single(m)).collect();

        results.sort_by(|a, b| {
            b.score
                .partial_cmp(&a.score)
                .unwrap_or(std::cmp::Ordering::Equal)
                .then_with(|| a.id.cmp(&b.id))
        });

        serde_json::to_string(&results).unwrap_or_else(|_| "[]".to_string())
    }
}

impl HybridScorer {
    fn score_single(&self, m: &IssueMetrics) -> ScoredResult {
        let status_score = normalize_status(m.status);
        let priority_score = normalize_priority(m.priority);
        let impact_score = normalize_impact(m.blocker_count, self.max_blocker_count);
        let recency_score = normalize_recency(m.days_since_update);
        let pagerank = normalize_pagerank(m.pagerank);

        let score = self.weights.text * safe_number(m.text_score)
            + self.weights.pagerank * pagerank
            + self.weights.status * status_score
            + self.weights.impact * impact_score
            + self.weights.priority * priority_score
            + self.weights.recency * recency_score;

        ScoredResult {
            id: m.id.clone(),
            score,
            text_score: safe_number(m.text_score),
            components: [pagerank, status_score, impact_score, priority_score, recency_score],
        }
    }
}

fn normalize_status(status: u8) -> f64 {
    match status {
        0 => 1.0,
        1 => 0.8,
        2 => 0.5,
        3 => 0.1,
        _ => 0.5,
    }
}

fn normalize_priority(priority: u8) -> f64 {
    match priority {
        0 => 1.0,
        1 => 0.8,
        2 => 0.6,
        3 => 0.4,
        4 => 0.2,
        _ => 0.5,
    }
}

fn normalize_impact(blocker_count: u32, max_blocker_count: u32) -> f64 {
    if max_blocker_count == 0 {
        return 0.5;
    }
    if blocker_count == 0 {
        return 0.0;
    }
    if blocker_count >= max_blocker_count {
        return 1.0;
    }
    blocker_count as f64 / max_blocker_count as f64
}

fn normalize_recency(days_since_update: f64) -> f64 {
    if !days_since_update.is_finite() || days_since_update < 0.0 {
        return 0.5;
    }
    (-days_since_update / 30.0).exp()
}

fn normalize_pagerank(pagerank: f64) -> f64 {
    if pagerank.is_finite() {
        pagerank
    } else {
        0.5
    }
}

fn safe_number(value: f64) -> f64 {
    if value.is_finite() {
        value
    } else {
        0.0
    }
}
