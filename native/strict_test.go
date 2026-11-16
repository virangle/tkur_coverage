// SPDX-License-Identifier: EPL-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictManagedFields(t *testing.T) {
	for _, raw := range []string{`{"Line":1,"Offset":0,"EndOffset":1,"Path":0,"Hits":1}`, `{"Line":1,"Offset":null,"EndOffset":1,"Path":0,"Ordinal":0,"Hits":1}`, `{"Line":1,"Offset":0,"EndOffset":1,"Path":0,"Ordinal":0,"Hits":1,"hits":2}`, `{"Line":1,"Offset":0,"EndOffset":1,"Path":0,"Ordinal":0,"Hits":1,"IsReachable":null}`} {
		var b netBranch
		if e := json.Unmarshal([]byte(raw), &b); e == nil {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{`{"Lines":{},"lines":{},"Branches":[]}`, `{"lines":{},"Branches":[]}`} {
		var m netMethod
		if e := json.Unmarshal([]byte(raw), &m); e == nil {
			t.Fatal(raw)
		}
	}
	if uniqueJSON([]byte{'"', 255, '"'}) == nil {
		t.Fatal("invalid utf8")
	}
}
func TestCoveredBranchBoundsAndAliases(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "t.cs")
	os.WriteFile(p, []byte("C();\n"), 0600)
	m := map[string]any{"Lines": map[string]int{"1": 1}, "Branches": []any{map[string]any{"Line": 99, "Offset": 0, "EndOffset": 2, "Path": 0, "Ordinal": 0, "Hits": 1}}}
	documents := map[string]any{p: map[string]any{"C": map[string]any{"M": m}}}
	raw, _ := json.Marshal(map[string]any{"T": documents})
	if _, e := parseCoverlet(raw, options{Root: root}); e == nil {
		t.Fatal("covered branch bounds")
	}
	m["Branches"] = []any{}
	documents["t.cs"] = map[string]any{"C": map[string]any{"N": m}}
	raw, _ = json.Marshal(map[string]any{"T": documents})
	if _, e := parseCoverlet(raw, options{Root: root}); e == nil {
		t.Fatal("alias")
	}
}
func TestImplicitElseHasNoInventedRange(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.js")
	os.WriteFile(p, []byte("if(x)f();\n"), 0600)
	loc := map[string]any{"start": map[string]int{"line": 1, "column": 0}, "end": map[string]int{"line": 1, "column": 9}}
	v := map[string]any{"path": p, "statementMap": map[string]any{}, "s": map[string]int{}, "fnMap": map[string]any{}, "f": map[string]int{}, "branchMap": map[string]any{"0": map[string]any{"type": "if", "locations": []any{loc, map[string]any{"start": map[string]int{}, "end": map[string]int{}}}}}, "b": map[string]any{"0": []int{1, 0}}}
	raw, _ := json.Marshal(map[string]any{p: v})
	f, e := parseIstanbul(raw, options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	if f[0].Metrics["branches"].Total != 2 || len(f[0].Ranges) != 1 {
		t.Fatal(f[0])
	}
	raw = []byte(strings.Replace(string(raw), `"type":"if"`, `"type":"other"`, 1))
	if _, e = parseIstanbul(raw, options{Root: root}); e == nil {
		t.Fatal("invalid missing location")
	}
}
