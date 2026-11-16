// SPDX-License-Identifier: EPL-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type metric struct {
	Covered int64    `json:"covered"`
	Total   int64    `json:"total"`
	Percent *float64 `json:"percent"`
}
type position struct{ Line, Column int }
type sourceRange struct {
	Start, End position
	Count      int64
}
type sourceFile struct {
	Name     string            `json:"original"`
	Snapshot string            `json:"snapshot"`
	HTML     string            `json:"html"`
	SHA256   string            `json:"sha256"`
	Metrics  map[string]metric `json:"metrics"`
	Bytes    []byte            `json:"-"`
	Ranges   []sourceRange     `json:"-"`
}

func measured(hit, total int64) (metric, error) {
	if hit < 0 || total < 0 || hit > total {
		return metric{}, errors.New("invalid measured numerator/denominator")
	}
	m := metric{Covered: hit, Total: total}
	if total > 0 {
		v := math.Round(float64(hit)/float64(total)*10000) / 100
		m.Percent = &v
	}
	return m, nil
}
func addMetric(a, b metric) (metric, error) {
	if a.Total > math.MaxInt64-b.Total || a.Covered > math.MaxInt64-b.Covered {
		return metric{}, errors.New("metric integer overflow")
	}
	return measured(a.Covered+b.Covered, a.Total+b.Total)
}
func digest(data []byte) string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }
func readRegular(name string, limit int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", name)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file too large: %s", name)
	}
	return os.ReadFile(name)
}
func within(root, name string) bool {
	r, err := filepath.Rel(root, name)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator)) && !filepath.IsAbs(r)
}
func resolveSource(root, name, module string) (string, error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") {
		return "", errors.New("invalid source path")
	}
	// Reject traversal before cleaning, even if its final destination is inside root.
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", errors.New("source path traversal rejected")
		}
	}
	if module != "" && strings.HasPrefix(name, module+"/") {
		name = strings.TrimPrefix(name, module+"/")
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !within(root, path) {
		return "", fmt.Errorf("source outside --source-root: %s", name)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !within(root, resolved) {
		return "", fmt.Errorf("symlink escapes --source-root: %s", name)
	}
	// Reject source aliases to remove ambiguous path identities and TOCTOU symlink surprises.
	if resolved != path {
		return "", fmt.Errorf("symlink source paths are not accepted: %s", name)
	}
	return path, nil
}
func loadSource(root, name, module string) (sourceFile, error) {
	path, err := resolveSource(root, name, module)
	if err != nil {
		return sourceFile{}, err
	}
	data, err := readRegular(path, 16<<20)
	if err != nil {
		return sourceFile{}, err
	}
	return sourceFile{Name: path, Bytes: data, SHA256: digest(data), Metrics: map[string]metric{}}, nil
}
func lineOffsets(data []byte) []int {
	out := []int{0}
	for i, b := range data {
		if b == '\n' {
			out = append(out, i+1)
		}
	}
	return out
}
func offset(data []byte, lines []int, p position) (int, error) {
	if p.Line < 1 || p.Line > len(lines) || p.Column < 1 {
		return 0, errors.New("coverage coordinate outside source")
	}
	start := lines[p.Line-1]
	end := len(data)
	if p.Line < len(lines) {
		end = lines[p.Line] - 1
	}
	at := start + p.Column - 1
	if at > end {
		return 0, fmt.Errorf("coverage column %d outside line %d", p.Column, p.Line)
	}
	return at, nil
}
func validateRanges(file *sourceFile) error {
	lines := lineOffsets(file.Bytes)
	for _, r := range file.Ranges {
		a, err := offset(file.Bytes, lines, r.Start)
		if err != nil {
			return fmt.Errorf("%s: %w", file.Name, err)
		}
		b, err := offset(file.Bytes, lines, r.End)
		if err != nil {
			return err
		}
		if a > b || r.Count < 0 {
			return errors.New("invalid coverage range")
		}
	}
	return nil
}
