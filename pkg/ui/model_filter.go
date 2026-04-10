package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/bql"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
)

// getDiffStatus returns the diff status for an issue if time-travel mode is active
func (m Model) getDiffStatus(id string) DiffStatus {
	if !m.timeTravelMode {
		return DiffStatusNone
	}
	if m.newIssueIDs[id] {
		return DiffStatusNew
	}
	if m.closedIssueIDs[id] {
		return DiffStatusClosed
	}
	if m.modifiedIssueIDs[id] {
		return DiffStatusModified
	}
	return DiffStatusNone
}

// hasActiveFilters returns true if any filter is currently applied
// (status filter, label filter, recipe filter, or fuzzy search)
func (m *Model) hasActiveFilters() bool {
	// Check status/label/recipe filter
	if m.currentFilter != "all" {
		return true
	}
	// Check if fuzzy search filter is active
	if m.list.FilterState() == list.Filtering || m.list.FilterState() == list.FilterApplied {
		return true
	}
	return false
}

// clearAllFilters resets all filters to their default state
func (m *Model) clearAllFilters() {
	m.currentFilter = "all"
	m.setActiveRecipe(nil) // Clear any active recipe filter
	m.activeBQLExpr = nil  // Clear BQL state
	// Reset the fuzzy search filter by resetting the filter state
	m.list.ResetFilter()
	m.applyFilter()
}

func (m *Model) setActiveRecipe(r *recipe.Recipe) {
	m.activeRecipe = r
	if m.backgroundWorker != nil {
		m.backgroundWorker.SetRecipe(r)
	}
}

func (m *Model) matchesCurrentFilter(issue model.Issue) bool {
	// Workspace repo filter (nil = all repos)
	if m.workspaceMode && m.activeRepos != nil {
		repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
		if repoKey != "" && !m.activeRepos[repoKey] {
			return false
		}
	}

	switch m.currentFilter {
	case "all":
		return true
	case "open":
		return !isClosedLikeStatus(issue.Status)
	case "closed":
		return isClosedLikeStatus(issue.Status)
	case "ready":
		// Ready = Open/InProgress AND NO Open Blockers
		if isClosedLikeStatus(issue.Status) || issue.Status == model.StatusBlocked {
			return false
		}
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				return false
			}
		}
		return true
	default:
		if strings.HasPrefix(m.currentFilter, "label:") {
			label := strings.TrimPrefix(m.currentFilter, "label:")
			for _, l := range issue.Labels {
				if l == label {
					return true
				}
			}
		}
		return false
	}
}

