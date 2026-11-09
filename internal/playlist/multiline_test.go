package playlist

import "testing"

func TestMergeChannelLines(t *testing.T) {
	raw := []Channel{
		{Name: "CCTV-1", TVGID: "cctv1", URL: "https://example.org/a"},
		{Name: "CCTV 1", TVGID: "CCTV1", URL: "https://example.org/b"},
		{Name: "CCTV-1", TVGID: "cctv1", URL: "https://example.org/a"},
		{Name: "东方 卫视", URL: "https://example.org/c"},
		{Name: "东方卫视", URL: "https://example.org/d"},
	}
	merged := Merge(raw)
	if len(merged) != 2 || len(merged[0].Lines) != 2 || len(merged[1].Lines) != 2 {
		t.Fatalf("unexpected merge: %+v", merged)
	}
	if PreferredLine(merged[0], raw[1].URL) != 1 || PreferredLine(merged[0], "expired") != 0 {
		t.Fatal("preferred line selection failed")
	}
	tried := map[string]bool{raw[0].URL: true}
	if NextLine(merged[0], tried) != 1 {
		t.Fatal("failed to find untried route")
	}
	tried[raw[1].URL] = true
	if NextLine(merged[0], tried) != -1 {
		t.Fatal("exhausted routes should not loop")
	}
	again := Merge(merged)
	if len(again) != 2 || len(again[0].Lines) != 2 {
		t.Fatal("merge should be idempotent")
	}
}
func TestMissingIDUsesUnambiguousName(t *testing.T) {
	raw := []Channel{
		{Name: "CCTV-1", URL: "https://example.org/a"},
		{Name: "CCTV 1", TVGID: "cctv1", URL: "https://example.org/b"},
	}
	merged := Merge(raw)
	if len(merged) != 1 || len(merged[0].Lines) != 2 || merged[0].TVGID != "cctv1" {
		t.Fatalf("missing-ID fallback failed: %+v", merged)
	}
	ambiguous := append(raw, Channel{Name: "CCTV1", TVGID: "other", URL: "https://example.org/c"})
	if len(Merge(ambiguous)) != 3 {
		t.Fatal("ambiguous names must not join distinct IDs")
	}
}

func TestKeepDistinctRegionsQualityAndIDs(t *testing.T) {
	raw := []Channel{
		{Name: "上海新闻", URL: "https://example.org/1"},
		{Name: "北京新闻", URL: "https://example.org/2"},
		{Name: "CCTV5", URL: "https://example.org/3"},
		{Name: "CCTV5+", URL: "https://example.org/4"},
		{Name: "News HD", TVGID: "news", URL: "https://example.org/5"},
		{Name: "News SD", TVGID: "news", URL: "https://example.org/6"},
		{Name: "Same", TVGID: "one", URL: "https://example.org/7"},
		{Name: "Same", TVGID: "two", URL: "https://example.org/8"},
	}
	if merged := Merge(raw); len(merged) != len(raw) {
		t.Fatalf("distinct channels were merged: %+v", merged)
	}
}
