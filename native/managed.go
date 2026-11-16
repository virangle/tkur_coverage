// SPDX-License-Identifier: EPL-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Reject duplicate keys before typed decoding: encoding/json otherwise silently
// replaces counters, identities, source paths and provenance fields.
func uniqueJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("JSON must be valid UTF8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 128 {
			return errors.New("JSON nesting limit")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return errors.New("duplicate/invalid JSON object key")
				}
				seen[s] = true
				if e = value(depth + 1); e != nil {
					return e
				}
			}
			_, e = d.Token()
			return e
		case json.Delim('['):
			for d.More() {
				if e = value(depth + 1); e != nil {
					return e
				}
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON document")
	}
	return nil
}
func counter(raw json.RawMessage) (int64, error) {
	var n int64
	if len(raw) == 0 || string(raw) == "null" {
		return 0, errors.New("missing integer counter")
	}
	if e := json.Unmarshal(raw, &n); e != nil {
		return 0, e
	}
	if n < 0 || n > 9007199254740991 {
		return 0, errors.New("counter outside nonnegative safe integer range")
	}
	return n, nil
}

type jsPos struct {
	Line   int  `json:"line"`
	Column *int `json:"column"`
}
type jsLoc struct {
	Start jsPos `json:"start"`
	End   jsPos `json:"end"`
}
type jsFunction struct {
	Loc jsLoc `json:"loc"`
}
type jsBranch struct {
	Type      string  `json:"type"`
	Locations []jsLoc `json:"locations"`
}
type jsFile struct {
	Path         string                       `json:"path"`
	StatementMap map[string]jsLoc             `json:"statementMap"`
	FnMap        map[string]jsFunction        `json:"fnMap"`
	BranchMap    map[string]jsBranch          `json:"branchMap"`
	S            map[string]json.RawMessage   `json:"s"`
	F            map[string]json.RawMessage   `json:"f"`
	B            map[string][]json.RawMessage `json:"b"`
}

