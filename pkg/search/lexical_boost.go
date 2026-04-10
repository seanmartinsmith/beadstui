package search

import (
	"sort"
	"strings"
)

const shortQueryDocBoost = 0.35

// ShortQueryLexicalBoost returns a literal-match boost for short queries.
// It operates on the same document text used for indexing.
func ShortQueryLexicalBoost(query string, doc string) float64 {
	if !IsShortQuery(query) {
		return 0
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" || doc == "" {
		return 0
	}
	if strings.Contains(strings.ToLower(doc), needle) {
		return shortQueryDocBoost
	}
	return 0
}

// ApplyShortQueryLexicalBoost adds a literal-match boost to short-query results and re-sorts.
func ApplyShortQueryLexicalBoost(results []SearchResult, query string, docs map[string]string) []SearchResult {
	if len(results) == 0 || len(docs) == 0 {
		return results
	}
	if !IsShortQuery(query) {
		return results
	}

	boosted := false
	for i := range results {
		doc, ok := docs[results[i].IssueID]
		if !ok {
			continue
		}
		boost := ShortQueryLexicalBoost(query, doc)
		if boost == 0 {
			continue
		}
		results[i].Score += boost
		boosted = true
	}

	if boosted {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Score == results[j].Score {
				return results[i].IssueID < results[j].IssueID
			}
			return results[i].Score > results[j].Score
		})
	}

	return results
}
