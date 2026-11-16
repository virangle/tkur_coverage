// SPDX-License-Identifier: EPL-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUTF16Columns(t *testing.T) {
	data := []byte("\tЖ😀abc\r\n")
	for _, c := range []struct{ u, b int }{{0, 1}, {1, 2}, {2, 4}, {4, 8}, {7, 11}} {
		v := c.u
		p, e := jsPosition(data, jsPos{1, &v})
		if e != nil || p.Column != c.b {
			t.Fatalf("%+v %v", p, e)
		}
	}
	u := 3
	if _, e := jsPosition(data, jsPos{1, &u}); e == nil {
		t.Fatal("split surrogate accepted")
	}
}
func TestDuplicateJSON(t *testing.T) {
	for _, s := range []string{`{"x":1,"x":2}`, `{"a":{"b":0,"b":1}}`, `{} {}`, `[1,]`} {
		if uniqueJSON([]byte(s)) == nil {
			t.Fatal(s)
		}
	}
}
func TestCoverletPhysicalAndProducerLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "t.cs")
	os.WriteFile(path, []byte("class C{}\n"), 0600)
	m := func(hit int) any {
		return map[string]any{"Lines": map[string]int{"1": hit}, "Branches": []any{map[string]any{"Line": 1, "Offset": 0, "EndOffset": 2, "Path": 0, "Ordinal": 0, "Hits": hit}, map[string]any{"Line": 1, "Offset": 0, "EndOffset": 3, "Path": 1, "Ordinal": 1, "Hits": 0, "IsReachable": false}}}
	}
	raw, _ := json.Marshal(map[string]any{"target.dll": map[string]any{path: map[string]any{"C": map[string]any{"A": m(1), "B": m(0)}}}})
	f, e := parseCoverlet(raw, options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	if f[0].Metrics["lines"].Total != 1 || f[0].Metrics["lines"].Covered != 1 || f[0].ProducerMetrics["lines"].Total != 2 || f[0].Metrics["branches"].Total != 2 || f[0].Metrics["branches"].Covered != 1 {
		t.Fatalf("%+v", f[0])
	}
	out, e := renderLines(f[0])
	if e != nil || !strings.Contains(string(out[0].Markup), `class="pc"`) {
		t.Fatalf("%+v %v", out, e)
	}
}
func TestIstanbulIDsAndLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "t.js")
	os.WriteFile(path, []byte("let x=1; x++;\n"), 0600)
	loc := func(a, b int) any {
		return map[string]any{"start": map[string]int{"line": 1, "column": a}, "end": map[string]int{"line": 1, "column": b}}
	}
	v := map[string]any{"path": path, "statementMap": map[string]any{"0": loc(0, 8), "1": loc(9, 13)}, "s": map[string]int{"0": 1, "1": 0}, "fnMap": map[string]any{}, "f": map[string]int{}, "branchMap": map[string]any{"0": map[string]any{"locations": []any{loc(0, 8), loc(9, 13)}}}, "b": map[string]any{"0": []int{1, 0}}}
	raw, _ := json.Marshal(map[string]any{path: v})
	f, e := parseIstanbul(raw, options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	if f[0].Metrics["lines"].Covered != 1 || f[0].Metrics["lines"].Total != 1 || f[0].Metrics["statements"].Covered != 1 || f[0].Metrics["statements"].Total != 2 || f[0].Metrics["branches"].Total != 2 {
		t.Fatal(f[0].Metrics)
	}
	v["s"] = map[string]int{"wrong": 0}
	raw, _ = json.Marshal(map[string]any{path: v})
	if _, e = parseIstanbul(raw, options{Root: root}); e == nil {
		t.Fatal("bad IDs")
	}
}
