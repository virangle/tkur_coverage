// SPDX-License-Identifier: EPL-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const lab = `Испытательная лаборатория ООО "ТЕХКОНСУР"`

var keys = []string{"statements", "lines", "branches", "functions", "regions"}
var labels = map[string]string{"statements": "Операторы", "lines": "Строки", "branches": "Ветви", "functions": "Функции", "regions": "Регионы"}

type column struct{ Name, Label string }
type cell struct {
	Missed, Total int64
	Percent       string
}
type row struct {
	Label, Link string
	Cells       []cell
}
type page struct {
	Title, Assets, Home, Engine, Timestamp string
	Columns                                []column
	Rows                                   []row
	Totals                                 []cell
	Lines                                  []sourceLine
}
type sourceLine struct {
	Number int
	Markup template.HTML
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><link rel="stylesheet" href="{{.Assets}}/report.css"><link rel="stylesheet" href="{{.Assets}}/adapter.css"><link rel="stylesheet" href="{{.Assets}}/native.css"><link rel="icon" href="{{.Assets}}/report.svg" type="image/svg+xml"></head><body><div class="tkur-header"><img class="tkur-logo" src="{{.Assets}}/tkur.svg" alt="Логотип ТЕХКОНСУР"><span class="tkur-wordmark">Испытательная лаборатория ООО &#34;ТЕХКОНСУР&#34;</span><div class="tkur-product"><span class="tkur-product-name">tkur_coverage 0.2.0</span><span class="tkur-product-description">ОТЧЁТ О ПОКРЫТИИ КОДА</span><span class="tkur-created">Сформирован: {{.Timestamp}}</span></div></div><div class="breadcrumb" id="breadcrumb">{{if .Home}}<a class="el_report" href="{{.Home}}">Покрытие кода</a> &gt; <span class="el_source">{{.Title}}</span>{{else}}<span class="info"><a class="el_source" href="source/index.html">Исходные файлы</a></span><span class="el_report">Покрытие кода</span>{{end}}</div><h1>{{.Title}}</h1><div class="tkur-legend"><span class="tkur-covered">Покрыто</span><span class="tkur-partial">Частично</span><span class="tkur-missed">Не покрыто</span></div>{{if .Lines}}<div class="native-source"><table class="native-code"><tbody>{{range .Lines}}<tr id="L{{.Number}}"><td class="native-number"><a href="#L{{.Number}}">{{.Number}}</a></td><td><pre>{{.Markup}}</pre></td></tr>{{end}}</tbody></table></div>{{else}}<div class="native-table"><table class="coverage" cellspacing="0"><thead><tr><td>Элемент</td>{{range .Columns}}<td class="ctr1">{{.Label}}: пропущено</td><td class="ctr2">Всего</td><td class="ctr2">Покрытие</td>{{end}}</tr></thead><tfoot><tr><td>Всего</td>{{range .Totals}}<td class="ctr1">{{.Missed}}</td><td class="ctr2">{{.Total}}</td><td class="ctr2">{{.Percent}}</td>{{end}}</tr></tfoot><tbody>{{range .Rows}}<tr><td><a class="el_source" href="{{.Link}}">{{.Label}}</a></td>{{range .Cells}}<td class="ctr1">{{.Missed}}</td><td class="ctr2">{{.Total}}</td><td class="ctr2">{{.Percent}}</td>{{end}}</tr>{{end}}</tbody></table></div>{{end}}<div class="footer"><span class="right">Создано с помощью {{.Engine}}</span><span class="tkur-owner">Испытательная лаборатория ООО &#34;ТЕХКОНСУР&#34; · tkur_coverage</span></div></body></html>`))

func cells(metrics map[string]metric, cols []column) []cell {
	var out []cell
	for _, c := range cols {
		m, ok := metrics[c.Name]
		p := "н/д"
		if ok && m.Percent != nil {
			p = fmt.Sprintf("%.2f%%", *m.Percent)
		}
		out = append(out, cell{m.Total - m.Covered, m.Total, p})
	}
	return out
}
func writePage(name string, p page) error {
	var b bytes.Buffer
	if err := pageTemplate.Execute(&b, p); err != nil {
		return err
	}
	return os.WriteFile(name, b.Bytes(), 0644)
}

// Source markup contains only fixed generated span tags plus escaped source
// bytes. Profile strings and source filenames are never treated as trusted HTML.
func renderLines(file sourceFile) ([]sourceLine, error) {
	offsets := lineOffsets(file.Bytes)
	type event struct {
		at        int
		hit, miss int
	}
	events := []event{}
	for _, r := range file.Ranges {
		a, err := offset(file.Bytes, offsets, r.Start)
		if err != nil {
			return nil, err
		}
		b, err := offset(file.Bytes, offsets, r.End)
		if err != nil {
			return nil, err
		}
		if a == b {
			continue
		}
		hit, miss := 0, 0
		if r.Count > 0 {
			hit = 1
		} else {
			miss = 1
		}
		events = append(events, event{a, hit, miss}, event{b, -hit, -miss})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].at < events[j].at })
	result := []sourceLine{}
	hit, miss, ei := 0, 0, 0
	for line, start := range offsets {
		end := len(file.Bytes)
		if line+1 < len(offsets) {
			end = offsets[line+1] - 1
		}
		if start == len(file.Bytes) && start > 0 {
			break
		}
		var out strings.Builder
		at := start
		for at < end {
			for ei < len(events) && events[ei].at <= at {
				hit += events[ei].hit
				miss += events[ei].miss
				ei++
			}
			next := end
			if ei < len(events) && events[ei].at < next {
				next = events[ei].at
			}
			class := ""
			if hit > 0 && miss > 0 {
				class = "pc"
			} else if hit > 0 {
				class = "fc"
			} else if miss > 0 {
				class = "nc"
			}
			if class != "" {
				out.WriteString(`<span class="` + class + `">`)
			}
			out.WriteString(html.EscapeString(string(file.Bytes[at:next])))
			if class != "" {
				out.WriteString("</span>")
			}
			at = next
		}
		result = append(result, sourceLine{line + 1, template.HTML(out.String())})
	}
	return result, nil
}

