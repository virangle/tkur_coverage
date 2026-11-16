# tkur_coverage native 0.3.0

This project preserves native0.2 Go/LLVM imports and adds **Istanbul JSON** and **Coverlet JSON**. Existing Python wheel0.1.1 remains compatible and unchanged. No XML backend is advertised. Release assets contain compiled binaries and legal notices only; producer helpers below are developer source tooling.

## JavaScript / TypeScript measured imports

```sh
./tkur_coverage javascript --istanbul-json coverage.json --source-root /project/src --output /reports/js
./tkur_coverage typescript --istanbul-json remapped.json --source-root /project/src --output /reports/ts --provenance replay.json
```

The input must be a complete Istanbul **coverage map**, not a summary. Statements count instrumented IDs; functions count IDs; branches count all individual arm slots; lines aggregate unique statement **start** lines with maximum hit count (Istanbul3.2.2 behavior). Empty denominators are null/н/д. Native percentages round to nearest two decimals; Istanbul itself truncates, so compare numerators/denominators rather than formatted percentages.

JS columns are zero-based UTF-16 code units, converted to one-based UTF-8 byte intervals in exact snapshots. Invalid/out-of-range/surrogate-splitting columns fail. Source color intervals combine measured statements/branch arms; function entry hits do not color an entire function body. Istanbul implicit else slots have no coordinates (`{start:{},end:{}}`): their real denominator is counted, **no source span is invented**.

TypeScript input must already be remapped externally to original TS. This importer does not parse/fetch source maps, execute JS, or trust embedded map paths. `tools/replay-istanbul.cjs` demonstrates a controlled replay of the **same exported fuzz function** using Node24.22.3, AST instrumentation6.0.3/coverage3.2.2/source-maps5.0.6 and TS5.9.3. Compiler5.9.3 is an intentional compatible pin shared with the JS fuzzer, not a claim to latest TS. The source-tree producer retains generated and remapped coverage, original compiler map and separately resolved map plus generated JS, hashes/versions/corpus inputs. Its controlled compiler map is validated before remapping; it is not an arbitrary hostile-source-map loader.

`--istanbul-origin ast` is default. Explicit `v8` accepts externally converted V8/Istanbul JSON but records **line-as-statement records and V8 block-range branches**, not AST equivalents; raw V8 conversion/provenance belongs to the external producer. The product does not claim a native raw V8 importer. Real acceptance examples use AST mode.

## C#/.NET measured imports

```sh
./tkur_coverage dotnet --coverlet-json coverage.json --source-root /project --output /reports/dotnet --provenance replay.json
```

Input: native Coverlet10.1.1 JSON (module→document→class→method→Lines/Branches), not Cobertura/OpenCover. Physical `lines` are **derived unique executable physical source lines**, hits=max per-method line hits. `producer_metrics.lines` preserves Coverlet's original **method-line entries**; lambdas/methods sharing a physical line can make these denominators differ. Branches count reachable slots using exact module/document/class/method/Line/Offset/EndOffset/Path/Ordinal identities. `IsReachable:false` is excluded, missing flag defaults true exactly as the producer model. Source rendering uses whole measured lines; missed branches mark partial lines, with **no invented statement spans or IL-offset-to-column conversion**. No .NET statement metric exists. Across different modules the same physical file merges line hits but retains separate method branch instances.

Coverlet's OpenCover sequence points use synthetic sc1/ec2 columns and its Cobertura loses branch identity; neither format is advertised as supported. Source coverage is collected on the original target+portable PDB through an independent bounded replay; SharpFuzz live feedback is not used. All target methods in the measured assembly remain in its denominator unless explicitly filtered, including unexecuted Crash/Hang example methods.

Pinned producer tools are developer-only: SDK10.0.404/runtime10.0.15 and scoped `dotnet tool install coverlet.console --version 10.1.1 --tool-path .tools`. When the application DLL itself is passed as primary assembly, actual validation needed `--include-test-assembly` (otherwise an empty result was honestly rejected). No rebuild may occur while instrumented. Final validation host invokes the real public Fuzz(byte[]) and records loaded assembly path/hash **during** instrumentation; hash differs then and equals original after Coverlet restoration. The portable producer commands below record this proof; historical private orchestration scripts and measured artifact trees are not required or shipped as standalone tooling.

Both new JSON formats reject duplicate keys, case-shadowed critical fields, invalid UTF-8, missing/null required tuples/maps, unsafe counts/paths/coordinates and aliases. Counters are nonnegative integers ≤2^53−1, rejected rather than rounded. Optional external JSON provenance is retained byte-for-byte and hashed, not followed as filesystem read instructions. `execution_owned_by: external` is always explicit; importer does not invent replay statuses/counts. Importer runtime needs no producer tools, Python or network.


Compiled standalone source-coverage **importer**, no Python dependency. Complements the existing Python tkur_coverage0.1.1; it does not replace that version's replay commands or Python coverage.py support. No target code is executed by this executable. Imports external measured profiles and explicitly records `execution_owned_by: external`, not invented replay outcomes.

## Build

```sh
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local CGO_ENABLED=0 go build -trimpath -o build/tkur_coverage .
```

Go>=1.24 needed only to build. The resulting executable embeds its local CSS/SVG/notices and needs neither Python nor network. No external module dependencies. Consumers need LLVM/Go toolchains only when collecting coverage before import.

## Portable managed producer commands

Run from this `native` directory with pinned `node`, `npm`, `dotnet` and optionally `go` on PATH. No `/opt` or original development checkout is required. These helpers execute trusted target code and are **not a target sandbox**. Choose absolute target/corpus paths yourself. Output directories must be new and disjoint from sources; do not overwrite previous measurements.

### Single-file JS or TS replay

