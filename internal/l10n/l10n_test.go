package l10n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestLanguagesAndFallback(t *testing.T) {
	for _, tc := range []struct{ tag, source, want string }{
		{"zh-Hans", "设置", "设置"},
		{"en", "设置", "Settings"},
		{"en-US", "设置", "Settings"},
		{"de", "设置", "设置"},
		{"en", "unknown", "unknown"},
	} {
		if got := In(tc.tag, tc.source); got != tc.want {
			t.Errorf("In(%q, %q) = %q, want %q", tc.tag, tc.source, got, tc.want)
		}
	}
	if got := fmt.Sprintf(In("en", "订阅 %s 更新完成，共 %d 个频道"), "测试源", 42); got != "Subscription 测试源 refresh complete. Channels: 42" {
		t.Fatal(got)
	}
	if !Matches("Subscription refresh complete; preparing channels…", "订阅更新完成，正在整理频道…") {
		t.Fatal("completion message not recognized across languages")
	}
}

func TestCatalogFormattingAndCompleteness(t *testing.T) {
	// Preserve formatting verbs and their order, including escaped percent
	// signs and %w. A translation must never change the caller's arguments.
	verbs := regexp.MustCompile(`%(?:\[[0-9]+\])?[-+# 0]*(?:[0-9]+|\*)?(?:\.(?:[0-9]+|\*))?[a-zA-Z%]`)
	for source, translated := range english {
		if strings.TrimSpace(source) == "" || strings.TrimSpace(translated) == "" {
			t.Errorf("empty catalog entry: %q", source)
		}
		if a, b := strings.Join(verbs.FindAllString(source, -1), ","), strings.Join(verbs.FindAllString(translated, -1), ","); a != b {
			t.Errorf("format changed for %q: %q -> %q", source, a, b)
		}
	}
	files, err := filepath.Glob("../app/*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok || fn.Name != "tr" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true // Controlled source-language enum values.
			}
			source, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			checked++
			if _, ok := english[source]; !ok {
				t.Errorf("%s: missing English translation for %q", fset.Position(lit.Pos()), source)
			}
			return true
		})
	}
	if checked < 300 {
		t.Fatalf("catalog coverage unexpectedly small: %d call sites", checked)
	}
}
