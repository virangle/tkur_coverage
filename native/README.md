# tkur_coverage native 0.2.0

Compiled standalone source-coverage **importer**, no Python dependency. Complements the existing Python tkur_coverage0.1.1; it does not replace that version's replay commands or Python coverage.py support. No target code is executed by this executable. Imports external measured profiles and explicitly records `execution_owned_by: external`, not invented replay outcomes.

## Build

```sh
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local CGO_ENABLED=0 go build -trimpath -o build/tkur_coverage .
```

Go>=1.24 needed only to build. The resulting executable embeds its local CSS/SVG/notices and needs neither Python nor network. No external module dependencies. Consumers need LLVM/Go toolchains only when collecting coverage before import.

## Rust / LLVM import

Build with `-C instrument-coverage` using your pinned Rust toolchain, replay the corpus, merge profiles with the **same toolchain's** `llvm-profdata`, and export complete JSON with its `llvm-cov`. System LLVM18 is not interchangeable with Rust nightly's LLVM23 raw profiles.

```sh
# Paths are examples: use matching tools of your actual Rust toolchain.
LLVM_BIN=/path/to/rust-toolchain/lib/rustlib/x86_64-unknown-linux-gnu/bin
"$LLVM_BIN/llvm-profdata" merge -sparse profiles/*.profraw -o merged.profdata
"$LLVM_BIN/llvm-cov" export ./instrumented-replay \
  -instr-profile=merged.profdata > llvm-export.json

./tkur_coverage rust --llvm-json llvm-export.json \
  --source-root /absolute/path/to/project/src --output report-rust
```

`llvm-import` and `native-import` aliases use the same parser with generic LLVM engine labeling; `rust` labels Rust. Supported LLVM export JSON schemas2.x and3.x, tested against actual LLVM23.1.5 schema3.1.0 and unit fixtures2.0.1. **Do not use `-summary-only`:** source segments are required. Unknown schema families or malformed segments fail clearly.

The importer uses measured per-file lines/functions/regions/branches summaries, ignoring supplied percentages and recomputing `100*covered/total`. It excludes ordinary absolute paths outside source-root (dependency/runtime files), rejects traversal and source symlink aliases, and requires at least one selected source. No source file outside root is read. Aggregate totals are sums of selected file summaries, not unfiltered engine totals.

Rust block instrumentation often has branches0/0: output is **н/д/null**, never100%. Branch instrumentation is toolchain/flag-dependent and may require nightly flags; absence is not invented as measured coverage. Function instantiations are not relabeled separate functions. Source colors use counted non-gap LLVM **segment intervals**, not inferred per-line execution counters; macros/expansion views are not reproduced. Full LLVM JSON is retained for functions/expansions/MC/DC and other details not rendered by this minimal importer.

## Go import

`go test -fuzz` cannot combine with `-coverprofile`; collect source coverage in a separate ordinary seed/corpus replay, without `-fuzz`. Generated fuzz cache entries are **not** ordinary test seeds: explicitly copy an approved snapshot into an isolated module's `testdata/fuzz/FuzzTarget`, preserving Go's typed corpus format. This importer does not discover/mutate caches or source trees.

```sh
# Run inside the isolated module snapshot, using its FuzzTarget/testdata seeds.
go test . -run='^FuzzTarget$' -count=1 -timeout=60s \
  -covermode=atomic -coverprofile=coverage.out

go tool cover -func=coverage.out
./tkur_coverage go-import --profile coverage.out \
  --module-path example.org/project --source-root "$PWD" --output /other/path/report-go
```

Alias: `go`. `--module-path` strips exactly that prefix from profile filenames; relative source paths are also accepted. Nonmatching paths must resolve under source-root or fail. Multi-module/external dependency remapping is not automatic.

For a `go build -cover` application replay, collect `GOCOVERDIR` metadata/counters and first run `go tool covdata textfmt -i=COVDIR -o=coverage.out`; import that native profile. An unrecovered panic/fatal exit can lose counters; importer does not assert all inputs ran successfully.