The target must be self-contained CommonJS JS or TS compiled by the helper to CommonJS and export `fuzz(Buffer)` (sync or awaited Promise). Multi-module/bundled projects should produce their own measured/remapped Istanbul map using their pinned build pipeline instead of treating this small demonstration as a universal loader.

```sh
npm ci --prefix tools
ROOT="$PWD"
TARGET=/absolute/path/to/target.ts
CORPUS=/absolute/path/to/flat-corpus
MEASURE=/absolute/path/to/new-measure
REPORT=/absolute/path/to/new-report
node tools/replay-istanbul.cjs ts "$TARGET" "$CORPUS" "$MEASURE"
# Use `js` + `javascript` for a JavaScript target.
"$ROOT/build/tkur_coverage" typescript --istanbul-json "$MEASURE/istanbul.json" \
  --source-root "$(dirname "$TARGET")" --output "$REPORT" \
  --provenance "$MEASURE/provenance.json"
```

### .NET replay and instrumentation proof

Build your original target assembly with portable PDB **before** running Coverlet. Choose a public static `void METHOD(byte[])`; its logic should be the same target used by your fuzzer. The included generic replay host loops the selected corpus, not a fuzz loop. Use an isolated copy of all target runtime dependencies; do not instrument an actively used build. The commands below use ordinary no-space temporary paths because Coverlet passes `--targetargs` through its own argument parser; other path layouts require appropriate installed-version quoting.

```sh
ROOT="$PWD"
DOTNET=$(command -v dotnet)
dotnet tool install coverlet.console --version 10.1.1 --tool-path "$ROOT/.tools"
dotnet build tools/replay-host/replay-host.csproj -c Release -o "$ROOT/build/replay-host"
# Target.dll/.pdb and dependencies already built/copied into /absolute/isolated-build.
ASSEMBLY=/absolute/isolated-build/Target.dll
SOURCE_ROOT=/absolute/path/to/target-sources
CORPUS=/absolute/path/to/flat-corpus
MEASURE=/absolute/path/to/new-net-measure
REPORT=/absolute/path/to/new-net-report
TYPE=Example.Target
METHOD=Fuzz
HOST="$ROOT/build/replay-host/replay-host.dll"
mkdir "$MEASURE"
BEFORE=$(sha256sum "$ASSEMBLY"); BEFORE=${BEFORE%% *}
# Capture observed tools, not a hardcoded claimed package/runtime version.
dotnet --info > "$MEASURE/dotnet-info.txt"
"$ROOT/.tools/coverlet" --version > "$MEASURE/coverlet-version.txt"
"$ROOT/.tools/coverlet" "$ASSEMBLY" --target "$DOTNET" \
  --targetargs "$HOST $ASSEMBLY $CORPUS $MEASURE/loaded.json $TYPE $METHOD" \
  --include-test-assembly --format json --output "$MEASURE/coverage.json" \
  > "$MEASURE/coverlet.log" 2>&1
node -e 'const f=require("fs"),d=process.argv[1];f.writeFileSync(d+"/versions.json",JSON.stringify({dotnet_info:f.readFileSync(d+"/dotnet-info.txt","utf8"),coverlet:f.readFileSync(d+"/coverlet-version.txt","utf8")},null,2))' "$MEASURE"
node tools/net-provenance.cjs "$MEASURE" "$ASSEMBLY" "$HOST" "$BEFORE" "$MEASURE/versions.json"
"$ROOT/build/tkur_coverage" dotnet --coverlet-json "$MEASURE/coverage.json" \
  --source-root "$SOURCE_ROOT" --output "$REPORT" --provenance "$MEASURE/provenance.json"
```

Adjust Coverlet `--include`/`--exclude` filters for your assembly; without explicit filters, dependencies with source/PDB may also be measured and their source paths must lie under the selected source root. The external helper records loaded assembly/source hashes and observed input returns; the importer still owns no execution and does not fabricate statuses. Tool installation/build above require network/SDK as appropriate; importing an already produced JSON requires only the compiled renderer.

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

Header/footer/table DOM follows existing tkur_coverage and actual tkur_jacoco `ReportPage.java`. Full laboratory title, real product `tkur_coverage0.3.0`, generation timestamp/timezone and honest LLVM/Go tool attribution. Exact upstream `report.css` and SVG logo/icons from tkur_jacoco commit `fbd7e252d9a424db1e5a7dd685dd25f520a3db15`; existing fixed `adapter.css` preserved. `native.css` only adapts numbered source ranges. No CDN/fonts/network, no dashboards/cards or obsolete license boilerplate.

Original CSS/SVG notices and provenance are embedded and copied unchanged. EPL-2.0 for generator/adaptation and JaCoCo materials; Lucide/Feather ISC/MIT notices retained, logos not relicensed. See LICENSE and assets/{jacoco-LICENSE.md,lucide-LICENSE.txt,NOTICE.txt,provenance.json}. Independently implemented Go profile parser has no x/tools vendored source.

## Real tests

From project root in a prepared environment:

```sh
# Override GO,RUST,LLVM_BIN for your pinned installations, OUT must be new.
sh testdata/real-smoke.sh
TKUR_REAL_ARTIFACTS="$PWD/artifacts/real-1" go test -v ./...
```

The script compiles Rust and Go fixtures, replays A/B leaving a known missing path, imports real JSON/profiles and calls the renderer with `PATH=/nonexistent` (no Python or shell helpers). Defaults reference the development environment only; production importer uses no hardcoded tool paths. Unit tests cover weighted statements, duplicate/overflow/malformed data, missing segments, schema2/3, escaping, source symlink rejection, output collision, local links and exact logo bytes. Real acceptance checks compare known source totals and snapshots; no mocked percentages are substituted.