func (m *Model) filteredIssuesForActiveView() []model.Issue {
	// BQL filter active? Use BQL executor (set-level operations: ORDER BY, EXPAND)
	if m.activeBQLExpr != nil && strings.HasPrefix(m.currentFilter, "bql:") {
		issues := m.workspacePrefilter(m.issues)
		opts := bql.ExecuteOpts{IssueMap: m.issueMap}
		return m.bqlEngine.Execute(m.activeBQLExpr, issues, opts)
	}

	filtered := make([]model.Issue, 0, len(m.issues))
	recipeFilterActive := m.activeRecipe != nil && strings.HasPrefix(m.currentFilter, "recipe:")
	if recipeFilterActive {
		for _, issue := range m.issues {
			if m.workspaceMode && m.activeRepos != nil {
				repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
				if repoKey != "" && !m.activeRepos[repoKey] {
					continue
				}
			}
			if issueMatchesRecipe(issue, m.issueMap, m.activeRecipe) {
				filtered = append(filtered, issue)
			}
		}
		sortIssuesByRecipe(filtered, m.analysis, m.activeRecipe)
		return filtered
	}
	for _, issue := range m.issues {
		if m.matchesCurrentFilter(issue) {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func (m *Model) refreshBoardAndGraphForCurrentFilter() {
	if !m.isBoardView && !m.isGraphView {
		return
	}

	filteredIssues := m.filteredIssuesForActiveView()
	recipeFilterActive := m.activeRecipe != nil && strings.HasPrefix(m.currentFilter, "recipe:")
	if m.isBoardView {
		useSnapshot := m.snapshot != nil && m.snapshot.BoardState != nil && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.snapshot.Issues)
		if useSnapshot {
			if recipeFilterActive {
				useSnapshot = m.snapshot.RecipeName == m.activeRecipe.Name && m.snapshot.RecipeHash == recipeFingerprint(m.activeRecipe)
			} else {
				useSnapshot = m.currentFilter == "all"
			}
		}
		if useSnapshot {
			m.board.SetSnapshot(m.snapshot)
		} else {
			m.board.SetIssues(filteredIssues)
		}
	}

	if m.isGraphView {
		useSnapshot := m.snapshot != nil && m.snapshot.GraphLayout != nil && len(filteredIssues) == len(m.snapshot.Issues)
		if useSnapshot {
			if recipeFilterActive {
				useSnapshot = m.snapshot.RecipeName == m.activeRecipe.Name && m.snapshot.RecipeHash == recipeFingerprint(m.activeRecipe)
			} else {
				useSnapshot = m.currentFilter == "all"
			}
		}
		if useSnapshot {
			m.graphView.SetSnapshot(m.snapshot)
		} else {
			filterIns := m.analysis.GenerateInsights(len(filteredIssues))
			m.graphView.SetIssues(filteredIssues, &filterIns)
		}
	}
}

func (m *Model) applyFilter() {
	var filteredItems []list.Item
	var filteredIssues []model.Issue

	for _, issue := range m.issues {
		if m.matchesCurrentFilter(issue) {
			// Use pre-computed graph scores (avoid redundant calculation)
			item := IssueItem{
				Issue:      issue,
				GraphScore: m.analysis.GetPageRankScore(issue.ID),
				Impact:     m.analysis.GetCriticalPathScore(issue.ID),
				DiffStatus: m.getDiffStatus(issue.ID),
				RepoPrefix: ExtractRepoPrefix(issue.ID),
			}
			// Add triage data (bv-151)
			item.TriageScore = m.triageScores[issue.ID]
			if reasons, exists := m.triageReasons[issue.ID]; exists {
				item.TriageReason = reasons.Primary
				item.TriageReasons = reasons.All
			}
			item.IsQuickWin = m.quickWinSet[issue.ID]
			item.IsBlocker = m.blockerSet[issue.ID]
			item.UnblocksCount = len(m.unblocksMap[issue.ID])
			filteredItems = append(filteredItems, item)
			filteredIssues = append(filteredIssues, issue)
		}
	}

	// Apply sort mode (bv-3ita)
	m.sortFilteredItems(filteredItems, filteredIssues)

	m.list.SetItems(filteredItems)
	m.updateSemanticIDs(filteredItems)
	if m.snapshot != nil && m.snapshot.BoardState != nil && m.currentFilter == "all" && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.snapshot.Issues) {
		m.board.SetSnapshot(m.snapshot)
	} else {
		m.board.SetIssues(filteredIssues)
	}
	if m.snapshot != nil && m.snapshot.GraphLayout != nil && m.currentFilter == "all" && len(filteredIssues) == len(m.snapshot.Issues) {
		m.graphView.SetSnapshot(m.snapshot)
	} else {
		// Generate insights for graph view (for metric rankings and sorting)
		filterIns := m.analysis.GenerateInsights(len(filteredIssues))
		m.graphView.SetIssues(filteredIssues, &filterIns)
	}

	// Keep selection in bounds
	if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
		m.list.Select(0)
	}
	m.updateViewportContent()
}

// refreshListItemsPhase2 updates visible items with Phase 2 scores and triage data
// without rebuilding the filtered set.
func (m *Model) refreshListItemsPhase2() {
	items := m.list.Items()
	if len(items) == 0 {
		return
	}

	selectedIdx := m.list.Index()
	for i := range items {
		item, ok := items[i].(IssueItem)
		if !ok {
			continue
		}
		issueID := item.Issue.ID
		if m.analysis != nil {
			item.GraphScore = m.analysis.GetPageRankScore(issueID)
			item.Impact = m.analysis.GetCriticalPathScore(issueID)
		}
		item.TriageScore = m.triageScores[issueID]
		if reasons, exists := m.triageReasons[issueID]; exists {
			item.TriageReason = reasons.Primary
			item.TriageReasons = reasons.All
		} else {
			item.TriageReason = ""
			item.TriageReasons = nil
		}
		item.IsQuickWin = m.quickWinSet[issueID]
		item.IsBlocker = m.blockerSet[issueID]
		item.UnblocksCount = len(m.unblocksMap[issueID])
		items[i] = item
	}

	m.list.SetItems(items)
	if selectedIdx >= 0 && selectedIdx < len(items) {
		m.list.Select(selectedIdx)
	}
	m.updateViewportContent()
}

