# Native tkur_coverage 0.2.0 validation

15 November 2026; Ubuntu development container, Go1.27.4, rustc1.102nightly2026-11-12, matching bundled LLVM23.1.5. Implementation scope is this isolated project only; existing published Python package unchanged.

Commands (inside project):

```sh
export GOTOOLCHAIN=local PATH=/opt/tkur/go-bootstrap/bin:$PATH
OUT=$PWD/artifacts/real-final-20261115 sh testdata/real-smoke.sh
TKUR_REAL_ARTIFACTS=$PWD/artifacts/real-final-20261115 go test -v .
go vet .
CGO_ENABLED=0 go build -trimpath -o build/tkur_coverage .
sha256sum build/tkur_coverage
```

Result: **11 tests PASS, zero skips**, go vet PASS. Native Linux-amd64 binary SHA256:
`86c49f8fe46fcd96826cb5385e55b7c6e93e8a21521030e58a1d9d58ae05f452`.

Production importer calls in the real smoke run use `PATH=/nonexistent`: no Python, shell helper, LLVM/Go invocation or network is needed for importing/rendering measurements. Measurement collection explicitly invokes the appropriate external toolchains before import.

| Actual source measurement | Covered/total | Percent |
|---|---:|---:|
| Rust direct A/B replay, LLVM lines | 8/9 | 88.89% |
| Rust direct A/B replay, LLVM regions | 13/16 | 81.25% |
| Rust direct functions | 2/2 | 100% |
| Rust branches (not instrumented) | 0/0 | н/д / JSON null |
| Go ordinary FuzzClassify seed replay, statements | 4/5 | 80% |

Go's separate `go tool cover -func` output reports Classify80.0% and total statements80.0%, agreeing with imported weighted NumStmt totals. Rust comes from actual LLVM23 export schema3.1.0; an initial unsupported-version rejection was investigated and support added with schema2/3 regression coverage, not bypassed using fake input.

Additionally, the Rust owner's real cargo-fuzz coverage export was imported with source-root set to its demo. Dependency/runtime files were excluded without reading outside-root source paths. Result: **4/5 LLVM lines (80%), 6/7regions (85.71%), 1/1functions, branches0/0null**. Output: `artifacts/actual-cargo-fuzz-20261115`; raw LLVM export is retained exactly. That imported output does not fabricate per-input replay statuses.

Tests cover malformed/negative/overflow/setcount/duplicate Go profiles, NumStmt weighting, LLVM missing/null counters and segment fields, supported/unknown schemas, summary-only rejection, byte-column bounds, inactive segment bounds, UTF-8/CRLF and overlapping partial ranges, `<script>`/filename escaping, source symlink/traversal rejection, output/source overlap and overwrite refusal, null zero denominators, exact upstream logo hash, local HTML links, real fixture numerators and snapshot hashes/byte equality. Snapshots use neutral `.txt` names to avoid executable HTML interpretation or accidental Go package compilation; original filenames remain metadata.

All reports retain exact canonical tkur_jacoco CSS/SVG/legal assets and use existing full laboratory branding/compact tables, not new cards or technical-field dashboards. Independent QA was notified with frozen CLI contract and binary hash; its separate result is not implied by this document.