// JS columns count UTF-16 code units, not UTF-8 bytes or visual tab widths.
func jsPosition(data []byte, p jsPos) (position, error) {
	lines := lineOffsets(data)
	if p.Line < 1 || p.Line > len(lines) || p.Column == nil || *p.Column < 0 {
		return position{}, errors.New("invalid JS source coordinate")
	}
	start := lines[p.Line-1]
	end := len(data)
	if p.Line < len(lines) {
		end = lines[p.Line] - 1
	}
	units := 0
	for at := start; at <= end; {
		if units == *p.Column {
			return position{p.Line, at - start + 1}, nil
		}
		if at == end {
			break
		}
		r, n := utf8.DecodeRune(data[at:end])
		if r == utf8.RuneError && n == 1 {
			return position{}, errors.New("invalid UTF8 source")
		}
		u := 1
		if r > 0xffff {
			u = 2
		}
		if units+u > *p.Column {
			return position{}, errors.New("JS column splits UTF16 surrogate pair")
		}
		units += u
		at += n
	}
	return position{}, errors.New("JS column outside source")
}
func jsRange(f sourceFile, l jsLoc, n int64) (sourceRange, error) {
	a, e := jsPosition(f.Bytes, l.Start)
	if e != nil {
		return sourceRange{}, e
	}
	b, e := jsPosition(f.Bytes, l.End)
	if e != nil {
		return sourceRange{}, e
	}
	return sourceRange{a, b, n}, nil
}
func parseIstanbul(raw []byte, o options) ([]sourceFile, error) {
	if e := uniqueJSON(raw); e != nil {
		return nil, e
	}
	var doc map[string]jsFile
	if e := json.Unmarshal(raw, &doc); e != nil {
		return nil, e
	}
	var files []sourceFile
	seen := map[string]bool{}
	for key, v := range doc {
		if key != v.Path {
			return nil, errors.New("Istanbul key and path differ")
		}
		f, e := loadSource(o.Root, v.Path, "")
		if e != nil {
			return nil, e
		}
		if !utf8.Valid(f.Bytes) {
			return nil, errors.New("source must be valid UTF8")
		}
		if seen[f.Name] {
			return nil, errors.New("source aliases rejected")
		}
		seen[f.Name] = true
		if v.S == nil || v.F == nil || v.B == nil || v.StatementMap == nil || v.FnMap == nil || v.BranchMap == nil {
			return nil, errors.New("incomplete Istanbul coverage object")
		}
		if len(v.S) != len(v.StatementMap) || len(v.F) != len(v.FnMap) || len(v.B) != len(v.BranchMap) {
			return nil, errors.New("Istanbul map/counter IDs differ")
		}
		lines := map[int]int64{}
		var sh, fh, bh, bt int64
		for id, r := range v.S {
			l, ok := v.StatementMap[id]
			if !ok {
				return nil, errors.New("missing statement location")
			}
			n, e := counter(r)
			if e != nil {
				return nil, e
			}
			rg, e := jsRange(f, l, n)
			if e != nil {
				return nil, e
			}
			f.Ranges = append(f.Ranges, rg)
			if n > 0 {
				sh++
			}
			if old, ok := lines[l.Start.Line]; !ok || n > old {
				lines[l.Start.Line] = n
			}
		}
		for id, r := range v.F {
			l, ok := v.FnMap[id]
			if !ok {
				return nil, errors.New("missing function location")
			}
			n, e := counter(r)
			if e != nil {
				return nil, e
			}
			if _, e = jsRange(f, l.Loc, n); e != nil {
				return nil, e
			}
			if n > 0 {
				fh++
			}
		}
		for id, arms := range v.B {
			b, ok := v.BranchMap[id]
			if !ok || arms == nil || len(arms) != len(b.Locations) {
				return nil, errors.New("branch arm/location mismatch")
			}
			for i, r := range arms {
				n, e := counter(r)
				if e != nil {
					return nil, e
				}
				l := b.Locations[i]
				// Istanbul records the implicit else of an if without else as
				// {start:{},end:{}}. It is still a real branch slot, but has
				// no source span: never invent one.
				implicit := b.Type == "if" && i == 1 && len(arms) == 2 && l.Start.Line == 0 && l.End.Line == 0 && l.Start.Column == nil && l.End.Column == nil
				if !implicit {
					rg, e := jsRange(f, l, n)
					if e != nil {
						return nil, e
					}
					f.Ranges = append(f.Ranges, rg)
				}
				bt++
				if n > 0 {
					bh++
				}
			}
		}
		var lh int64
		for _, n := range lines {
			if n > 0 {
				lh++
			}
		}
		f.Metrics["statements"], _ = measured(sh, int64(len(v.S)))
		f.Metrics["functions"], _ = measured(fh, int64(len(v.F)))
		f.Metrics["branches"], _ = measured(bh, bt)
		f.Metrics["lines"], _ = measured(lh, int64(len(lines)))
		if e = validateRanges(&f); e != nil {
			return nil, e
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

type netBranch struct {
	Line, Offset, EndOffset, Path int
	Ordinal                       uint64
	Hits                          json.RawMessage
	IsReachable                   *bool
}
type netMethod struct {
	Lines    map[string]json.RawMessage
	Branches []netBranch
}
type netModules map[string]map[string]map[string]map[string]netMethod

func parseCoverlet(raw []byte, o options) ([]sourceFile, error) {
	if e := uniqueJSON(raw); e != nil {
		return nil, e
	}
	var doc netModules
	if e := json.Unmarshal(raw, &doc); e != nil {
		return nil, e
	}
	fileMap := map[string]*sourceFile{}
	lineMap := map[string]map[int]int64{}
	branchIDs := map[string]bool{}
	for module, documents := range doc {
		aliases := map[string]string{}
		for path, classes := range documents {
			f, e := loadSource(o.Root, path, "")
			if e != nil {
				return nil, e
			}
			if old, ok := aliases[f.Name]; ok && old != path {
				return nil, errors.New("Coverlet document source aliases rejected")
			}
			aliases[f.Name] = path
			if !utf8.Valid(f.Bytes) {
				return nil, errors.New("source must be valid UTF8")
			}
			prev := fileMap[f.Name]
			if prev == nil {
				prev = &f
				fileMap[f.Name] = prev
				lineMap[f.Name] = map[int]int64{}
				prev.ProducerMetrics = map[string]metric{}
			}
			for class, methods := range classes {
				for method, m := range methods {
					if m.Lines == nil || m.Branches == nil {
						return nil, errors.New("Coverlet method requires Lines and Branches")
					}
					var lh int64
					for line, r := range m.Lines {
						ln, e := strconv.Atoi(line)
						if e != nil || ln < 1 || strconv.Itoa(ln) != line {
							return nil, errors.New("invalid Coverlet source line")
						}
						n, e := counter(r)
						if e != nil {
							return nil, e
						}
						if n > 0 {
							lh++
						}
						old, ok := lineMap[f.Name][ln]
						if !ok || n > old {
							lineMap[f.Name][ln] = n
						}
					}
					metricLine, _ := measured(lh, int64(len(m.Lines)))
					sum, e := addMetric(prev.ProducerMetrics["lines"], metricLine)
					if e != nil {
						return nil, e
					}
					prev.ProducerMetrics["lines"] = sum
					for _, b := range m.Branches {
						if b.Line < 1 || b.Offset < 0 || b.EndOffset < 0 || b.Path < 0 || b.Ordinal > math.MaxInt32 {
							return nil, errors.New("invalid Coverlet branch coordinates")
						}
						if _, e := wholeLine(f, b.Line, 0); e != nil {
							return nil, e
						}
						n, e := counter(b.Hits)
						if e != nil {
							return nil, e
						}
						identityData, _ := json.Marshal([]any{module, path, class, method, b.Line, b.Offset, b.EndOffset, b.Path, b.Ordinal})
						identity := string(identityData)
						if branchIDs[identity] {
							return nil, errors.New("duplicate Coverlet branch identity")
						}
						branchIDs[identity] = true
						if b.IsReachable != nil && !*b.IsReachable {
							continue
						}
						one, _ := measured(0, 1)
						if n > 0 {
							one, _ = measured(1, 1)
						}
						next, e := addMetric(prev.Metrics["branches"], one)
						if e != nil {
							return nil, e
						}
						prev.Metrics["branches"] = next
						// A missed branch makes a measured line partial, not an invented source arm.
						if n == 0 {
							rg, e := wholeLine(f, b.Line, 0)
							if e != nil {
								return nil, e
							}
							prev.Ranges = append(prev.Ranges, rg)
						}
					}
				}
			}
		}
	}
	var files []sourceFile
	for name, f := range fileMap {
		var hit int64
		for ln, n := range lineMap[name] {
			if n > 0 {
				hit++
			}
			r, e := wholeLine(*f, ln, n)
			if e != nil {
				return nil, e
			}
			f.Ranges = append(f.Ranges, r)
		}
		f.Metrics["lines"], _ = measured(hit, int64(len(lineMap[name])))
		if _, ok := f.Metrics["branches"]; !ok {
			f.Metrics["branches"], _ = measured(0, 0)
		}
		if e := validateRanges(f); e != nil {
			return nil, e
		}
		files = append(files, *f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}
func wholeLine(f sourceFile, line int, n int64) (sourceRange, error) {
	ls := lineOffsets(f.Bytes)
	if line < 1 || line > len(ls) {
		return sourceRange{}, errors.New("Coverlet line outside source")
	}
	end := len(f.Bytes)
	if line < len(ls) {
		end = ls[line] - 1
	}
	return sourceRange{position{line, 1}, position{line, end - ls[line-1] + 1}, n}, nil
}
func managedSemantics(o options) map[string]string {
	if strings.HasPrefix(o.Engine, ".NET") {
		return map[string]string{"lines": "derived unique executable physical source lines; maximum per-method hit count", "producer_lines": "Coverlet method-line entries, not deduplicated physical lines", "branches": "Coverlet reachable BranchInfo slots, exact module/document/class/method/IL tuple identity", "source_view": "measured whole source lines; missed reachable branches mark partial lines; no source columns present"}
	}
	if o.Origin == "v8" {
		return map[string]string{"statements": "V8-derived line records, not AST statements", "branches": "V8-derived block ranges, not AST branch arms", "lines": "Istanbul statement-start aggregation of V8-derived line records", "source_view": "externally converted V8 ranges with UTF16 columns"}
	}
	return map[string]string{"statements": "Istanbul instrumented AST statement IDs", "branches": "Istanbul measured branch-arm slots", "lines": "unique statement start lines, maximum statement hit count", "functions": "Istanbul function IDs", "source_view": "measured statement and branch-arm ranges; UTF16 columns converted to UTF8 byte intervals"}
}
func managedRaw(o options) string {
	if strings.HasPrefix(o.Engine, ".NET") {
		return "raw/coverlet.json"
	}
	return "raw/istanbul.json"
}