// cycleSortMode cycles through available sort modes (bv-3ita)
func (m *Model) cycleSortMode() {
	m.sortMode = (m.sortMode + 1) % numSortModes
	m.applyFilter() // Re-apply filter with new sort
}

// sortFilteredItems sorts the filtered items based on current sortMode (bv-3ita)
func (m *Model) sortFilteredItems(items []list.Item, issues []model.Issue) {
	if len(items) == 0 {
		return
	}

	// Sort indices to keep items and issues in sync
	indices := make([]int, len(items))
	for i := range indices {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		iItem := items[indices[i]].(IssueItem)
		jItem := items[indices[j]].(IssueItem)

		switch m.sortMode {
		case SortCreatedAsc:
			// Oldest first
			return iItem.Issue.CreatedAt.Before(jItem.Issue.CreatedAt)
		case SortCreatedDesc:
			// Newest first
			return iItem.Issue.CreatedAt.After(jItem.Issue.CreatedAt)
		case SortPriority:
			// Priority ascending (P0 first)
			return iItem.Issue.Priority < jItem.Issue.Priority
		case SortUpdated:
			// Most recently updated first
			return iItem.Issue.UpdatedAt.After(jItem.Issue.UpdatedAt)
		default:
			// Default: Open first, then priority, then newest
			iClosed := isClosedLikeStatus(iItem.Issue.Status)
			jClosed := isClosedLikeStatus(jItem.Issue.Status)
			if iClosed != jClosed {
				return !iClosed
			}
			if iItem.Issue.Priority != jItem.Issue.Priority {
				return iItem.Issue.Priority < jItem.Issue.Priority
			}
			return iItem.Issue.CreatedAt.After(jItem.Issue.CreatedAt)
		}
	})

	// Reorder items and issues based on sorted indices
	sortedItems := make([]list.Item, len(items))
	sortedIssues := make([]model.Issue, len(issues))
	for newIdx, oldIdx := range indices {
		sortedItems[newIdx] = items[oldIdx]
		sortedIssues[newIdx] = issues[oldIdx]
	}
	copy(items, sortedItems)
	copy(issues, sortedIssues)
}

func matchesRecipeStatus(status model.Status, filter string) bool {
	normalized := strings.ToLower(strings.TrimSpace(filter))
	statusKey := strings.ToLower(string(status))
	switch normalized {
	case string(model.StatusClosed):
		return isClosedLikeStatus(status)
	case string(model.StatusTombstone):
		return status == model.StatusTombstone
	case string(model.StatusOpen):
		return status == model.StatusOpen
	case string(model.StatusInProgress):
		return status == model.StatusInProgress
	case string(model.StatusBlocked):
		return status == model.StatusBlocked
	default:
		return statusKey == normalized
	}
}

