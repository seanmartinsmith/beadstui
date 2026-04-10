package export

import (
	"fmt"
	"html"
	"strings"
	"testing"
)

func TestGenerateUltimateHTML_EscapesTitleAndProject(t *testing.T) {
	title := `bad <title> "x"`
	project := `proj & more <name>`
	hash := `hash<bad&`

	out := generateUltimateHTML(title, hash, `{}`, 1, 1, project, "", "")

	safeTitle := html.EscapeString(title)
	safeProject := html.EscapeString(project)
	safeHash := html.EscapeString(hash)

	if !strings.Contains(out, fmt.Sprintf("<title>%s | bv Graph</title>", safeTitle)) {
		t.Fatalf("expected escaped title in <title> tag")
	}
	if !strings.Contains(out, fmt.Sprintf("<h1><span>%s</span> Graph</h1>", safeTitle)) {
		t.Fatalf("expected escaped title in header")
	}
	if !strings.Contains(out, fmt.Sprintf("Hash: %s", safeHash)) {
		t.Fatalf("expected escaped hash in footer")
	}
	if !strings.Contains(out, fmt.Sprintf("Project: %s", safeProject)) {
		t.Fatalf("expected escaped project name in footer")
	}
}
