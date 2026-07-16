package ui

// Glyph table (bt-5f3bo) — the single source of truth for every TUI chrome
// glyph. All status marks, scope/filter marks, type/priority icons, footer
// signals, tree/braille connectors, and typographic separators are named
// entries here rather than inline literals, so the whole TUI can switch glyph
// vocabulary at startup.
//
// Two tiers, resolved once from the BT_GLYPHS env var:
//
//   - nerdfont (DEFAULT): Nerd Font PUA icons (Font Awesome range,
//     U+E000..U+F8FF, single-width, written as \uXXXX escapes) plus
//     unicode-but-not-emoji marks (circle status dots, braille, middle-dot
//     separator, return arrow). Every glyph here is single-width so the
//     layout math holds.
//   - ascii (BT_GLYPHS=ascii): pure ASCII, every glyph < 0x80.
//
// There is NO emoji tier. Emoji render double-width and break layout math; they
// were deleted from the codebase (starship/yazi precedent — NF default, ASCII
// fallback, no font detection because no terminal API reports installed fonts).
//
// Provisional Nerd Font picks (user sign-off pending, bt-5f3bo): each pick is a
// one-line change in nerdfontGlyphs below. Roles beyond the approved palette
// (folder/globe/filter/tag/magnifier/sort/check/play/ban/alert/bell) map to the
// closest Font Awesome glyph; see the bead report for the full list.

import (
	"os"
	"strings"
)

// GlyphSet names every chrome glyph the TUI draws. Both tiers implement the
// same field set; a field is one string (usually one display cell wide).
type GlyphSet struct {
	// --- Typography / separators ---
	Sep      string // chrome separator between footer/lens segments ("·")
	SepBar   string // vertical separator between key-hint pills
	Bullet   string // list bullet
	Ellipsis string // truncation suffix
	Return   string // enter/return key hint glyph

	// --- Lifecycle status marks (detail pane, tree, list) ---
	StOpen       string
	StInProgress string
	StBlocked    string
	StClosed     string
	StDeferred   string
	StPinned     string
	StHooked     string
	StReview     string
	StUnknown    string

	// --- Footer actionable triad (bt-p8y2f: default center-zone content,
	// lens-scoped ready/in-flight/blocked; replaces the deleted per-status
	// stat dots) ---
	TriadReady    string // actionable/unblocked segment (fa-check)
	TriadInFlight string // in_progress segment (fa-play)
	TriadBlocked  string // graph-blocked segment (fa-ban)
	Phase2Dot     string // "metrics pending" marker

	// --- Priority ---
	PrCritical string
	PrHigh     string
	PrMedium   string
	PrLow      string
	PrBacklog  string

	// --- Issue types ---
	TypeBug     string
	TypeFeature string
	TypeTask    string
	TypeEpic    string
	TypeChore   string
	TypeDefault string

	// --- Dependency-edge kinds (dep tree) ---
	DepRoot       string
	DepBlocks     string
	DepRelated    string
	DepChild      string
	DepDiscovered string

	// --- Toast / severity ---
	Success string // ok/confirm
	Cross   string // failure / error / rejected
	Warning string // warning / degraded
	Info    string // note

	// --- Lifecycle events (history correlation) ---
	EventCreated  string
	EventClaimed  string
	EventClosed   string
	EventReopened string
	EventModified string

	// --- Footer scope / filter chrome ---
	ScopeProject string // single-project scope (folder)
	ScopeAll     string // all-projects scope (globe)
	FilterAll    string // status filter: all
	FilterOpen   string // status filter: open
	FilterClosed string // status filter: closed
	FilterReady  string // status filter: ready
	FilterBQL    string // BQL query filter
	FilterRecipe string // recipe filter
	FilterStatus string // generic status filter (filter funnel)
	Search       string // search / magnifier
	Sort         string // sort/order
	Tag          string // label filter
	Graph        string // graph analysis
	RepoDrawer   string // repo filter drawer
	Workspace    string // workspace summary
	Bell         string // unread events
	Star         string // self-update / quick-win star
	Session      string // cass session count
	Bolt         string // secondary alert / "blocks N"
	Unlock       string // unblocks indicator
	Comment      string // comment count
	Clock        string // time-travel / timer

	// --- Gate badges ---
	GateHuman string
	GateCI    string
	GatePR    string
	GateBead  string
	GatePause string
	Overdue   string
	Stale     string

	// --- Semantic colour dots (legends / confidence; colour via styling) ---
	DotReady   string // green
	DotWarn    string // yellow
	DotHigh    string // orange
	DotBlocked string // red
	DotInfo    string // blue
	DotClosed  string // black/neutral
	DotIdle    string // white/hollow

	// --- Decorative section / node icons ---
	BarChart     string
	Target       string
	Bulb         string
	Brain        string
	Microscope   string
	Books        string
	Satellite    string
	Building     string
	Construction string
	Knot         string
	Cycle        string
	Hourglass    string
	Wrench       string
	Memo         string
	Page         string
	Clipboard    string
	Pushpin      string
	Hook         string
	Broom        string
	Package      string
	Paperclip    string
	Scroll       string
	TestTube     string
	Snowflake    string
	Coffee       string
	Sleep        string
	Fire         string
	Rocket       string
	Link         string
	NoEntry      string
	Question     string
	New          string
	User         string
	Pencil       string
	Globe        string
	Refresh      string

	// --- Braille progress-bar cells (epics tree) ---
	BrailleFull  string
	BrailleHalf  string
	BrailleTrack string

	// --- Worker spinner frames ---
	Spinner []string
}