// applyRecipe applies a recipe's filters and sort to the current view
func (m *Model) applyRecipe(r *recipe.Recipe) {
	if r == nil {
		return
	}

	var filteredItems []list.Item
	var filteredIssues []model.Issue

	for _, issue := range m.issues {
		include := true

		// Workspace repo filter (nil = all repos)
		if m.workspaceMode && m.activeRepos != nil {
			repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
			if repoKey != "" && !m.activeRepos[repoKey] {
				include = false
			}
		}

		// Apply status filter
		if len(r.Filters.Status) > 0 {
			statusMatch := false
			for _, s := range r.Filters.Status {
				if matchesRecipeStatus(issue.Status, s) {
					statusMatch = true
					break
				}
			}
			include = include && statusMatch
		}

		// Apply priority filter
		if include && len(r.Filters.Priority) > 0 {
			prioMatch := false
			for _, p := range r.Filters.Priority {
				if issue.Priority == p {
					prioMatch = true
					break
				}
			}
			include = include && prioMatch
		}

		// Apply tags filter (must have ALL specified tags)
		if include && len(r.Filters.Tags) > 0 {
			labelSet := make(map[string]bool)
			for _, l := range issue.Labels {
				labelSet[l] = true
			}
			for _, required := range r.Filters.Tags {
				if !labelSet[required] {
					include = false
					break
				}
			}
		}

		// Apply actionable filter
		if include && r.Filters.Actionable != nil && *r.Filters.Actionable {
			// Check if issue is blocked
			isBlocked := false
			for _, dep := range issue.Dependencies {
				if dep == nil || !dep.Type.IsBlocking() {
					continue
				}
				if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
					isBlocked = true
					break
				}
			}
			include = !isBlocked
		}

		if include {
			item := IssueItem{
				Issue:      issue,
				GraphScore: m.analysis.GetPageRankScore(issue.ID),
				Impact:     m.analysis.GetCriticalPathScore(issue.ID),
				DiffStatus: m.getDiffStatus(issue.ID),
				RepoPrefix: ExtractRepoPrefix(issue.ID),
			}
			// Add triage data (bv-151)
			item.TriageScore = m.triageScores[issue.ID]
			if reasons, exists := m.triageReasons[issue.ID]; exists {
				item.TriageReason = reasons.Primary
				item.TriageReasons = reasons.All
			}
			item.IsQuickWin = m.quickWinSet[issue.ID]
			item.IsBlocker = m.blockerSet[issue.ID]
			item.UnblocksCount = len(m.unblocksMap[issue.ID])
			filteredItems = append(filteredItems, item)
			filteredIssues = append(filteredIssues, issue)
		}
	}

	// Apply sort
	field := r.Sort.Field
	descending := r.Sort.Direction == "desc"
	if field != "" {
		compare := func(a, b model.Issue) int {
			switch field {
			case "priority":
				switch {
				case a.Priority < b.Priority:
					return -1
				case a.Priority > b.Priority:
					return 1
				default:
					return 0
				}
			case "created", "created_at":
				switch {
				case a.CreatedAt.Before(b.CreatedAt):
					return -1
				case a.CreatedAt.After(b.CreatedAt):
					return 1
				default:
					return 0
				}
			case "updated", "updated_at":
				switch {
				case a.UpdatedAt.Before(b.UpdatedAt):
					return -1
				case a.UpdatedAt.After(b.UpdatedAt):
					return 1
				default:
					return 0
				}
			case "impact":
				if m.analysis == nil {
					switch {
					case a.Priority < b.Priority:
						return -1
					case a.Priority > b.Priority:
						return 1
					default:
						return 0
					}
				}
				aScore := m.analysis.GetCriticalPathScore(a.ID)
				bScore := m.analysis.GetCriticalPathScore(b.ID)
				switch {
				case aScore < bScore:
					return -1
				case aScore > bScore:
					return 1
				default:
					return 0
				}
			case "pagerank":
				if m.analysis == nil {
					switch {
					case a.Priority < b.Priority:
						return -1
					case a.Priority > b.Priority:
						return 1
					default:
						return 0
					}
				}
				aScore := m.analysis.GetPageRankScore(a.ID)
				bScore := m.analysis.GetPageRankScore(b.ID)
				switch {
				case aScore < bScore:
					return -1
				case aScore > bScore:
					return 1
				default:
					return 0
				}
			default:
				switch {
				case a.Priority < b.Priority:
					return -1
				case a.Priority > b.Priority:
					return 1
				default:
					return 0
				}
			}
		}

		sort.Slice(filteredItems, func(i, j int) bool {
			iItem := filteredItems[i].(IssueItem)
			jItem := filteredItems[j].(IssueItem)

			cmp := compare(iItem.Issue, jItem.Issue)
			if cmp == 0 {
				return iItem.Issue.ID < jItem.Issue.ID
			}
			if descending {
				return cmp > 0
			}
			return cmp < 0
		})

		// Re-sort issues list too
		sort.Slice(filteredIssues, func(i, j int) bool {
			ii := filteredIssues[i]
			jj := filteredIssues[j]

			cmp := compare(ii, jj)
			if cmp == 0 {
				return ii.ID < jj.ID
			}
			if descending {
				return cmp > 0
			}
			return cmp < 0
		})
	}

	m.list.SetItems(filteredItems)
	m.updateSemanticIDs(filteredItems)
	m.board.SetIssues(filteredIssues)
	// Generate insights for graph view (for metric rankings and sorting)
	recipeIns := m.analysis.GenerateInsights(len(filteredIssues))
	m.graphView.SetIssues(filteredIssues, &recipeIns)

	// Update filter indicator
	m.currentFilter = "recipe:" + r.Name

	// Keep selection in bounds
	if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
		m.list.Select(0)
	}
	m.updateViewportContent()
}

