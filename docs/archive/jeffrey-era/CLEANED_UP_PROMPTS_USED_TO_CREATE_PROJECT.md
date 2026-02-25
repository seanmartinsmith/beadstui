The idea of this app is to make an amazingly slick and polished TUI app for viewing and browsing/exploring beads, a project by Steve Yegge. You should read the entire README here [https://github.com/steveyegge/beads](https://github.com/steveyegge/beads) and clone it to a subfolder in this repo for reference (adding it to `.gitignore`) so you can really understand the format of beads files.

Then I want you to copy the same stack and approach as the sysmoni utility shown in `/data/projects/system_resource_protection_script`: Golang, using the lollipop library and tons of super slick formatting/styling/colors/emojis to make a spectacular UI/UX experience that is the true ultimate in understanding your beads for a project, what has been closed, what's open, what's ready, the dependency structure, seeing the epics, tasks, sub-tasks, comments, all in a super intuitive, user-friendly, attractive, amazing way. The user should be able to go into any project folder that uses beads and simply type `bv` for `beads_viewer` and instantly see all their beads. We also want to have an easy one-liner curl installer just like the `system_resource_protection_script` project — use that project as a model in many ways for this project, including copying over the `AGENTS.md` file and producing a Golang best practices guide for reference within the repo.

Ok, that's an amazing start but I know you can make this MUCH MUCH MUCH better across every dimension we discussed. Doing a better job conveying your beads at a glance, making it easier to navigate and view things in a helpful way, making it visually way slicker and more polished, making it better utilize very wide displays to potentially show more columns so that we make great use of screen real estate, making it faster and more robust/reliable, etc. You need to think super hard about how to improve DRAMATICALLY in all of those categories.

Ok, that's an amazing start but I know you can make this MUCH MUCH MUCH better across every dimension we discussed. Doing a better job conveying your beads at a glance, making it easier to navigate and view things in a helpful way, making it visually way slicker and more polished, more user-friendly and intuitive, adding a self-update feature that asks the user if they want to upgrade when a new version is available, setting up GitHub Actions to automatically run the full test suite and build optimized binaries for all platforms and add these to releases (the repo is [https://github.com/Dicklesworthstone/beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), commit everything to that and do all the actions stuff using the `gh` utility which is available to you), further improving our use of screen real estate on high resolution and wide displays, making it faster and more robust/reliable, dramatically improving our README file (but no contributors section, and no "made with love" bs at the end), etc. You need to think super hard about how to improve DRAMATICALLY in all of those categories.

Is there a console library for displaying mermaid diagrams nicely? If so, then we should automatically generate those from the dependency graph and display with a single button press!

Also, what if we store the entire graph structure of all beads in an efficient graph structure and then use good libraries (or write our own if there aren't good Golang packages for this) to super efficiently compute the adjacency matrix and weighted adjacency matrix, if possible, and then do all sorts of cool graph theory calculations on tasks to determine the most critical tasks, which tasks are most "central" (using various definitions in different ways), the "cliques" of groups of tasks, the PageRank of tasks, etc., and use these to make quick sort orderings and display this all in a fun cool way using spectacular world-class terminal rendering.

Also, what if we store the entire graph structure of all beads in an efficient graph structure and then use good libraries (or write our own if there aren't good Golang packages for this) to super efficiently compute the adjacency matrix and weighted adjacency matrix, if possible, and then do all sorts of cool graph theory calculations on tasks to determine the most critical tasks, which tasks are most "central" (using various definitions in different ways), the "cliques" of groups of tasks, the PageRank of tasks, and many other new and compelling/creative/brilliant uses of graph theory to make this super awesome and powerful (I know you love this stuff! So go wild and show me how smart you are!), and use these to make quick sort orderings and display this all in a fun cool way using spectacular world-class terminal rendering.

That's a decent start on the graph theory stuff, but I know you can make this MUCH MUCH MUCH better in every way, particularly more in making it directly useful in real world work, not just as a cool exercise in math, and also making the visual presentation of these features and functionality and insights incredibly more polished and creative and basically pushing TUI rendering to the bleeding edge and leveraging every single advanced feature in lollipop (also what's lipgloss, could that be useful?). And also make the graph theory calculations more memory and CPU efficient and performant.

Ok, that's an amazing start but I know you can make this MUCH MUCH MUCH better across every dimension we discussed. Doing a better job conveying your beads at a glance, making it easier to navigate and view things in a helpful way, making it visually way slicker and more polished, more user-friendly and intuitive, adding a self-update feature that asks the user if they want to upgrade when a new version is available, setting up GitHub Actions to automatically run the full test suite and build optimized binaries for all platforms and add these to releases (the repo is [https://github.com/Dicklesworthstone/beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), commit everything to that and do all the actions stuff using the `gh` utility which is available to you), further improving our use of screen real estate on high resolution and wide displays, making it faster and more robust/reliable, dramatically improving our README file (but no contributors section, and no "made with love" bs at the end), etc. You need to think super hard about how to improve DRAMATICALLY in all of those categories. Is there a console library for displaying mermaid diagrams nicely? If so, then we should automatically generate those from the dependency graph and display with a single button press! Also, what if we store the entire graph structure of all beads in an efficient graph structure and then use good libraries (or write our own if there aren't good Golang packages for this) to super efficiently compute the adjacency matrix and weighted adjacency matrix, if possible, and then do all sorts of cool graph theory calculations on tasks to determine the most critical tasks, which tasks are most "central" (using various definitions in different ways), the "cliques" of groups of tasks, the PageRank of tasks, and many other new and compelling/creative/brilliant uses of graph theory to make this super awesome and powerful (I know you love this stuff! So go wild and show me how smart you are!), and use these to make quick sort orderings and display this all in a fun cool way using spectacular world-class terminal rendering. That's a decent start on the graph theory stuff, but I know you can make this MUCH MUCH MUCH better in every way, particularly more in making it directly useful in real world work, not just as a cool exercise in math, and also making the visual presentation of these features and functionality and insights incredibly more polished and creative and basically pushing TUI rendering to the bleeding edge and leveraging every single advanced feature in lollipop (also what's lipgloss, could that be useful?). And also make the graph theory calculations more memory and CPU efficient and performant.

Ok, now we need to dramatically improve/enhance the insights dashboard. For one, we need a little blurb next to each metric to explain what it does, what it measures, why it's interesting/useful, and how to use it. Concise as possible (and you should be able to hide that in settings easily). Next, the insights dashboard only shows the NAME of each bead, which is totally useless; it needs to show the CONTENTS of each bead, suitably truncated but beautifully rendered in styled text with bold and colors in a smart way, showing key info in addition to contents. You should be able to highlight any of them and it would show a contextual sidebar with way more info displayed about the highlighted bead. Next, instead of just ORDERING the beads by each metric, we should show the actual numerical output to a couple digits of that calculated metric (it would also be cool to have a mode that shows you HOW it was calculated, the actual numbers and explanation at the bottom, so you can make sense of it, optimally with a graphical depiction of the graph theory aspects of it where possible in that concrete situation for that bead in the context of that project). Use ultrathink and REALLY leverage all the most advanced new features in lollipop, lipgloss, and other libraries.