// nerdfontGlyphs is the DEFAULT tier: Nerd Font PUA icons (\uXXXX, Font Awesome
// nf-fa range) + non-emoji unicode marks. All single-width.
var nerdfontGlyphs = GlyphSet{
	Sep:      "·",
	SepBar:   "│",
	Bullet:   "•",
	Ellipsis: "…",
	Return:   "⏎",

	StOpen:       "○",
	StInProgress: "◐",
	StBlocked:    "⊘",
	StClosed:     "●",
	StDeferred:   "◇",
	StPinned:     "◆",
	StHooked:     "◈",
	StReview:     "◑",
	StUnknown:    "◌",

	TriadReady:    "", // fa-check
	TriadInFlight: "", // fa-play
	TriadBlocked:  "", // fa-ban
	Phase2Dot:     "◌",

	PrCritical: "", // fa-fire
	PrHigh:     "", // fa-bolt
	PrMedium:   "", // fa-circle
	PrLow:      "", // fa-coffee
	PrBacklog:  "", // fa-moon-o

	TypeBug:     "", // fa-bug
	TypeFeature: "", // fa-star
	TypeTask:    "", // fa-check-square-o
	TypeEpic:    "", // fa-rocket
	TypeChore:   "", // fa-wrench
	TypeDefault: "", // fa-file-o

	DepRoot:       "", // fa-map-marker
	DepBlocks:     "", // fa-ban
	DepRelated:    "", // fa-link
	DepChild:      "", // fa-archive
	DepDiscovered: "", // fa-search

	Success: "", // fa-check
	Cross:   "", // fa-times
	Warning: "", // fa-exclamation-triangle
	Info:    "", // fa-info-circle

	EventCreated:  "", // fa-plus-circle
	EventClaimed:  "", // fa-user
	EventClosed:   "", // fa-check
	EventReopened: "", // fa-refresh
	EventModified: "", // fa-pencil

	ScopeProject: "", // fa-folder
	ScopeAll:     "", // fa-globe
	FilterAll:    "", // fa-list-ul
	FilterOpen:   "", // fa-folder-open
	FilterClosed: "", // fa-check
	FilterReady:  "", // fa-rocket
	FilterBQL:    "", // fa-search
	FilterRecipe: "", // fa-list-ol
	FilterStatus: "", // fa-filter
	Search:       "", // fa-search
	Sort:         "", // fa-sort
	Tag:          "", // fa-tag
	Graph:        "", // fa-bar-chart
	RepoDrawer:   "", // fa-folder-open-o
	Workspace:    "", // fa-users
	Bell:         "", // fa-bell
	Star:         "", // fa-star
	Session:      "", // fa-paperclip
	Bolt:         "", // fa-bolt
	Unlock:       "", // fa-unlock
	Comment:      "", // fa-comment
	Clock:        "", // fa-clock-o

	GateHuman: "", // fa-user
	GateCI:    "", // fa-cog
	GatePR:    "", // fa-code-fork
	GateBead:  "", // fa-link
	GatePause: "", // fa-pause
	Overdue:   "", // fa-clock-o
	Stale:     "", // fa-moon-o

	DotReady:   "●",
	DotWarn:    "●",
	DotHigh:    "●",
	DotBlocked: "●",
	DotInfo:    "●",
	DotClosed:  "●",
	DotIdle:    "○",

	BarChart:     "", // fa-bar-chart
	Target:       "", // fa-bullseye
	Bulb:         "", // fa-lightbulb-o
	Brain:        "", // fa-cogs
	Microscope:   "", // fa-flask
	Books:        "", // fa-book
	Satellite:    "", // fa-globe
	Building:     "", // fa-building
	Construction: "", // fa-wrench
	Knot:         "", // fa-link
	Cycle:        "", // fa-refresh
	Hourglass:    "", // fa-hourglass-half
	Wrench:       "", // fa-wrench
	Memo:         "", // fa-pencil
	Page:         "", // fa-file-o
	Clipboard:    "", // fa-clipboard
	Pushpin:      "", // fa-thumb-tack
	Hook:         "", // fa-link
	Broom:        "", // fa-eraser
	Package:      "", // fa-archive
	Paperclip:    "", // fa-paperclip
	Scroll:       "", // fa-file-text-o
	TestTube:     "", // fa-flask
	Snowflake:    "", // fa-snowflake-o
	Coffee:       "", // fa-coffee
	Sleep:        "", // fa-moon-o
	Fire:         "", // fa-fire
	Rocket:       "", // fa-rocket
	Link:         "", // fa-link
	NoEntry:      "", // fa-ban
	Question:     "", // fa-question-circle
	New:          "", // fa-plus-circle
	User:         "", // fa-user
	Pencil:       "", // fa-pencil
	Globe:        "", // fa-globe
	Refresh:      "", // fa-refresh

	BrailleFull:  "⣿", // U+28FF
	BrailleHalf:  "⡇", // U+2847
	BrailleTrack: "⠤", // U+2824

	Spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
}