// recalculateSplitPaneSizes updates list and viewport dimensions after pane ratio changes
func (m *Model) recalculateSplitPaneSizes() {
	if !m.isSplitView {
		return
	}

	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	// Calculate dimensions accounting for 2 panels with borders(2)+padding(2) = 4 overhead each
	availWidth := m.width - 8
	if availWidth < 10 {
		availWidth = 10
	}

	listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
	detailInnerWidth := availWidth - listInnerWidth

	listHeight := bodyHeight - 4
	if listHeight < 3 {
		listHeight = 3
	}

	m.list.SetSize(listInnerWidth, listHeight)
	m.viewport = viewport.New(viewport.WithWidth(detailInnerWidth), viewport.WithHeight(bodyHeight-2))
	m.renderer.SetWidthWithTheme(detailInnerWidth, m.theme)
	m.updateViewportContent()
}

func (m *Model) updateViewportContent() {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		m.viewport.SetContent("No issues selected")
		return
	}

	// Safe type assertion
	issueItem, ok := selectedItem.(IssueItem)
	if !ok {
		m.viewport.SetContent("Error: invalid item type")
		return
	}
	item := issueItem.Issue

	var sb strings.Builder

	if m.updateAvailable {
		sb.WriteString(fmt.Sprintf("⭐ **Update Available:** [%s](%s)\n\n", m.updateTag, m.updateURL))
	}

	// Title Block
	sb.WriteString(fmt.Sprintf("# %s %s\n", GetTypeIconMD(string(item.IssueType)), item.Title))

	// Meta Table
	sb.WriteString("| ID | Status | Priority | Assignee | Created |\n|---|---|---|---|---|\n")
	sb.WriteString(fmt.Sprintf("| **%s** | **%s** | %s | @%s | %s |\n\n",
		item.ID,
		strings.ToUpper(string(item.Status)),
		fmt.Sprintf("%s P%d", GetPriorityIcon(item.Priority), item.Priority),
		item.Assignee,
		FormatTimeAbs(item.CreatedAt),
	))

	// Labels (bv-f103 fix: display labels in detail view)
	if len(item.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("**Labels:** %s\n\n", strings.Join(item.Labels, ", ")))
	}

	// Triage Insights (bv-151)
	if issueItem.TriageScore > 0 || issueItem.TriageReason != "" || issueItem.UnblocksCount > 0 || issueItem.IsQuickWin || issueItem.IsBlocker {
		sb.WriteString("### 🎯 Triage Insights\n")

		// Score with visual indicator
		scoreIcon := "🔵"
		if issueItem.TriageScore >= 0.7 {
			scoreIcon = "🔴"
		} else if issueItem.TriageScore >= 0.4 {
			scoreIcon = "🟠"
		}
		sb.WriteString(fmt.Sprintf("- **Triage Score:** %s %.2f/1.00\n", scoreIcon, issueItem.TriageScore))

		// Special flags
		if issueItem.IsQuickWin {
			sb.WriteString("- **⭐ Quick Win** — Low effort, high impact opportunity\n")
		}
		if issueItem.IsBlocker {
			sb.WriteString("- **🔴 Critical Blocker** — Completing this unblocks significant downstream work\n")
		}

		// Unblocks count
		if issueItem.UnblocksCount > 0 {
			sb.WriteString(fmt.Sprintf("- **🔓 Unblocks:** %d downstream items when completed\n", issueItem.UnblocksCount))
		}

		// Primary reason
		if issueItem.TriageReason != "" {
			sb.WriteString(fmt.Sprintf("- **Primary Reason:** %s\n", issueItem.TriageReason))
		}

		// All reasons (if multiple)
		if len(issueItem.TriageReasons) > 1 {
			sb.WriteString("- **All Reasons:**\n")
			for _, reason := range issueItem.TriageReasons {
				sb.WriteString(fmt.Sprintf("  - %s\n", reason))
			}
		}

		sb.WriteString("\n")
	}

	// Search Scores (hybrid mode)
	if m.semanticSearchEnabled && m.semanticHybridEnabled && issueItem.SearchScoreSet && m.list.FilterState() != list.Unfiltered {
		sb.WriteString("### 🔎 Search Scores\n")
		sb.WriteString(fmt.Sprintf("- **Hybrid Score:** %.3f\n", issueItem.SearchScore))
		sb.WriteString(fmt.Sprintf("- **Text Score:** %.3f\n", issueItem.SearchTextScore))
		if len(issueItem.SearchComponents) > 0 {
			sb.WriteString("- **Components:**\n")
			order := []string{"pagerank", "status", "impact", "priority", "recency"}
			for _, key := range order {
				if val, ok := issueItem.SearchComponents[key]; ok {
					sb.WriteString(fmt.Sprintf("  - %s: %.3f\n", key, val))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Graph Analysis (using thread-safe accessors)
	pr := m.analysis.GetPageRankScore(item.ID)
	bt := m.analysis.GetBetweennessScore(item.ID)
	imp := m.analysis.GetCriticalPathScore(item.ID)
	ev := m.analysis.GetEigenvectorScore(item.ID)
	hub := m.analysis.GetHubScore(item.ID)
	auth := m.analysis.GetAuthorityScore(item.ID)

	sb.WriteString("### Graph Analysis\n")
	sb.WriteString(fmt.Sprintf("- **Impact Depth**: %.0f (downstream chain length)\n", imp))
	sb.WriteString(fmt.Sprintf("- **Centrality**: PR %.4f • BW %.4f • EV %.4f\n", pr, bt, ev))
	sb.WriteString(fmt.Sprintf("- **Flow Role**: Hub %.4f • Authority %.4f\n\n", hub, auth))

	// Description
	if item.Description != "" {
		sb.WriteString("### Description\n")
		sb.WriteString(item.Description + "\n\n")
	}

	// Design Notes
	if item.Design != "" {
		sb.WriteString("### Design Notes\n")
		sb.WriteString(item.Design + "\n\n")
	}

	// Acceptance Criteria
	if item.AcceptanceCriteria != "" {
		sb.WriteString("### Acceptance Criteria\n")
		sb.WriteString(item.AcceptanceCriteria + "\n\n")
	}

	// Notes
	if item.Notes != "" {
		sb.WriteString("### Notes\n")
		sb.WriteString(item.Notes + "\n\n")
	}

	// Resolution (for closed issues with close_reason)
	if item.Status.IsClosed() && item.CloseReason != nil && *item.CloseReason != "" {
		sb.WriteString("### Resolution\n")
		sb.WriteString(*item.CloseReason + "\n\n")
	}

	// Dependency Graph (Tree)
	if len(item.Dependencies) > 0 {
		rootNode := BuildDependencyTree(item.ID, m.issueMap, 3) // Max depth 3
		treeStr := RenderDependencyTree(rootNode)
		sb.WriteString("```\n" + treeStr + "```\n\n")
	}

	// Comments
	if len(item.Comments) > 0 {
		sb.WriteString(fmt.Sprintf("### Comments (%d)\n", len(item.Comments)))
		for _, comment := range item.Comments {
			sb.WriteString(fmt.Sprintf("> **%s** (%s)\n> \n> %s\n\n",
				comment.Author,
				FormatTimeRel(comment.CreatedAt),
				strings.ReplaceAll(comment.Text, "\n", "\n> ")))
		}
	}

	// History Section (if data is loaded)
	if m.historyView.HasReport() {
		historyMD := m.renderBeadHistoryMD(item.ID)
		if historyMD != "" {
			sb.WriteString(historyMD)
		}
	}

	rendered, err := m.renderer.Render(sb.String())
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("Error rendering markdown: %v", err))
	} else {
		m.viewport.SetContent(rendered)
	}
}

// renderBeadHistoryMD generates markdown for a bead's history
func (m *Model) renderBeadHistoryMD(beadID string) string {
	hist := m.historyView.GetHistoryForBead(beadID)
	if hist == nil || len(hist.Commits) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### 📜 History\n\n")

	// Lifecycle milestones from events
	if len(hist.Events) > 0 {
		sb.WriteString("**Lifecycle:**\n")
		for _, event := range hist.Events {
			icon := getEventIcon(event.EventType)
			sb.WriteString(fmt.Sprintf("- %s **%s** %s by %s\n",
				icon,
				event.EventType,
				event.Timestamp.Format("Jan 02 15:04"),
				event.Author,
			))
		}
		sb.WriteString("\n")
	}

	// Correlated commits
	sb.WriteString(fmt.Sprintf("**Related Commits (%d):**\n", len(hist.Commits)))
	for i, commit := range hist.Commits {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more commits\n", len(hist.Commits)-5))
			break
		}

		// Confidence indicator
		confIcon := "🟢"
		if commit.Confidence < 0.5 {
			confIcon = "🟡"
		} else if commit.Confidence < 0.8 {
			confIcon = "🟠"
		}

		sb.WriteString(fmt.Sprintf("- %s **%.0f%%** `%s` %s\n",
			confIcon,
			commit.Confidence*100,
			commit.ShortSHA,
			truncateString(commit.Message, 40),
		))

		// Show files for high-confidence commits
		if commit.Confidence >= 0.8 && len(commit.Files) > 0 && len(commit.Files) <= 3 {
			for _, f := range commit.Files {
				sb.WriteString(fmt.Sprintf("  - `%s` (+%d, -%d)\n", f.Path, f.Insertions, f.Deletions))
			}
		}
	}

	sb.WriteString("\n*Press H for full history view*\n\n")
	return sb.String()
}