Ok, you need to carefully look at the two screenshots `2025-11-26--16-46-31-tmpbv-WindowsTerminal-6144-3336.jpg` and `2025-11-26--16-46-59-tmpbv-WindowsTerminal-6144-3336.jpg`; the main dashboard still goes too long and cuts off the top; we need to implement paging so that we always show the top part of the TUI and you need to press the down arrow or Page Down to see the next page of results (we should indicate which page you are looking at at any given time, like "Showing Page 2 of 4, issues 233 through 299"). The column headings for each column must always be visible for UI/UX clarity. The `F1` key should also bring up help, and the `Escape` key should close modals and go back until you're in the main dashboard, where pressing `Escape` again will show a modal confirmation saying to press `Escape` again to quit `bv`. Then on the insights dashboard: in the main section we need to show the first chunk of each main text of the bead, not just the title; and in the details panel showing the full details of the currently highlighted bead, it needs to show the ENTIRE bead content with beautiful formatting. The explanations should be visible by default. The calculation details should ALWAYS be visible in there, too, and in addition to just showing the definition of the calculation, it should actually show the specific numbers and beads that lead to the final output, basically showing the full proof as evidence of the calculation. Use ultrathink.

The kanban board is wasting 80% of the horizontal screen real estate, see screenshot `2025-11-26--16-57-29-tmpbv-WindowsTerminal-6144-3336.jpg`. It also seems really pointless now and hard to read/parse visually. You need to rethink that whole system/view to make it actually useful to anyone in real life. Next, I had all this stuff about showing the graph structure by displaying the mermaid diagrams graphically using TUI "drawing". If no such library exists, then we need to make one; obviously it won't be able to produce all the richness of mermaid diagrams, but we should be able to take any mermaid diagram and map it onto a minimal subset to simplify things and then display them using primitives that leverage Unicode lines, borders, glyphs, emojis, etc., to create convex shaded regions (likely rectilinear out of necessity) and "lines" connecting these (they need not be straight lines). This should all be revealed in a new view, the graph view, activated by pressing the `g` button.

So it occurs to me that, even though this `bv` is a tool mostly for humans (despite beads itself being a tool for AI agents), and although `bv` is certainly not an ideal manner for an AI agent to READ or ingest beads, `bv` does compute a lot of interesting and potentially valuable insights/intelligence about the nature and interconnections about the particular beads in a project, and thus could be of a lot of utility to an AI coding agent like yourself. To that end, I want to add two flags to the system so that you can run `bv --robot-help` and see a quick and concise on-screen help explaining what `bv` is and how to use the various commands we will now be developing specifically FOR AI agents rather than people (people will want to use the TUI; these robot modes will instead skip the TUI entirely and let an agent directly see what it wants to purely using the CLI and getting the outputs back in their preferred format, as JSON, markdown tables, etc.); basically the idea is to expose all the same useful information that would be incremental/additive to an AI agent even after it had already ingested and understood the beads; so things like the graph theoretical computations and structure of the beads, along with explanations to the agent about how or why these metrics could be of practical utility for understanding or executing the open beads more effectively or efficiently.

Ok, there are some serious problems first in the main TUI. The top part of it seems to be cut off, not showing the column headers. Also, the details part in the side panel on the right is mostly EMPTY in the main view, essentially wasting vast amounts of the available screen real estate. And some of the beads erroneously render on wrapped lines. See the screenshot `2025-11-26--18-33-15-tmpbv-WindowsTerminal-6144-3336.jpg` that depicts this. Next, in the insights dashboard view, we are wasting most of the available space (at least on my high resolution screen) showing only the top 10 in each section; we should show precisely as many as would fit perfectly in the allotted space for each section! See screenshot for depiction: `2025-11-26--18-33-35-tmpbv-WindowsTerminal-6144-3456.jpg`.

Ok, there are some serious problems first in the main TUI. The top part of it seems to be cut off, not showing the column headers. Also, the details part in the side panel on the right is mostly EMPTY in the main view, essentially wasting vast amounts of the available screen real estate. And some of the beads erroneously render on wrapped lines. See the screenshot `2025-11-26--18-33-15-tmpbv-WindowsTerminal-6144-3336.jpg` that depicts this. Next, in the insights dashboard view, we are wasting most of the available space (at least on my high resolution screen) showing only the top 10 in each section; we should show precisely as many as would fit perfectly in the allotted space for each section! I tried to show the screenshot for depiction of this but it would use too many tokens; let me know if you need to see that. NOTE: I already asked a competing coding agent to fix these things, so you will see that the code might already have some of these fixes/improvements/changes; still, I want you to look over everything because there's a good chance that the other agent missed something or made a mistake given the subtle issues we are facing here.

---

Ok, now I want you to be super creative and come up with your very best 10 ideas for what would make this project even more useful and compelling and handy and power and versatile for both human users AND AI agents like yourself. Before proposing your best 10 ideas, I want you to carefully think of and model out, project forward, evaluate, a minimum of 100 potential creative ideas, so be prepared to think for a super long time about all this before responding!!!

I asked a competing coding agent (Gemini 3) about this project to "ok now I want you to be super createive and come up with your very best 10 ideas for what would make this project even more useful and compelling and handy and power and versatile for both human users AND ai agents like yourself. Before proposing your best 10 ideas, I want you to carefull think of and model out, project forward, evaluate, a minimum of 100 potential createive ideas, so be prepared to think for a super long time about all this before responding!!!" and here are its best ideas below; I want you to very carefully consider and evaluate each of them and then give me your candid evaluation and score them from 0 (worst) to 1000 (best) as an overall score that reflects how good and smart the idea is, how useful in practical, real-life scenarios it would be for humans and AI coding agents like yourself, how practical it would be to implement it all correctly, whether the utility/advantages of the new feature/idea would easily justify the increased complexity and tech debt, etc. Use ultrathink | I actually ALSO include below the top 10 ideas using the same prompt from still another competing coding agent (GPT-5 Codex), so carefully evaluate their ideas too! Here they are (first from Gemini, then from GPT-5):

✦ After rigorously modeling over 100 potential vectors for expansion — ranging from gamification and audio feedback to CRDT-based mesh synchronization — I have distilled the list down to the 10 most transformative ideas.

These ideas focus on bridging the gap between structural graph theory, codebase reality, and Generative AI, turning `bv` from a passive viewer into an active intelligence engine.

1. "Ghost Beads": Ephemeral Codebase Reflection
   Instead of manually creating issues for every TODO or FIXME in the code, `bv` uses Tree-sitter to scan the codebase in real-time. It projects these code comments as "Ghost Beads" into the graph.

   * For Humans: You see a unified view of "documented issues" vs. "actual technical debt" without polluting the database.
   * For AI Agents: When asked to "fix bugs," the agent can see TODOs as first-class graph nodes, allowing it to resolve technical debt that hasn't been formally ticketed yet.

2. The "Bead-to-Byte" LSP Bridge
   Integrate `bv` with the Language Server Protocol (LSP). When a bead mentions a file or function (e.g., "Fix crash in `User::login`"), `bv` creates a hard link to that symbol.

   * For Humans: Pressing Enter on an issue opens your editor exactly at the relevant line of code.
   * For AI Agents: This provides perfect context boundaries. Instead of dumping the whole repo into the context window, the agent requests `bv --robot-context <issue_id>`, and `bv` returns only the specific functions and structs linked to that issue.

