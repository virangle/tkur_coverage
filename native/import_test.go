// SPDX-License-Identifier: EPL-2.0
package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func source(t *testing.T, name, text string) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	return root, path
}
func TestGoWeightedStatementsAndEscaping(t *testing.T) {
	root, _ := source(t, "x.go", "<script>α</script>\nreturn 1\n")
	raw := []byte("mode: atomic\nmod/x.go:1.1,1.10 3 0\nmod/x.go:2.1,2.9 2 4\nmod/x.go:2.1,2.9 2 7\n")
	files, err := parseGo(raw, options{Root: root, Module: "mod"})
	if err != nil {
		t.Fatal(err)
	}
	m := files[0].Metrics["statements"]
	if m.Covered != 2 || m.Total != 5 || *m.Percent != 40 {
		t.Fatal(m)
	}
	rendered, err := renderLines(files[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered[0].Markup)
	if strings.Contains(text, "<script>") || !strings.Contains(text, "&lt;script&gt;") {
		t.Fatal(text)
	}
	if len(files[0].Ranges) != 2 || files[0].Ranges[1].Count != 11 {
		t.Fatal(files[0].Ranges)
	}
}
func TestGoProfileValidation(t *testing.T) {
	root, _ := source(t, "x.go", "abcdef\n")
	for _, raw := range []string{
		"", "mode: invalid\n", "mode: set\nx.go:1.1,1.3 1 2\n",
		"mode: count\nx.go:1.1,1.3 1 -1\n", "mode: set\n../x.go:1.1,1.3 1 0\n",
		"mode: set\nx.go:1.1,9.3 1 0\n", "mode: set\nx.go:1.1,1.9999 1 0\n",
		"mode: set\nx.go:1.1,1.3 1 0\nx.go:1.1,1.3 2 1\n",
		"mode: count\nx.go:1.1,1.3 1 9223372036854775807\nx.go:1.1,1.3 1 1\n",
	} {
		if _, err := parseGo([]byte(raw), options{Root: root}); err == nil {
			t.Fatalf("accepted malformed profile %q", raw)
		}
	}
}
func TestZeroAndOverflow(t *testing.T) {
	m, err := measured(0, 0)
	if err != nil || m.Percent != nil {
		t.Fatal(m, err)
	}
	if _, err := measured(4, 3); err == nil {
		t.Fatal("invalid numerator accepted")
	}
	if _, err := addMetric(metric{Total: math.MaxInt64}, metric{Total: 1}); err == nil {
		t.Fatal("overflow accepted")
	}
}
func llvmFixture(t *testing.T, name string) []byte {
	t.Helper()
	doc := map[string]any{"type": "llvm.coverage.json.export", "version": "2.0.1", "data": []any{map[string]any{"files": []any{map[string]any{"filename": name, "summary": map[string]any{"lines": map[string]int{"count": 2, "covered": 1}, "functions": map[string]int{"count": 1, "covered": 1}, "regions": map[string]int{"count": 2, "covered": 1}}, "segments": []any{[]any{1, 1, 1, true, true, false}, []any{1, 4, 0, true, true, false}, []any{2, 4, 0, false, false, false}}}}}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestLLVMMeasuredSummaryAndSegments(t *testing.T) {
	root, path := source(t, "x.rs", "abc\nxyz\n")
	files, err := parseLLVM(llvmFixture(t, path), options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Metrics["lines"].Covered != 1 || files[0].Metrics["lines"].Total != 2 {
		t.Fatal(files)
	}
	if _, ok := files[0].Metrics["branches"]; ok {
		t.Fatal("invented branch metric")
	}
	lines, err := renderLines(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lines[0].Markup), `class="fc"`) || !strings.Contains(string(lines[1].Markup), `class="nc"`) {
		t.Fatal(lines)
	}
	if _, err := parseLLVM(llvmFixture(t, "../x.rs"), options{Root: root}); err == nil {
		t.Fatal("traversal accepted")
	}
	files, err = parseLLVM(llvmFixture(t, "/unrelated/dependency.rs"), options{Root: root})
	if err != nil || len(files) != 0 {
		t.Fatal("absolute dependencies should be excluded", err)
	}
}
func TestLLVMSchemaAndSummaryOnly(t *testing.T) {
	root, path := source(t, "x.rs", "abc\nxyz\n")
	raw := llvmFixture(t, path)
	modern := strings.Replace(string(raw), `"version":"2.0.1"`, `"version":"3.1.0"`, 1)
	if _, err := parseLLVM([]byte(modern), options{Root: root}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(raw), `"version":"2.0.1"`, `"version":"99.0.0"`, 1), strings.Replace(string(raw), `"segments":`, `"ignored_segments":`, 1)} {
		if _, err := parseLLVM([]byte(bad), options{Root: root}); err == nil {
			t.Fatal("unsupported schema/summary-only accepted")
		}
	}
}

func TestLLVMRejectsNullCountersAndSegments(t *testing.T) {
	root, path := source(t, "x.rs", "abc\nxyz\n")
	raw := string(llvmFixture(t, path))
	for _, bad := range []string{strings.Replace(raw, `"covered":1`, `"covered":null`, 1), strings.Replace(raw, `"covered":1`, `"missing":1`, 1), strings.Replace(raw, `[1,1,1,true,true,false]`, `[1,1,null,true,true,false]`, 1), strings.Replace(raw, `[1,1,1,true,true,false]`, `[1,1,1,null,true,false]`, 1), strings.Replace(raw, `[2,4,0,false,false,false]`, `[999,4,0,false,false,false]`, 1)} {
		if _, err := parseLLVM([]byte(bad), options{Root: root}); err == nil {
			t.Fatalf("accepted malformed export %s", bad)
		}
	}
}

func TestUTF8CRLFAndPartialRanges(t *testing.T) {
	file := sourceFile{Bytes: []byte("\tα<&\r\nnext\n"), Ranges: []sourceRange{{position{1, 1}, position{1, 7}, 1}, {position{1, 4}, position{1, 6}, 0}}}
	lines, err := renderLines(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(lines[0].Markup)
	if !strings.Contains(text, `class="pc"`) || !strings.Contains(text, "α") || !strings.Contains(text, "&lt;&amp;") {
		t.Fatal(text)
	}
}

func TestSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside, path := source(t, "private.go", "secret")
	_ = outside
	if err := os.Symlink(path, filepath.Join(root, "alias.go")); err != nil {
		t.Skip("symlink unavailable")
	}
	if _, err := loadSource(root, "alias.go", ""); err == nil {
		t.Fatal("symlink escaped source root")
	}
}
func TestOutputSafetyAndManifest(t *testing.T) {
	root, _ := source(t, "x.go", "abc\n")
	input := filepath.Join(t.TempDir(), "coverage.out")
	raw := []byte("mode: set\nx.go:1.1,1.4 1 1\n")
	if err := os.WriteFile(input, raw, 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "report")
	o := options{Engine: "Go / cover", Root: root, Input: input, Output: output}
	if err := generate(o); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["execution_owned_by"] != "external" || manifest["fuzzing_feedback"] != false {
		t.Fatal(manifest)
	}
	if _, ok := manifest["replay"]; ok {
		t.Fatal("invented replay statuses")
	}
	if err := generate(o); err == nil {
		t.Fatal("overwrote report")
	}
	o.Output = filepath.Join(root, "nested")
	if err := generate(o); err == nil {
		t.Fatal("source directory overlap accepted")
	}
}