Metric is **Go statements**, computed as `sum(NumStmt for blocks with Count>0) / sum(NumStmt)`. It is not line coverage or branch coverage. A function percentage printed by `go tool cover -func` is statement coverage in its range, not separate function-hit instrumentation. Source colors use byte-column block ranges; overlaps with both hit and missing blocks are partial. Statement-free tracked blocks may color ranges without contributing to denominators.

Parser independently implemented with Go standard library (no copied x/tools code). Modes set/count/atomic, strict nonnegative integers/byte coordinates, duplicate ranges merge using OR for set or checked addition for count/atomic; inconsistent NumStmt and arithmetic overflow reject. Out-of-source coordinates reject before output creation.

## Provenance and safe paths

Pass `--provenance replay-manifest.json` to retain an external **valid JSON** build/replay/input manifest byte-for-byte plus SHA256. Prefer exact corpus SHA256, explicit toolchain version, command arrays and binary fingerprint from the actual producer. The importer cannot independently prove that an imported profile belongs to current source bytes: it checks source ranges and records exact snapshots/hashes, but producers must tie source/build/corpus identities together. Do not import stale profiles and describe them as newly measured.

Output must be new or empty, disjoint from source-root and input artifacts. File paths are validated before reading; lexical traversal, symlink escapes, nonregular/oversized files, unknown schemas and malformed counters reject. Absolute outside-root LLVM dependency paths are skipped. Imported HTML/JavaScript is never accepted. Source text/filenames escape through safe templates; generated span tags are fixed, not profile data. Source snapshots retain exact original bytes. The source root should be immutable during import; this is not a sandbox against concurrent malicious filesystem changes.

Artifacts:

- `index.html`, `source/index.html`, `source/N.html`: compact measured table and numbered/range-highlighted source pages.
- `report.json`: schema2, external-import origin, real numerators/denominators, raw-data/source hashes, snapshots and engine identity. No fabricated `replay` statuses.
- `raw/llvm-export.json` or `raw/coverage.out`: exact imported bytes.
- `raw/provenance.json` when provided: exact external manifest, not trusted executable instructions.
- `source-files/N.txt`: exact source bytes.
- `assets/*`: embedded styles/SVG and original legal notices.

Online fuzz `cov`/`ft`/interesting-input counters are not source coverage percentages. Imported source coverage describes external replay and its selected source scope only. Zero denominators show н/д. Go statements and LLVM lines/branches have different definitions and are not cross-language rankings.

## Exact shared style and legal notices

Header/footer/table DOM follows existing tkur_coverage and actual tkur_jacoco `ReportPage.java`. Full laboratory title, real product `tkur_coverage0.2.0`, generation timestamp/timezone and honest LLVM/Go tool attribution. Exact upstream `report.css` and SVG logo/icons from tkur_jacoco commit `fbd7e252d9a424db1e5a7dd685dd25f520a3db15`; existing fixed `adapter.css` preserved. `native.css` only adapts numbered source ranges. No CDN/fonts/network, no dashboards/cards or obsolete license boilerplate.

Original CSS/SVG notices and provenance are embedded and copied unchanged. EPL-2.0 for generator/adaptation and JaCoCo materials; Lucide/Feather ISC/MIT notices retained, logos not relicensed. See LICENSE and assets/{jacoco-LICENSE.md,lucide-LICENSE.txt,NOTICE.txt,provenance.json}. Independently implemented Go profile parser has no x/tools vendored source.

## Real tests

From project root in a prepared environment:

```sh
# Override GO,RUST,LLVM_BIN for your pinned installations, OUT must be new.
sh testdata/real-smoke.sh
TKUR_REAL_ARTIFACTS="$PWD/artifacts/real-1" go test -v ./...
```

The script compiles Rust and Go fixtures, replays A/B leaving a known missing path, imports real JSON/profiles and calls the renderer with `PATH=/nonexistent` (no Python or shell helpers). Defaults reference the development environment only; production importer uses no hardcoded tool paths. Unit tests cover weighted statements, duplicate/overflow/malformed data, missing segments, schema2/3, escaping, source symlink rejection, output collision, local links and exact logo bytes. Real acceptance checks compare known source totals and snapshots; no mocked percentages are substituted.