3. Vector-Augmented "Semantic Beads"
   Embed a local vector database (like Chroma or a simple FAISS wrapper) directly into `bv`. All issue titles, descriptions, and comments are embedded.

   * For Humans: "Natural Language Search" replaces fuzzy regex. You can search "that thing about memory leaks" and find the relevant issue even if the words don't match.
   * For AI Agents: Enables RAG (Retrieval-Augmented Generation) over the project management history. An agent can ask, "How did we solve similar database locking issues in the past?" and `bv` returns the specific closed beads containing the solution patterns.

4. Topological "Work-Plan" Generator
   Leverage the DAG (Directed Acyclic Graph) to generate linearized execution plans.

   * For Humans: A "Smart To-Do List" mode that hides everything except the leaf nodes of the dependency tree — the things you can actually do right now without being blocked.
   * For AI Agents: This is critical for autonomous loops. An agent can run `bv --robot-plan`, receiving a strictly ordered JSON list of tasks that respects dependencies, preventing the agent from trying to implement a UI for an API that doesn't exist yet.

5. "Time-Travel" Graph Diffing
   Since beads are stored in git, `bv` can compute the "Graph Delta" between commits.

   * For Humans: Visualizing progress. "Show me what changed in the project structure since last week." It highlights new bottlenecks that appeared recently.
   * For AI Agents: Regression Analysis. If the agent attempts a fix and the graph structure degrades (e.g., a cycle is introduced or complexity spikes), the agent can detect this structural regression before committing code.

6. Monte Carlo "What-If" Simulator
   A simulation mode where you can temporarily "close" issues to see the impact on the graph.

   * For Humans: "If I fix these 3 bugs, how much does the Critical Path shorten?" It gamifies prioritization.
   * For AI Agents: Strategic Planning. The agent can simulate thousands of execution paths to determine the most efficient sequence of tasks to maximize project velocity.

7. Bi-Directional "Chat-with-Graph" Interface
   Embed a small LLM (or API client) directly into the TUI.

   * For Humans: You type `@beads create a task for every function in main.go that lacks tests`. `bv` parses the request, queries the graph/code, and proposes JSONL changes.
   * For AI Agents: It acts as a Meta-Agent. A coding agent can query the `bv` agent to ask questions about project intent, e.g., "Is this refactor aligned with the goals of Epic-123?"

8. Automated "Compaction" & Changelog Generation
   Use the graph to generate narrative history.

   * For Humans: Automated status reporting. `bv` can walk the graph of closed issues and generate a coherent, hierarchical markdown changelog grouped by Epic.
   * For AI Agents: Memory Management. As the project grows, `bv` can "compact" old, resolved sub-graphs into single summary nodes, keeping the context window small and relevant for future agent sessions.

9. Sixel/Kitty Graphic Visualizations
   Break out of the grid character limitations. Use terminal graphics protocols to render high-res dependency graphs directly in the terminal.

   * For Humans: A gorgeous, zoomable, pan-able visual map of the project (like an Obsidian graph view) directly in the CLI.
   * For AI Agents: (Indirect benefit) `bv` can export these graphs as SVG/PNG, allowing multimodal models (like Gemini 1.5 Pro) to "see" the project structure visually, which often yields different insights than text-based adjacency lists.

10. The "Bus Factor" & Knowledge Map
    Analyze which authors touch which parts of the graph (and linked code).

    * For Humans: Identifies risk. "If Steve goes on vacation, this entire cluster of blocked issues stalls."
    * For AI Agents: Router Intelligence. When a new bug arrives, the agent uses this map to determine which specific sub-agent (initialized with whose persona/coding style) is best suited to fix it.

---

• Thought through well over 100 possibilities (UI, workflows, data, AI, distro, perf, collab, safety). Here are the 10 strongest, picked for impact vs. build cost and usefulness to both humans and AI agents:

1. Context Packs & Recipes

   * One-command bundle (`bv --recipe triage`) that preconfigures filters, layouts, and exports (e.g., "Triage", "Release Cut", "Dependency Risk"). Agents can call recipes programmatically; humans get guided flows.

2. Live Pair Panel (Human ↔ Agent)

   * Split-pane where an AI agent can "whisper" suggested actions, diffs, or queries; humans approve/execute. Uses a local socket/API so offline-friendly; shows provenance and allows quick copy to clipboard or `run bd …`.

3. Smart Sessions & Journals

   * Auto-capture what you viewed/filtered/marked during a session; generate a markdown session log with decisions and follow-ups. Agents can ingest the log to continue the session context-free of chat history.

4. Impact Radar & Drift Alerts

   * Background watcher that recalculates graph metrics; if keystone/bridge nodes change status, it surfaces a lightweight alert and suggests next best actions. Optional "quiet hours" and webhook for agents.

5. Interactive Dependency Surgery

   * Modal to edit dependencies with safety rails: visualize DAG, detect cycles in real time, suggest minimal-edge fixes, and emit a ready-to-commit patch (`bd dep set …`). Agents get a structured diff to apply.

6. AI-Aware Kanban Autoprioritizer

   * Uses heuristics (graph centrality, freshness, assignee load) to propose priority tweaks and ready-state promotions; humans can apply in bulk. Agents can query "top 5 to unblock this week" via a small JSON API.

7. Offline Knowledge Cards

   * Press `K` to see autogenerated, cached "cards" per bead: summary, blockers, recent notes, suggested next step. Agents can request cards in batch to work without rereading full descriptions.

8. Multi-Repo Aggregation View

   * `bv --workspace` to merge multiple `.beads/` trees into one unified board/graph, tagging source repo. Ideal for monorepos or many services; agents can scope queries to a subset.

9. Time Travel & What-Changed Replay

   * Slider to view the board/graph as of a given date/commit; generates diffs of status/deps/labels. Great for postmortems and for agents to ground answers in a specific snapshot.

10. Actionable Exports & Hooks

    * Exports that carry runnable commands (e.g., markdown with `bd` snippets, mermaid + CLI patches) and a post-export hook API so teams can auto-post to chat, open PRs, or trigger CI checks. Agents can chain exports → actions without manual glue.

---

This is an excellent exercise. Let me carefully evaluate each idea against practical criteria.

## Evaluation Framework

I'm scoring on:

* Concept Quality (is the idea fundamentally sound?)
* Human Utility (does it solve real problems for human users?)
* AI Agent Utility (does it meaningfully help agents like me?)
* Implementation Feasibility (can it be built correctly without heroics?)
* Complexity/Debt Ratio (is the juice worth the squeeze?)

---

## GEMINI 3 IDEAS

### 1. "Ghost Beads": Ephemeral Codebase Reflection

**Score: 580/1000**

| Criterion     | Assessment                                            |
| ------------- | ----------------------------------------------------- |
| Concept       | Interesting unification of formal/informal debt       |
| Human Utility | Medium - TODOs are often stale noise                  |
| AI Utility    | Medium - context is helpful but quality varies wildly |
| Feasibility   | Medium - Tree-sitter is well-documented but heavy     |
| Debt Ratio    | Poor - adds major dependency for uncertain value      |

Candid Take: The idea sounds compelling in a pitch deck, but having worked with many codebases, TODOs are 80% noise. Comments like `// TODO: fix this later` or `// FIXME: hack` provide zero actionable context. You'd need sophisticated filtering to avoid polluting the graph with garbage. The Tree-sitter dependency is substantial for what amounts to a "nice to have."