// asciiGlyphs is the fallback tier: pure ASCII, every glyph < 0x80. Selected via
// BT_GLYPHS=ascii. Distinct-enough marks so meaning survives without color.
var asciiGlyphs = GlyphSet{
	Sep:      ".",
	SepBar:   "|",
	Bullet:   "-",
	Ellipsis: "...",
	Return:   "Ent",

	StOpen:       "o",
	StInProgress: "*",
	StBlocked:    "!",
	StClosed:     "x",
	StDeferred:   "z",
	StPinned:     "^",
	StHooked:     "&",
	StReview:     ":",
	StUnknown:    "?",

	TriadReady:    "ready ",
	TriadInFlight: "in-flight ",
	TriadBlocked:  "blocked ",
	Phase2Dot:     ".",

	PrCritical: "!",
	PrHigh:     "^",
	PrMedium:   "=",
	PrLow:      "-",
	PrBacklog:  "z",

	TypeBug:     "b",
	TypeFeature: "*",
	TypeTask:    "t",
	TypeEpic:    "E",
	TypeChore:   "c",
	TypeDefault: ".",

	DepRoot:       "@",
	DepBlocks:     "x",
	DepRelated:    "~",
	DepChild:      "+",
	DepDiscovered: "?",

	Success: "v",
	Cross:   "x",
	Warning: "!",
	Info:    "i",

	EventCreated:  "+",
	EventClaimed:  "@",
	EventClosed:   "v",
	EventReopened: "o",
	EventModified: "*",

	ScopeProject: "#",
	ScopeAll:     "*",
	FilterAll:    "=",
	FilterOpen:   ">",
	FilterClosed: "v",
	FilterReady:  "^",
	FilterBQL:    "?",
	FilterRecipe: "=",
	FilterStatus: "#",
	Search:       "?",
	Sort:         "^",
	Tag:          "#",
	Graph:        "#",
	RepoDrawer:   "/",
	Workspace:    "@",
	Bell:         "!",
	Star:         "*",
	Session:      "@",
	Bolt:         "^",
	Unlock:       "o",
	Comment:      "#",
	Clock:        "@",

	GateHuman: "@",
	GateCI:    "%",
	GatePR:    "Y",
	GateBead:  "=",
	GatePause: "=",
	Overdue:   "!",
	Stale:     "z",

	DotReady:   "o",
	DotWarn:    "*",
	DotHigh:    "*",
	DotBlocked: "!",
	DotInfo:    "o",
	DotClosed:  "x",
	DotIdle:    "o",

	BarChart:     "#",
	Target:       "@",
	Bulb:         "*",
	Brain:        "%",
	Microscope:   "?",
	Books:        "=",
	Satellite:    "^",
	Building:     "#",
	Construction: "!",
	Knot:         "%",
	Cycle:        "o",
	Hourglass:    "~",
	Wrench:       "%",
	Memo:         "*",
	Page:         "=",
	Clipboard:    "=",
	Pushpin:      "+",
	Hook:         "?",
	Broom:        "/",
	Package:      "#",
	Paperclip:    "@",
	Scroll:       "=",
	TestTube:     "?",
	Snowflake:    "*",
	Coffee:       "u",
	Sleep:        "z",
	Fire:         "!",
	Rocket:       "^",
	Link:         "~",
	NoEntry:      "x",
	Question:     "?",
	New:          "+",
	User:         "@",
	Pencil:       "*",
	Globe:        "*",
	Refresh:      "o",

	BrailleFull:  "#",
	BrailleHalf:  "=",
	BrailleTrack: "-",

	Spinner: []string{"|", "/", "-", "\\"},
}

// activeGlyphs is the resolved tier, chosen once at startup from BT_GLYPHS.
// Unknown/empty -> nerdfont default. No font detection (impossible over
// terminal APIs); this is the starship/yazi precedent.
var activeGlyphs = resolveGlyphs(os.Getenv("BT_GLYPHS"))

// resolveGlyphs maps a BT_GLYPHS value to a tier. Only "ascii" selects the
// fallback; anything else (including "", "nerdfont", or a typo) keeps the
// nerdfont default.
func resolveGlyphs(mode string) GlyphSet {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "ascii":
		return asciiGlyphs
	default:
		return nerdfontGlyphs
	}
}

// Glyphs returns the resolved glyph tier (read-only). Exposed so external-test
// packages and any future consumer can reference the active chrome vocabulary
// without hard-coding literals.
func Glyphs() GlyphSet { return activeGlyphs }