func safeOutput(o options) (string, error) {
	output, err := filepath.Abs(o.Output)
	if err != nil {
		return "", err
	}
	// Resolve existing ancestors, including symlinks, before containment checks.
	ancestor := output
	var tail []string
	for {
		_, err = os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("invalid output parent")
		}
		tail = append(tail, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for i := len(tail) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, tail[i])
	}
	output = resolved
	if within(o.Root, output) || within(output, o.Root) {
		return "", errors.New("output and source-root must be disjoint")
	}
	for _, input := range []string{o.Input, o.Provenance} {
		if input != "" {
			abs, err := filepath.Abs(input)
			if err != nil {
				return "", err
			}
			if within(output, abs) {
				return "", errors.New("output must not contain input artifacts")
			}
		}
	}
	if entries, err := os.ReadDir(output); err == nil {
		if len(entries) > 0 {
			return "", errors.New("output directory must be new or empty")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return output, nil
}
func writeReport(o options, files []sourceFile, raw, provenance []byte) error {
	output, err := safeOutput(o)
	if err != nil {
		return err
	}
	totals := map[string]metric{}
	for _, f := range files {
		for k, m := range f.Metrics {
			next, err := addMetric(totals[k], m)
			if err != nil {
				return err
			}
			totals[k] = next
		}
	}
	var cols []column
	for _, k := range keys {
		if _, ok := totals[k]; ok {
			cols = append(cols, column{k, labels[k]})
		}
	}
	// Resolve all source ranges and templates before touching output.
	rendered := make([][]sourceLine, len(files))
	for i, f := range files {
		rendered[i], err = renderLines(f)
		if err != nil {
			return err
		}
	}
	for _, d := range []string{"assets", "source", "source-files", "raw"} {
		if err := os.MkdirAll(filepath.Join(output, d), 0755); err != nil {
			return err
		}
	}
	if err := fs.WalkDir(resources, "assets", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		data, e := resources.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(output, filepath.FromSlash(path)), data, 0644)
	}); err != nil {
		return err
	}
	rawName := "raw/llvm-export.json"
	if strings.HasPrefix(o.Engine, "Go") {
		rawName = "raw/coverage.out"
	}
	if err := os.WriteFile(filepath.Join(output, rawName), raw, 0644); err != nil {
		return err
	}
	if len(provenance) > 0 {
		if err := os.WriteFile(filepath.Join(output, "raw/provenance.json"), provenance, 0644); err != nil {
			return err
		}
	}
	now := time.Now()
	stamp := now.Format("02.01.2006 15:04:05 -0700")
	var rows []row
	for i := range files {
		f := &files[i]
		id := strconv.Itoa(i)
		// Neutral extension prevents imported source from becoming executable HTML
		// or an accidental Go package when test tools walk report artifacts.
		f.Snapshot = "source-files/" + id + ".txt"
		f.HTML = "source/" + id + ".html"
		if err := os.WriteFile(filepath.Join(output, f.Snapshot), f.Bytes, 0644); err != nil {
			return err
		}
		label, err := filepath.Rel(o.Root, f.Name)
		if err != nil {
			return err
		}
		rows = append(rows, row{filepath.ToSlash(label), f.HTML, cells(f.Metrics, cols)})
		if err := writePage(filepath.Join(output, f.HTML), page{Title: filepath.ToSlash(label), Assets: "../assets", Home: "../index.html", Engine: o.Engine, Timestamp: stamp, Lines: rendered[i]}); err != nil {
			return err
		}
	}
	base := page{Title: "Покрытие кода", Assets: "assets", Engine: o.Engine, Timestamp: stamp, Columns: cols, Rows: rows, Totals: cells(totals, cols)}
	if err := writePage(filepath.Join(output, "index.html"), base); err != nil {
		return err
	}
	sourceIndex := base
	sourceIndex.Title = "Файлы"
	sourceIndex.Assets = "../assets"
	sourceIndex.Home = "../index.html"
	sourceIndex.Rows = make([]row, len(rows))
	copy(sourceIndex.Rows, rows)
	for i := range sourceIndex.Rows {
		sourceIndex.Rows[i].Link = strings.TrimPrefix(sourceIndex.Rows[i].Link, "source/")
	}
	if err := writePage(filepath.Join(output, "source/index.html"), sourceIndex); err != nil {
		return err
	}
	meta := map[string]any{"schema_version": 2, "product": "tkur_coverage " + version, "engine": o.Engine, "generated_at": now.UTC().Format(time.RFC3339), "measurement": "imported-source-coverage", "measurement_origin": "imported-engine-output", "execution_owned_by": "external", "fuzzing_feedback": false, "source_root": o.Root, "metrics": totals, "source_snapshots": files, "raw_data": map[string]string{"path": rawName, "sha256": digest(raw)}}
	if strings.HasPrefix(o.Engine, "Go") {
		meta["metric_semantics"] = map[string]string{"statements": "Go NumStmt-weighted source blocks"}
	} else {
		meta["metric_semantics"] = map[string]string{"lines": "LLVM measured file summary", "source_view": "LLVM counted non-gap segment intervals; not inferred line hit counters"}
	}
	if len(provenance) > 0 {
		meta["provenance"] = map[string]string{"path": "raw/provenance.json", "sha256": digest(provenance)}
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "report.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println(filepath.Join(output, "index.html"))
	return nil
}