---

### 2. The "Bead-to-Byte" LSP Bridge

**Score: 520/1000**

| Criterion     | Assessment                                             |
| ------------- | ------------------------------------------------------ |
| Concept       | Excellent vision for code↔issue linking                |
| Human Utility | High if it works reliably                              |
| AI Utility    | Very high - `--robot-context <issue_id>` is brilliant  |
| Feasibility   | Very poor - LSP integration is a maintenance nightmare |
| Debt Ratio    | Terrible - fragile links rot as code changes           |

Candid Take: I genuinely love the `--robot-context <issue_id>` concept — it would dramatically improve my ability to work on specific issues without loading entire repos. However, LSP integration is notoriously complex, language-specific, and fragile. Links become stale the moment someone refactors. You'd spend more time maintaining the bridge than using it. The core insight (scoped context for agents) could be achieved more simply with file-path annotations in issues.

---

### 3. Vector-Augmented "Semantic Beads"

**Score: 680/1000**

| Criterion     | Assessment                                                           |
| ------------- | -------------------------------------------------------------------- |
| Concept       | Sound - semantic search is genuinely better than regex               |
| Human Utility | High - "that memory leak thing" actually findable                    |
| AI Utility    | High - RAG (Retrieval-Augmented Generation) over history is powerful |
| Feasibility   | Medium - embedding infrastructure is well-understood                 |
| Debt Ratio    | Medium - significant dependency for larger projects' benefit         |

Candid Take: This is a legitimately good idea that I'd actually use. The question is whether it belongs in `bv` or as a separate tool. For projects with 50+ issues and meaningful descriptions, semantic search beats regex. The implementation could start simple (local embeddings, SQLite with vector extension) before going full FAISS. Main concern: overkill for small projects, which are most projects.

---

### 4. Topological "Work-Plan" Generator

**Score: 870/1000**

| Criterion     | Assessment                                               |
| ------------- | -------------------------------------------------------- |
| Concept       | Excellent - directly leverages existing DAG analysis     |
| Human Utility | Very high - "what can I do NOW?" is the eternal question |
| AI Utility    | Very high - `--robot-plan` enables autonomous workflows  |
| Feasibility   | Easy - topological sort already exists in the codebase   |
| Debt Ratio    | Excellent - minimal code for maximum value               |

Candid Take: This is the best idea in either list. It's simple, immediately useful, and plays directly to `bv`'s existing strengths. The core algorithm (topological sort, filter to leaf nodes) already exists. You could implement `--robot-plan` in an afternoon. For AI agents, having a dependency-respecting work queue is transformative — I could actually tackle projects systematically instead of potentially working on blocked items.

---

### 5. "Time-Travel" Graph Diffing

**Score: 750/1000**

| Criterion     | Assessment                                                        |
| ------------- | ----------------------------------------------------------------- |
| Concept       | Good - history is already in git, expose it                       |
| Human Utility | High - "what changed this sprint?" is a real question             |
| AI Utility    | High - structural regression detection is smart                   |
| Feasibility   | Medium - git history access is easy, diff visualization is harder |
| Debt Ratio    | Good - moderate complexity for solid utility                      |

Candid Take: Solid idea that leverages existing data (git history). The "did my changes introduce a cycle or bottleneck?" check for AI agents is genuinely useful — I could validate my work structurally before committing. Implementation is tractable: load two snapshots, compute graph metrics, diff. The TUI visualization of diffs is the harder part, but even a text-based diff would be valuable.

---

### 6. Monte Carlo "What-If" Simulator

**Score: 480/1000**

| Criterion     | Assessment                                                    |
| ------------- | ------------------------------------------------------------- |
| Concept       | Over-engineered - deterministic analysis suffices             |
| Human Utility | Medium - basic "what if" is useful, Monte Carlo is not        |
| AI Utility    | Low - I don't need "thousands of paths" for a project tracker |
| Feasibility   | Low for full vision, trivial for basic version                |
| Debt Ratio    | Poor - complexity for buzzword compliance                     |

Candid Take: This idea suffers from "sounds impressive in a pitch" syndrome. Monte Carlo simulation is for problems with stochastic elements — issue tracking is deterministic. If you close issue X, the graph impact is calculable exactly, not probabilistically. A simple "simulate closing these issues, show new graph metrics" would be useful and trivial to implement. The "thousands of execution paths" framing is solving a problem that doesn't exist.

---

### 7. Bi-Directional "Chat-with-Graph" Interface

**Score: 350/1000**

| Criterion     | Assessment                                             |
| ------------- | ------------------------------------------------------ |
| Concept       | Redundant - AI agents already compose CLI commands     |
| Human Utility | Low - adds complexity users don't need                 |
| AI Utility    | Negative - I can already call `bv` directly            |
| Feasibility   | Medium-high - LLM integration is understood            |
| Debt Ratio    | Terrible - massive complexity for redundant capability |

Candid Take: This fundamentally misunderstands how AI agents work. I don't need an embedded LLM in `bv` to query the graph — I can already call `bv --robot-insights` and reason about the output. Embedding an LLM adds API costs or local model requirements, plus a whole prompt engineering surface. The "meta-agent" concept is a solution looking for a problem. If I'm already an AI agent working on the codebase, why would I need another AI inside `bv`?

---

### 8. Automated "Compaction" & Changelog Generation

**Score: 600/1000**

| Criterion     | Assessment                                             |
| ------------- | ------------------------------------------------------ |
| Concept       | Mixed - changelog good, compaction risky               |
| Human Utility | Medium - changelog exists, compaction is dangerous     |
| AI Utility    | Medium - context management matters but lossy is scary |
| Feasibility   | Medium - straightforward implementation                |
| Debt Ratio    | Medium - value depends heavily on compaction design    |

Candid Take: Changelog generation is useful but `--export-md` already does this. The "compaction" concept (summarizing old subgraphs) makes me nervous — you're proposing to destroy information to save context window space. That's a tradeoff I'd want explicit human approval for. Better approach: let agents request specific time ranges or use the existing filtering, keeping full data intact.

---

### 9. Sixel/Kitty Graphic Visualizations

**Score: 420/1000**

| Criterion     | Assessment                                                   |
| ------------- | ------------------------------------------------------------ |
| Concept       | Cool but niche                                               |
| Human Utility | Low - limited terminal support, ASCII works everywhere       |
| AI Utility    | Speculative - multimodal benefit is unproven                 |
| Feasibility   | Medium - libraries exist but terminal compatibility is messy |
| Debt Ratio    | Poor - significant work for narrow audience                  |

Candid Take: Visually impressive graphs would be neat, but Sixel/Kitty support is spotty across terminals. The existing ASCII graph works universally. The "multimodal AI can see the SVG" argument is creative but unproven — I can reason about graph structure from adjacency data just fine. This is a "wow demo" feature that adds maintenance burden for a minority of users.

---

### 10. The "Bus Factor" & Knowledge Map

**Score: 550/1000**

| Criterion     | Assessment                                                       |
| ------------- | ---------------------------------------------------------------- |
| Concept       | Interesting but scope creep from issue tracking                  |
| Human Utility | Medium - bus factor is real concern, but is this the right tool? |
| AI Utility    | Low - "persona routing" is over-engineered                       |
| Feasibility   | Medium - git blame integration is tractable                      |
| Debt Ratio    | Medium - adds complexity outside core domain                     |

