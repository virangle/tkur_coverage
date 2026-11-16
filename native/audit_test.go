// SPDX-License-Identifier: EPL-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestExactAssetsAndLocalLinks(t *testing.T) {
	logo, err := resources.ReadFile("assets/tkur.svg")
	if err != nil {
		t.Fatal(err)
	}
	if digest(logo) != "f21a0906fb1eee7e85881ffbfe37855a2769887c1947231a520ef9ea6132fafd" {
		t.Fatal("upstream logo changed")
	}
	for _, name := range []string{"report.css", "report.svg", "source.svg", "lucide-LICENSE.txt", "jacoco-LICENSE.md", "NOTICE.txt", "provenance.json"} {
		if _, err := resources.ReadFile("assets/" + name); err != nil {
			t.Fatal(err)
		}
	}
	root, _ := source(t, "evil<&.go", "<script>\n")
	input := filepath.Join(t.TempDir(), "profile")
	if err := os.WriteFile(input, []byte("mode: set\nevil<&.go:1.1,1.9 1 0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "report")
	if err := generate(options{Engine: "Go / cover", Root: root, Input: input, Output: output}); err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`(?:href|src)="([^"]+)"`)
	err = filepath.WalkDir(output, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if !strings.Contains(text, "Испытательная лаборатория") || !strings.Contains(text, "Сформирован:") {
			t.Errorf("missing branding in %s", path)
		}
		if strings.Contains(text, "<script>") {
			t.Errorf("source injection in %s", path)
		}
		for _, m := range refs.FindAllStringSubmatch(text, -1) {
			link := strings.Split(m[1], "#")[0]
			if link == "" {
				continue
			}
			if strings.Contains(link, "://") {
				t.Errorf("remote asset/link %s", link)
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), filepath.FromSlash(link))); err != nil {
				t.Error(path, link, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestRealImportedFixtures(t *testing.T) {
	artifacts := os.Getenv("TKUR_REAL_ARTIFACTS")
	if artifacts == "" {
		t.Skip("set TKUR_REAL_ARTIFACTS after testdata/real-smoke.sh")
	}
	for _, engine := range []string{"rust", "go"} {
		output := filepath.Join(artifacts, engine+"-report")
		data, err := os.ReadFile(filepath.Join(output, "report.json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Metrics map[string]metric `json:"metrics"`
			Sources []sourceFile      `json:"source_snapshots"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		if engine == "rust" {
			if m := doc.Metrics["lines"]; m.Covered != 8 || m.Total != 9 {
				t.Fatal(m)
			}
			if m := doc.Metrics["regions"]; m.Covered != 13 || m.Total != 16 {
				t.Fatal(m)
			}
			if m := doc.Metrics["branches"]; m.Percent != nil {
				t.Fatal("0/0 branches claimed measured percent")
			}
		}
		if engine == "go" {
			if m := doc.Metrics["statements"]; m.Covered != 4 || m.Total != 5 || *m.Percent != 80 {
				t.Fatal(m)
			}
			if _, ok := doc.Metrics["lines"]; ok {
				t.Fatal("invented Go lines")
			}
		}
		for _, f := range doc.Sources {
			data, err := os.ReadFile(filepath.Join(output, f.Snapshot))
			if err != nil {
				t.Fatal(err)
			}
			if digest(data) != f.SHA256 {
				t.Fatal("snapshot hash mismatch")
			}
			source, err := os.ReadFile(f.Name)
			if err != nil {
				t.Fatal(err)
			}
			if string(source) != string(data) {
				t.Fatal("source bytes changed")
			}
		}
	}
}