// getEventIcon returns an icon for bead event types
func getEventIcon(eventType correlation.EventType) string {
	switch eventType {
	case correlation.EventCreated:
		return "🟢"
	case correlation.EventClaimed:
		return "🔵"
	case correlation.EventClosed:
		return "⚫"
	case correlation.EventReopened:
		return "🟡"
	case correlation.EventModified:
		return "📝"
	default:
		return "•"
	}
}

// shortError extracts the tail of a nested error chain for display in the
// status bar (bv-9x36). Go errors like "connect: cannot reach Dolt server:
// dial tcp ...: connectex: ..." are too verbose for a single-line footer.
func shortError(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i != -1 {
		s = s[i+2:]
	}
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// truncateString truncates a string to maxLen runes with ellipsis.
// Uses rune-based counting to safely handle UTF-8 multi-byte characters.
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// workspacePrefilter removes issues not in the active repo set (workspace mode).
// Returns the input slice unchanged if not in workspace mode or all repos are active.
func (m *Model) workspacePrefilter(issues []model.Issue) []model.Issue {
	if !m.workspaceMode || m.activeRepos == nil {
		return issues
	}
	filtered := make([]model.Issue, 0, len(issues))
	for _, issue := range issues {
		repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
		if repoKey == "" || m.activeRepos[repoKey] {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// applyBQL applies a parsed BQL query using the dedicated BQL execution path.
// This bypasses matchesCurrentFilter() because BQL has set-level operations
// (ORDER BY, EXPAND) that can't work per-issue.
func (m *Model) applyBQL(query *bql.Query, queryStr string) {
	issues := m.workspacePrefilter(m.issues)
	opts := bql.ExecuteOpts{IssueMap: m.issueMap}
	filtered := m.bqlEngine.Execute(query, issues, opts)

	var filteredItems []list.Item
	for _, issue := range filtered {
		item := IssueItem{
			Issue:      issue,
			GraphScore: m.analysis.GetPageRankScore(issue.ID),
			Impact:     m.analysis.GetCriticalPathScore(issue.ID),
			DiffStatus: m.getDiffStatus(issue.ID),
			RepoPrefix: ExtractRepoPrefix(issue.ID),
		}
		item.TriageScore = m.triageScores[issue.ID]
		if reasons, exists := m.triageReasons[issue.ID]; exists {
			item.TriageReason = reasons.Primary
			item.TriageReasons = reasons.All
		}
		item.IsQuickWin = m.quickWinSet[issue.ID]
		item.IsBlocker = m.blockerSet[issue.ID]
		item.UnblocksCount = len(m.unblocksMap[issue.ID])
		filteredItems = append(filteredItems, item)
	}

	m.list.SetItems(filteredItems)
	m.updateSemanticIDs(filteredItems)
	m.currentFilter = "bql:" + queryStr

	m.board.SetIssues(filtered)
	filterIns := m.analysis.GenerateInsights(len(filtered))
	m.graphView.SetIssues(filtered, &filterIns)

	if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
		m.list.Select(0)
	}
	m.updateViewportContent()
}

// GetTypeIconMD returns the emoji icon for an issue type (for markdown)
func GetTypeIconMD(t string) string {
	switch t {
	case "bug":
		return "🐛"
	case "feature":
		return "✨"
	case "task":
		return "📋"
	case "epic":
		return "🚀" // Use rocket instead of mountain - VS-16 variation selector causes width issues
	case "chore":
		return "🧹"
	default:
		return "•"
	}
}