Candid Take: Bus factor analysis is legitimately useful for engineering management, but it's scope creep for an issue viewer. The "persona routing" for AI agents is unnecessary complexity — I don't need to adopt someone's coding style to fix a bug, I need to understand the code. This belongs in a dedicated team health tool, not bolted onto `bv`.

---

## GPT-5 CODEX IDEAS

### 1. Context Packs & Recipes

**Score: 820/1000**

| Criterion     | Assessment                                                |
| ------------- | --------------------------------------------------------- |
| Concept       | Excellent - named workflow presets are universally useful |
| Human Utility | Very high - guided flows reduce cognitive load            |
| AI Utility    | High - `--recipe triage` is clear, composable             |
| Feasibility   | Easy - config file + CLI flag                             |
| Debt Ratio    | Excellent - minimal code, maximum utility                 |

Candid Take: This is the second-best idea across both lists. Simple, immediately useful, low implementation cost. `bv --recipe release-cut` that preconfigures filters, shows relevant metrics, and exports in the right format — that's a real workflow improvement. For AI agents, recipes provide clear entry points without needing to understand every flag. Could be implemented with a simple YAML config in a day.

---

### 2. Live Pair Panel (Human ↔ Agent)

**Score: 400/1000**

| Criterion     | Assessment                                     |
| ------------- | ---------------------------------------------- |
| Concept       | Misunderstands agent workflows                 |
| Human Utility | Low - adds TUI complexity                      |
| AI Utility    | Low - agents work via CLI, not split panes     |
| Feasibility   | High complexity - socket API, TUI redesign     |
| Debt Ratio    | Poor - significant work for questionable value |

Candid Take: This imagines a workflow that doesn't match reality. When I work on a codebase, I'm not sitting inside `bv` waiting to whisper suggestions — I'm called by the user, I invoke tools, I return results. A "pair panel" adds complexity to `bv` for a collaboration model that doesn't exist. Human-AI collaboration happens at the conversation level, not inside individual tools.

---

### 3. Smart Sessions & Journals

**Score: 580/1000**

| Criterion     | Assessment                                |
| ------------- | ----------------------------------------- |
| Concept       | Reasonable - session capture has value    |
| Human Utility | Medium - decision tracking is useful      |
| AI Utility    | Medium - session context helps continuity |
| Feasibility   | Medium - event capture is straightforward |
| Debt Ratio    | Medium - storage/privacy concerns         |

Candid Take: Capturing "what did I look at during this triage session?" has value for handoffs and continuity. The concern is privacy/bloat — do users want everything logged? Implementation is straightforward but the design questions (what to capture, retention, format) need thought. A "session summary" export at the end of a session is probably more useful than continuous journaling.

---

### 4. Impact Radar & Drift Alerts

**Score: 620/1000**

| Criterion     | Assessment                                  |
| ------------- | ------------------------------------------- |
| Concept       | Good - proactive alerts beat polling        |
| Human Utility | Medium - alert fatigue is real              |
| AI Utility    | Medium - webhook enables automation         |
| Feasibility   | Medium-high - requires daemon/watcher       |
| Debt Ratio    | Medium - daemon adds operational complexity |

Candid Take: The core idea (alert when graph structure degrades) is sound. The challenge is threshold tuning — what change magnitude warrants an alert? Too sensitive = noise, too quiet = missed issues. A daemon watching `.beads/` adds operational complexity. Better: compute drift on-demand (`bv --check-drift`) rather than continuous monitoring. Webhooks for CI integration are genuinely useful.

---

### 5. Interactive Dependency Surgery

**Score: 480/1000**

| Criterion     | Assessment                                        |
| ------------- | ------------------------------------------------- |
| Concept       | Wrong tool - `bv` is a viewer, `bd` is the editor |
| Human Utility | High if in the right tool                         |
| AI Utility    | Medium - structured diffs are useful              |
| Feasibility   | Medium - TUI modal design                         |
| Debt Ratio    | Poor in `bv`, good in `bd`                        |

Candid Take: Real-time cycle detection while editing dependencies is valuable — but this belongs in `bd` (beads), not `bv` (beads viewer). The "viewer" shouldn't mutate data. If this were proposed for `bd`, I'd score it 700+. As a `bv` feature, it violates the tool's architectural boundary.

---

### 6. AI-Aware Kanban Autoprioritizer

**Score: 760/1000**

| Criterion     | Assessment                                               |
| ------------- | -------------------------------------------------------- |
| Concept       | Good - leverage graph analysis for prioritization        |
| Human Utility | High - "what should I work on?" answered algorithmically |
| AI Utility    | High - `--robot-priority-queue` is immediately useful    |
| Feasibility   | Medium - heuristics need tuning                          |
| Debt Ratio    | Good - builds on existing analysis                       |

Candid Take: Using PageRank, betweenness, and freshness to suggest "unblock these 5 issues for maximum velocity" is exactly what graph analysis should enable. The challenge is building trust — humans may resist algorithmic prioritization. Start with suggestions that explain reasoning ("Issue X blocks 7 others and hasn't been touched in 2 weeks") rather than opaque reordering. For AI agents, a priority-ordered work queue is gold.

---

### 7. Offline Knowledge Cards

**Score: 580/1000**

| Criterion     | Assessment                                   |
| ------------- | -------------------------------------------- |
| Concept       | Reasonable - summary cards reduce reading    |
| Human Utility | Medium - quick context is nice               |
| AI Utility    | Medium - batch retrieval helps               |
| Feasibility   | Medium - good summaries are hard without LLM |
| Debt Ratio    | Medium - summary quality determines value    |

Candid Take: The idea of pressing `K` for a quick summary card is appealing, but "autogenerated summary" is doing a lot of heavy lifting. Without an LLM, summaries are just truncated text. With an LLM, you've added significant infrastructure. A simpler version (structured card showing ID, status, blocker count, last update, first 100 chars) could work without AI. Batch retrieval for agents is the stronger use case.

---

### 8. Multi-Repo Aggregation View

**Score: 680/1000**

| Criterion     | Assessment                                    |
| ------------- | --------------------------------------------- |
| Concept       | Good - unified view across repos is real need |
| Human Utility | High for multi-repo/monorepo users            |
| AI Utility    | High - cross-repo dependency visibility       |
| Feasibility   | Medium-high - ID namespacing, cross-repo deps |
| Debt Ratio    | Medium - complexity for subset of users       |

Candid Take: For organizations with multiple services or monorepos, a unified view (`bv --workspace`) is genuinely needed. The implementation challenges are real: ID collisions (two repos both have issue "123"), cross-repo dependencies, performance with many repos. Worth doing but should be opt-in and clearly scoped. Agents benefit from seeing the full picture without manually switching contexts.

---

### 9. Time Travel & What-Changed Replay

**Score: 730/1000**

| Criterion     | Assessment                                   |
| ------------- | -------------------------------------------- |
| Concept       | Good - same as Gemini's idea #5              |
| Human Utility | High - postmortems, progress tracking        |
| AI Utility    | High - snapshot grounding                    |
| Feasibility   | Medium - same implementation as Gemini's     |
| Debt Ratio    | Good - moderate complexity for solid utility |

