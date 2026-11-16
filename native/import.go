// SPDX-License-Identifier: EPL-2.0
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type llvmCounter struct {
	Count   int64 `json:"count"`
	Covered int64 `json:"covered"`
}

func (c *llvmCounter) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, name := range []string{"count", "covered"} {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			return fmt.Errorf("LLVM counter requires integer %s", name)
		}
	}
	if err := json.Unmarshal(fields["count"], &c.Count); err != nil {
		return err
	}
	return json.Unmarshal(fields["covered"], &c.Covered)
}

type llvmDocument struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Data    []struct {
		Files []struct {
			Filename string                 `json:"filename"`
			Summary  map[string]llvmCounter `json:"summary"`
			Segments [][]json.RawMessage    `json:"segments"`
		} `json:"files"`
	} `json:"data"`
}

func parseLLVM(raw []byte, o options) ([]sourceFile, error) {
	var doc llvmDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.Type != "llvm.coverage.json.export" || (!strings.HasPrefix(doc.Version, "2.") && !strings.HasPrefix(doc.Version, "3.")) {
		return nil, errors.New("expected LLVM coverage export JSON version 2.x or 3.x")
	}
	var files []sourceFile
	seen := map[string]bool{}
	for _, object := range doc.Data {
		for _, entry := range object.Files {
			// Ordinary absolute dependency/runtime sources outside the selected crate
			// are excluded, not opened. Malformed/traversal paths remain fatal errors.
			for _, part := range strings.Split(entry.Filename, "/") {
				if part == ".." {
					return nil, errors.New("LLVM source path traversal rejected")
				}
			}
			if strings.Contains(entry.Filename, "\\") {
				return nil, errors.New("invalid LLVM source path")
			}
			if filepath.IsAbs(entry.Filename) && !within(o.Root, filepath.Clean(entry.Filename)) {
				continue
			}
			file, err := loadSource(o.Root, entry.Filename, "")
			if err != nil {
				return nil, err
			}
			if seen[file.Name] {
				return nil, errors.New("duplicate source across LLVM objects; export a unified profile")
			}
			seen[file.Name] = true
			for _, name := range []string{"lines", "branches", "functions", "regions"} {
				if value, ok := entry.Summary[name]; ok {
					m, err := measured(value.Covered, value.Count)
					if err != nil {
						return nil, err
					}
					file.Metrics[name] = m
				}
			}
			if len(file.Metrics) == 0 {
				return nil, errors.New("LLVM per-file measured summary is missing")
			}
			if len(entry.Segments) == 0 {
				return nil, errors.New("LLVM source segments missing; do not use -summary-only")
			}
			type segment struct {
				at            position
				count         int64
				hasCount, gap bool
			}
			var segments []segment
			sourceOffsets := lineOffsets(file.Bytes)
			for _, values := range entry.Segments {
				for _, value := range values {
					if string(value) == "null" {
						return nil, errors.New("null LLVM segment value")
					}
				}
				if len(values) < 5 || len(values) > 6 {
					return nil, errors.New("invalid LLVM coverage segment")
				}
				var s segment
				if err := json.Unmarshal(values[0], &s.at.Line); err != nil {
					return nil, err
				}
				if err := json.Unmarshal(values[1], &s.at.Column); err != nil {
					return nil, err
				}
				if err := json.Unmarshal(values[2], &s.count); err != nil {
					return nil, err
				}
				if err := json.Unmarshal(values[3], &s.hasCount); err != nil {
					return nil, err
				}
				var regionEntry bool
				if err := json.Unmarshal(values[4], &regionEntry); err != nil {
					return nil, err
				}
				if len(values) == 6 {
					if err := json.Unmarshal(values[5], &s.gap); err != nil {
						return nil, err
					}
				}
				if s.at.Line < 1 || s.at.Column < 1 || s.count < 0 {
					return nil, errors.New("invalid LLVM segment coordinate/count")
				}
				if _, err := offset(file.Bytes, sourceOffsets, s.at); err != nil {
					return nil, err
				}
				if len(segments) > 0 {
					last := segments[len(segments)-1]
					if s.at.Line < last.at.Line || (s.at.Line == last.at.Line && s.at.Column < last.at.Column) {
						return nil, errors.New("unordered LLVM segments")
					}
				}
				segments = append(segments, s)
			}
			for i := 0; i+1 < len(segments); i++ {
				s := segments[i]
				if s.hasCount && !s.gap {
					file.Ranges = append(file.Ranges, sourceRange{s.at, segments[i+1].at, s.count})
				}
			}
			if err := validateRanges(&file); err != nil {
				return nil, err
			}
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

// The native Go cover format is parsed independently using only the standard
// library; weighted NumStmt and duplicate merge semantics follow go tool cover.
// No Go/x-tools source is copied or vendored.
var goLine = regexp.MustCompile(`^(.+):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+) ([0-9]+) ([0-9]+)$`)

type goBlock struct {
	sourceRange
	NumStmt int64
}

func parseGo(raw []byte, o options) ([]sourceFile, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		return nil, errors.New("empty Go profile")
	}
	mode := strings.TrimSpace(scanner.Text())
	if mode != "mode: set" && mode != "mode: count" && mode != "mode: atomic" {
		return nil, errors.New("unsupported Go coverage mode")
	}
	blocks := map[string]map[string]goBlock{}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		fields := goLine.FindStringSubmatch(line)
		if fields == nil {
			return nil, fmt.Errorf("malformed Go coverage line: %q", line)
		}
		values := make([]int64, 6)
		for i := range values {
			n, err := strconv.ParseInt(fields[i+2], 10, 64)
			if err != nil {
				return nil, err
			}
			values[i] = n
		}
		for _, v := range values[:4] {
			if v < 1 || v > math.MaxInt32 {
				return nil, errors.New("Go coordinate out of range")
			}
		}
		b := goBlock{sourceRange{position{int(values[0]), int(values[1])}, position{int(values[2]), int(values[3])}, values[5]}, values[4]}
		if mode == "mode: set" && b.Count > 1 {
			return nil, errors.New("set profile count must be 0 or 1")
		}
		name, err := resolveSource(o.Root, fields[1], o.Module)
		if err != nil {
			return nil, err
		}
		if blocks[name] == nil {
			blocks[name] = map[string]goBlock{}
		}
		key := fmt.Sprintf("%d.%d,%d.%d", b.Start.Line, b.Start.Column, b.End.Line, b.End.Column)
		if prev, ok := blocks[name][key]; ok {
			if prev.NumStmt != b.NumStmt {
				return nil, errors.New("duplicate Go block has inconsistent NumStmt")
			}
			if mode == "mode: set" {
				if prev.Count > b.Count {
					b.Count = prev.Count
				}
			} else {
				if b.Count > math.MaxInt64-prev.Count {
					return nil, errors.New("Go counter overflow")
				}
				b.Count += prev.Count
			}
		}
		blocks[name][key] = b
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	var files []sourceFile
	for name, entries := range blocks {
		file, err := loadSource(o.Root, name, "")
		if err != nil {
			return nil, err
		}
		var covered, total int64
		for _, b := range entries {
			if total > math.MaxInt64-b.NumStmt {
				return nil, errors.New("Go statement total overflow")
			}
			total += b.NumStmt
			if b.Count > 0 {
				covered += b.NumStmt
			}
			file.Ranges = append(file.Ranges, b.sourceRange)
		}
		sort.Slice(file.Ranges, func(i, j int) bool {
			a, b := file.Ranges[i], file.Ranges[j]
			if a.Start.Line != b.Start.Line {
				return a.Start.Line < b.Start.Line
			}
			return a.Start.Column < b.Start.Column
		})
		m, err := measured(covered, total)
		if err != nil {
			return nil, err
		}
		file.Metrics["statements"] = m
		if err := validateRanges(&file); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}