Candid Take: Essentially the same idea as Gemini's "Time-Travel Graph Diffing" — view the board/graph as of a specific commit. Both versions are good. The "slider" UI is nice but a CLI flag (`--as-of 2024-01-15`) is more practical and agent-friendly. Scoring slightly lower than Gemini's because the framing is less specific about the diff capabilities.

---

### 10. Actionable Exports & Hooks

**Score: 740/1000**

| Criterion     | Assessment                                          |
| ------------- | --------------------------------------------------- |
| Concept       | Good - exports that enable action, not just reading |
| Human Utility | High - copy-paste ready commands                    |
| AI Utility    | High - chain exports → actions                      |
| Feasibility   | Medium - hook security needs care                   |
| Debt Ratio    | Good - hooks enable extensibility                   |

Candid Take: Exports containing runnable `bd` commands ("to close these resolved issues, run: `bd close X Y Z`") is immediately practical. Post-export hooks enable CI integration, chat posting, etc. The security surface of hooks needs thought (don't auto-execute arbitrary commands), but the concept is sound. For AI agents, structured exports with next-action hints reduce the reasoning burden.

---

## SUMMARY RANKINGS

### Top 5 Overall

| Rank | Idea                            | Source    | Score | Why                                                                       |
| ---- | ------------------------------- | --------- | ----- | ------------------------------------------------------------------------- |
| 1    | Topological Work-Plan Generator | Gemini #4 | 870   | Simple, leverages existing code, immediately useful for humans and agents |
| 2    | Context Packs & Recipes         | GPT-5 #1  | 820   | Low-cost, high-utility workflow presets                                   |
| 3    | AI-Aware Autoprioritizer        | GPT-5 #6  | 760   | Smart use of graph analysis for actionable recommendations                |
| 4    | Time-Travel Diffing             | Gemini #5 | 750   | Solid utility, moderate complexity, uses existing git data                |
| 5    | Actionable Exports & Hooks      | GPT-5 #10 | 740   | Practical, enables automation without overengineering                     |

### Bottom 5 Overall

| Rank | Idea                  | Source    | Score | Why                                       |
| ---- | --------------------- | --------- | ----- | ----------------------------------------- |
| 16   | Monte Carlo Simulator | Gemini #6 | 480   | Buzzword-driven overengineering           |
| 17   | Dependency Surgery    | GPT-5 #5  | 480   | Wrong tool (viewer vs editor)             |
| 18   | Sixel/Kitty Graphics  | Gemini #9 | 420   | Niche, limited terminal support           |
| 19   | Live Pair Panel       | GPT-5 #2  | 400   | Misunderstands agent workflows            |
| 20   | Chat-with-Graph       | Gemini #7 | 350   | Redundant, adds complexity for no benefit |

## Verdict

Gemini had higher highs (Work-Plan Generator is the best single idea) but also more misses (Chat-with-Graph, Monte Carlo). The ideas trend toward "impressive-sounding" over "practically useful."

GPT-5 was more consistently practical but less ambitious. Recipes and Actionable Exports are solid workhorse features. The Live Pair Panel shows similar "sounds cool but doesn't match reality" thinking.

My recommendation: Implement Gemini #4 (Work-Plan Generator) and GPT-5 #1 (Recipes) first. They're complementary, low-risk, high-value, and could ship in a week.

---



OK, let's actually take your top 8 ideas by score and plan to do them all; take the exact proposals for all 8, study them carefully in terms of how we would actually need to go about implementing them cleverly and effectively; then take ALL of that and elaborate on it more and then create a comprehensive and granular set of beads for all this with tasks, subtasks, and dependency structure overlaid, with detailed comments so that the whole thing is totally self-contained and self-documenting (including relevant background, reasoning/justification, considerations, etc. — anything we'd want our "future self" to know about the goals and intentions and thought process and how it serves the over-arching goals of the project). Use ultrathink.

Check over each bead super carefully — are you sure it makes sense? Is it optimal? Could we change anything to make the system work better for users? If so, revise the beads. It's a lot easier and faster to operate in "plan space" before we start implementing these things! Use ultrathink.

---

# Beads Viewer Feature Roadmap

## Overview

| # | Epic                            | Priority | Tasks | Status         |
| - | ------------------------------- | -------- | ----- | -------------- |
| 1 | Topological Work-Plan Generator | P0       | 5     | Ready to start |
| 2 | Context Packs & Recipes         | P1       | 5     | Ready to start |
| 3 | AI-Aware Autoprioritizer        | P1       | 4     | Ready to start |
| 4 | Time-Travel Graph Diffing       | P1       | 4     | Ready to start |
| 5 | Actionable Exports & Hooks      | P2       | 3     | Ready to start |
| 6 | Multi-Repo Aggregation          | P2       | 4     | Ready to start |
| 7 | Semantic Vector Search          | P3       | 3     | Ready to start |
| 8 | Impact Radar & Drift Alerts     | P3       | 3     | Ready to start |

Total: 8 epics, 31 tasks

---

## Epic 1: Topological Work-Plan Generator

**ID:** bv-ub7 | **Priority:** P0 | **Labels:** ai-agent builds-on-existing high-value

### Description

Leverage the existing DAG analysis to generate linearized, dependency-respecting work plans. This is the highest-value feature because it directly answers: "What can I actually work on right now?"

### Value Proposition

* For Humans: A "Smart To-Do" view showing only actionable items eliminates cognitive overhead
* For AI Agents: `--robot-plan` enables autonomous workflows with strictly ordered task lists

### Tasks

| ID       | Task                                                   | Priority | Depends On |
| -------- | ------------------------------------------------------ | -------- | ---------- |
| bv-ub7.1 | Add `GetActionableIssues()` to analysis package        | P1       | —          |
| bv-ub7.2 | Add `GetExecutionPlan()` with parallel track detection | P1       | .1         |
| bv-ub7.3 | Add `--robot-plan` CLI flag                            | P1       | .2         |
| bv-ub7.4 | Add TUI Actionable view with a keybinding              | P2       | .2         |
| bv-ub7.5 | Add comprehensive tests for work plan generation       | P2       | .2         |

### Task Details

**bv-ub7.1: Add `GetActionableIssues()`**

Identify issues that can be worked on immediately — those with no open blocking dependencies.

```go
func (a *Analyzer) GetActionableIssues() []model.Issue
```

Key Design Decisions:

* Only `blocks` type creates hard dependencies
* Missing blockers don't block (graceful degradation)
* Closed issues are never actionable

**bv-ub7.2: Add `GetExecutionPlan()`**

Generate a dependency-respecting execution plan with parallel tracks identified.

```go
type ExecutionPlan struct {
  Tracks []ExecutionTrack
  Total  int
}

type ExecutionTrack struct {
  TrackID string
  Items   []PlanItem
  Reason  string  // Why these are grouped
}

type PlanItem struct {
  ID          string
  Title       string
  Priority    int
  UnblocksIDs []string  // What becomes actionable when done
}
```

**bv-ub7.3: Add `--robot-plan` CLI**

```json
{
  "generated_at": "2025-11-26T20:00:00Z",
  "tracks": [{
    "track_id": "track-1",
    "items": [
      {
        "id": "issue-123",
        "title": "...",
        "unblocks": ["issue-456"]
      }
    ],
    "reason": "Independent work stream focused on auth module"
  }],
  "total_actionable": 5,
  "total_blocked": 12,
  "summary": {
    "highest_impact": "issue-123",
    "impact_reason": "Unblocks 3 tasks"
  }
}
```

**bv-ub7.4: TUI Actionable View**

```text
┌─ ACTIONABLE (5 items) ────────────────────────┐
│ Track 1: Auth Module                          │
│ ├─ 🔥 P0 AUTH-123 Fix login timeout           │
│ │      └─ Unblocks: AUTH-456, AUTH-789        │
│ └─ ⚡ P1 AUTH-124 Add OAuth support           │
└───────────────────────────────────────────────┘
```

---

## Epic 2: Context Packs & Recipes System

**ID:** bv-ufd | **Priority:** P1 | **Labels:** ai-agent developer-experience workflow

### Description

Named workflow presets ("recipes") that bundle filters, layouts, and exports into one-command operations.

### Value Proposition

* For Humans: `bv --recipe triage` replaces complex flag combinations
* For AI Agents: Semantic entry points — "triage mode" vs "release mode"

### Built-in Recipes

| Recipe          | Description                             |
| --------------- | --------------------------------------- |
| triage          | Open/blocked issues, sorted by priority |
| release-cut     | Recently closed issues for changelog    |
| blocked-review  | All blocked items with blocker info     |
| dependency-risk | High betweenness/PageRank items         |
| quick-wins      | Low-priority, no-dependency items       |
| stale           | Items not updated in 14+ days           |

### Tasks

| ID       | Task                                                    | Priority | Depends On |
| -------- | ------------------------------------------------------- | -------- | ---------- |
| bv-ufd.1 | Define recipe schema and configuration structure        | P1       | —          |
| bv-ufd.2 | Implement recipe loader with embedded defaults          | P1       | .1         |
| bv-ufd.3 | Create built-in recipe library                          | P2       | .1         |
| bv-ufd.4 | Add `--recipe` CLI flag and `--robot-recipes` discovery | P1       | .2         |
| bv-ufd.5 | Add TUI recipe picker overlay                           | P2       | .2         |

### Task Details

**bv-ufd.1: Recipe Schema**

```yaml
recipes:
  triage:
    description: "Focus on urgent open/blocked items"
    filters:
      status: [open, blocked]
      priority_max: 2
    sort:
      by: priority
      direction: asc
    view: board
```

**bv-ufd.2: Recipe Loader**

Loading Order (later overrides earlier):

1. Embedded defaults (compiled into binary)
2. User config at `~/.config/bv/recipes.yaml`
3. Project config at `.bv/recipes.yaml`

**bv-ufd.5: TUI Recipe Picker**

```text
┌─ Select Recipe ─────────────────────────────┐
│  > triage                                   │
│    Urgent items needing attention           │
│                                             │
│    release-cut                              │
│    Recently closed items for changelog      │
│                                             │
│ j/k: navigate • enter: apply • esc: cancel  │
└─────────────────────────────────────────────┘
```

---

## Epic 3: AI-Aware Kanban Autoprioritizer

**ID:** bv-lkk | **Priority:** P1 | **Labels:** ai-agent analysis prioritization

### Description

Use graph metrics to suggest priority adjustments and surface high-impact items.

### Scoring Function

```text
impact_score = (
    0.3 × normalized_pagerank +
    0.3 × normalized_betweenness +
    0.2 × blocker_count / max_blocker_count +
    0.1 × freshness_penalty +
    0.1 × explicit_priority_boost
)
```

### Value Proposition

* For Humans: Smart suggestions explaining why items should be reprioritized
* For AI Agents: `--robot-priority` provides ranked work queue with reasoning

### Tasks

| ID       | Task                                                | Priority | Depends On |
| -------- | --------------------------------------------------- | -------- | ---------- |
| bv-lkk.1 | Implement priority scoring function                 | P1       | —          |
| bv-lkk.2 | Generate priority recommendations with explanations | P1       | .1         |
| bv-lkk.3 | Add `--robot-priority` CLI flag                     | P1       | .2         |
| bv-lkk.4 | Add TUI priority suggestion indicators              | P2       | .2         |

### Task Details

**bv-lkk.2: Recommendations Output**

```go
type PriorityRecommendation struct {
    IssueID           string
    CurrentPriority   int
    SuggestedPriority int
    ImpactScore       float64
    Confidence        float64   // 0-1
    Reasoning         []string  // Human-readable explanations
}
```

Reasoning Templates:

* "Blocks N high-priority items"
* "Critical path bottleneck"
* "High centrality in graph"
* "Stale for N days"

**bv-lkk.3: CLI Output**

```json
{
  "recommendations": [
    {
      "id": "issue-123",
      "current_priority": 2,
      "suggested_priority": 0,
      "impact_score": 0.85,
      "confidence": 0.9,
      "reasoning": ["Blocks 5 high-priority items", "Critical path bottleneck"]
    }
  ],
  "summary": {
    "total_issues": 50,
    "recommendations": 8,
    "high_confidence": 3
  }
}
```

---

## Epic 4: Time-Travel Graph Diffing

**ID:** bv-2a4 | **Priority:** P1 | **Labels:** analysis git-integration history

### Description

Compare graph structure across git commits to understand project evolution and detect structural regressions.

### Value Proposition

* For Humans: Visualize progress, sprint analysis, postmortems
* For AI Agents: Structural regression detection before committing

### Key Features

* Load historical state without checking out
* Compare metrics between snapshots
* Detect new cycles or bottlenecks
* "What-if" simulation

### Tasks

| ID       | Task                                           | Priority | Depends On |
| -------- | ---------------------------------------------- | -------- | ---------- |
| bv-2a4.1 | Implement git history loader for beads files   | P1       | —          |
| bv-2a4.2 | Create snapshot comparison and diff generation | P1       | .1         |
| bv-2a4.3 | Add `--diff-since` and `--as-of` CLI flags     | P1       | .2         |
| bv-2a4.4 | Add TUI diff highlighting and history mode     | P2       | .2         |

### Task Details

**bv-2a4.1: Git History Loader**

Load beads JSONL from specific commit via:

```bash
git show <commit>:.beads/beads.base.jsonl
```

Beads tracks JSONL in git (SQLite is git-ignored).

Supports: SHA, branch, tag, date resolution with caching.

**bv-2a4.2: Diff Generation**

```go
type GraphDiff struct {
  NewIssues      []string
  ClosedIssues   []string
  RemovedIssues  []string
  ModifiedIssues []string
  NewCycles      [][]string
  ResolvedCycles [][]string
  MetricDelta    map[string]float64
}
```

**bv-2a4.4: TUI History Mode**

* `T` enters time-travel mode
* 🆕 badge on new issues
* ✅ on newly closed
* Diff summary in footer

---

## Epic 5: Actionable Exports & Hooks System

**ID:** bv-qjc | **Priority:** P2 | **Labels:** automation export hooks

### Description

Enhance exports to include runnable commands and add a hook system for automation.

### Value Proposition

* For Humans: Exports include copy-paste ready commands
* For AI Agents: Chain exports → actions without manual parsing

### Hook Use Cases

* Post to Slack after export
* Create GitHub issue from bead
* Trigger CI pipeline
* Sync to external tracker

### Tasks

| ID       | Task                                           | Priority | Depends On |
| -------- | ---------------------------------------------- | -------- | ---------- |
| bv-qjc.1 | Add command snippets to markdown export        | P2       | —          |
| bv-qjc.2 | Design and implement hook configuration system | P2       | —          |
| bv-qjc.3 | Integrate hooks with export pipeline           | P2       | .2         |

### Task Details

**bv-qjc.1: Command Snippets**

### Quick Actions

```bash
# Close all resolved items
bd close issue-1 issue-2 issue-3

# Bulk priority update
bd update issue-4 issue-5 -p P1
```

#### bv-qjc.2: Hook Configuration

```yaml
# .bv/hooks.yaml
hooks:
  pre-export:
    - name: validate-issues
      command: "./scripts/validate.sh"
      timeout: 10s
  post-export:
    - name: slack-notify
      command: "curl -X POST ..."
      env:
        SLACK_WEBHOOK: ${SLACK_WEBHOOK}
```

Environment Variables: `BV_EXPORT_PATH`, `BV_EXPORT_FORMAT`, `BV_ISSUE_COUNT`, `BV_TIMESTAMP`

---

## Epic 6: Multi-Repo Aggregation View

**ID:** bv-epf | **Priority:** P2 | **Labels:** multi-repo scalability

### Description

Unified view across multiple `.beads/` directories for monorepos and multi-service projects.

### Value Proposition

* For Humans: Single view of all project issues
* For AI Agents: Cross-repo dependency visibility, scoped queries

### Key Challenges

* ID collision handling via namespacing
* Performance with many repos
* Cross-repo dependency semantics
* Clear provenance in UI

### Tasks

| ID       | Task                                           | Priority | Depends On |
| -------- | ---------------------------------------------- | -------- | ---------- |
| bv-epf.1 | Design workspace configuration schema          | P2       | —          |
| bv-epf.2 | Implement namespaced ID system                 | P2       | .1         |
| bv-epf.3 | Create aggregate loader for multiple repos     | P2       | .2         |
| bv-epf.4 | Add `--workspace` CLI flag and TUI repo filter | P2       | .3         |

### Task Details

**bv-epf.1: Workspace Config**

```yaml
# .bv/workspace.yaml
repos:
  - name: backend
    path: ../backend
    prefix: BE
  - name: frontend
    path: ../frontend
    prefix: FE
```

**bv-epf.2: Namespaced IDs**

Format: `<prefix>:<original_id>` (e.g., `BE:AUTH-123`)

Cross-repo dependencies use full namespaced IDs. Display adapts to context.

---

## Epic 7: Semantic Vector Search

**ID:** bv-9gf | **Priority:** P3 | **Labels:** ai-agent search semantic

### Description

Natural language search via vector embeddings, enabling queries like "memory leak issues" without exact keyword matches.

### Value Proposition

* For Humans: "that thing about memory leaks" actually finds it
* For AI Agents: RAG over project history — "how did we solve similar issues before?"

### Scope Control (Start Minimal)

* Embed title + description only
* Single embedding model
* Top-10 results
* No hybrid search initially

### Tasks

| ID       | Task                                        | Priority | Depends On |
| -------- | ------------------------------------------- | -------- | ---------- |
| bv-9gf.1 | Research and select embedding approach      | P3       | —          |
| bv-9gf.2 | Implement vector storage and indexing       | P3       | .1         |
| bv-9gf.3 | Add semantic search CLI and TUI integration | P3       | .2         |

### Task Details

**bv-9gf.1: Embedding Research**

Evaluate approaches:

* Python subprocess (sentence-transformers)
* Go-native
* API-based
* `sqlite-vec` extension

Consider: Quality, complexity, performance, binary size.

**bv-9gf.3: CLI/TUI Integration**

* `bv --search 'query'` for semantic search
* `--robot-search` outputs JSON
* TUI: `Ctrl+S` toggles semantic/regex mode

---

## Epic 8: Impact Radar & Drift Alerts

**ID:** bv-52t | **Priority:** P3 | **Labels:** ci monitoring quality

### Description

Detect significant changes in graph metrics and alert when structure degrades.

### Value Proposition

* For Humans: Early warning when project health degrades
* For AI Agents: Validate changes before committing — "did I introduce a cycle?"

### Alert Types

| Type              | Severity | Trigger                  |
| ----------------- | -------- | ------------------------ |
| New cycles        | Critical | Any cycle detected       |
| PageRank spike    | Warning  | Potential new bottleneck |
| Betweenness spike | Warning  | New critical path node   |
| Density increase  | Info     | Complexity growing       |

### Tasks

| ID       | Task                                       | Priority | Depends On |
| -------- | ------------------------------------------ | -------- | ---------- |
| bv-52t.1 | Implement baseline storage and management  | P3       | —          |
| bv-52t.2 | Create drift calculator with thresholds    | P3       | .1         |
| bv-52t.3 | Add `--check-drift` CLI for CI integration | P3       | .2         |

### Task Details

**bv-52t.1: Baseline Storage**

* `.bv/baseline.json` stores metrics snapshot
* `--save-baseline` creates snapshot
* `--baseline-info` shows current baseline
* Includes commit annotation

**bv-52t.3: CI Integration**

```bash
bv --check-drift
# Exit codes: 0=OK, 1=critical, 2=warning

bv --robot-drift  # JSON output for automation
```

---

## Dependency Graph

```text
Epic 1 (P0): bv-ub7.1 → bv-ub7.2 → bv-ub7.3
                            ├──→ bv-ub7.4
                            └──→ bv-ub7.5

Epic 2 (P1): bv-ufd.1 → bv-ufd.2 → bv-ufd.4
                │          └──→ bv-ufd.5
                └──→ bv-ufd.3

Epic 3 (P1): bv-lkk.1 → bv-lkk.2 → bv-lkk.3
                            └──→ bv-lkk.4

Epic 4 (P1): bv-2a4.1 → bv-2a4.2 → bv-2a4.3
                            └──→ bv-2a4.4

Epic 5 (P2): bv-qjc.1 (independent)
             bv-qjc.2 → bv-qjc.3

Epic 6 (P2): bv-epf.1 → bv-epf.2 → bv-epf.3 → bv-epf.4

Epic 7 (P3): bv-9gf.1 → bv-9gf.2 → bv-9gf.3

Epic 8 (P3): bv-52t.1 → bv-52t.2 → bv-52t.3
```

## Actionable Now (9 tasks)

| Priority | ID       | Task                    | Package       |
| -------- | -------- | ----------------------- | ------------- |
| P1       | bv-ub7.1 | `GetActionableIssues()` | pkg/analysis  |
| P1       | bv-ufd.1 | Recipe schema           | pkg/recipe    |
| P1       | bv-lkk.1 | Priority scoring        | pkg/analysis  |
| P1       | bv-2a4.1 | Git history loader      | pkg/loader    |
| P2       | bv-qjc.1 | Command snippets        | pkg/export    |
| P2       | bv-qjc.2 | Hook configuration      | pkg/hooks     |
| P2       | bv-epf.1 | Workspace config        | pkg/workspace |
| P3       | bv-9gf.1 | Embedding research      | research      |
| P3       | bv-52t.1 | Baseline storage        | pkg/drift     |
