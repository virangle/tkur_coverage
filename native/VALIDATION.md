# Native managed coverage0.3.0 — validation

Дата:16.11.2026. Изолированный проект, canonical native0.2 и Python wheel0.1.1 не менялись. No publishing performed.

## Frozen executable / tools

- `build/tkur_coverage`, Linux/amd64, Go1.27.4, `CGO_ENABLED=0 go build -trimpath`; SHA256 `57131d295dd288b0e68ca594aea246d26f1d33c63c18cf3da450a58ff374fe6f`.
- Node24.22.3; npm11.19.1; Istanbul instrumentation6.0.3/coverage3.2.2/source-maps5.0.6; TypeScript5.9.3 (intentional compatible pin). Exact transitive package-lock under tools/.
- SDK10.0.404/runtime10.0.15; Coverlet tool package10.1.1, executable reports `10.1.1.1+a84a5a9ab1`. Installed with scoped `--tool-path .tools`, not global.
- `go test -race ./...`, `go vet ./...` pass after hardening. `go test -list . ./...` lists18 tests (11 retained,7 new); the opt-in `TestRealImportedFixtures` may skip without its environment. Legacy Go/LLVM tests and copied real fixtures retained. No claim that legacy real Rust/Go replay was rerun in this phase; existing unit tests were.

## Six genuine target replay reports

All final imports executed with `PATH=/nonexistent`: renderer does not shell out or require Node/.NET/Python. Target logic is the actual owner-frozen JS and TS exported `fuzz` and C# public `Fuzz(byte[])`, not synthetic report fixtures. Source/producer/binary/corpus hashes retained.

| Target/corpus | Lines covered/total | Statements | Branch slots | Functions |
|---|---:|---:|---:|---:|
| JS A (`A`) |6/8|6/11|4/10|2/2|
| JS B (`A`,`B`) |7/8|8/11|6/10|2/2|
| TS A (`A`) |6/8|7/12|4/10|2/2|
| TS B (`A`,`B`) |7/8|9/12|6/10|2/2|
| C# A (`A`) |3/19|n/a|5/26|not rendered|
| C# B (`A`,`T`,`TK`) |5/19|n/a|10/26|not rendered|

JS/TS statements differ because TS compiler/remapper preserves mapped generated records; these are actual remapped Istanbul summary denominators, not forced equal to JS. `FileCoverage.toSummary()` numerators/denominators matched exactly for both. Rounding differs deliberately: native rounds, Istanbul truncates (e.g.6/11=54.55 native vs54.54 upstream). Branch arm totals include actual implicit else with no source span.

.NET `lines` are derived unique physical executable lines; producer method-line entries separately retained in report JSON. This fixture happens not to demonstrate duplicated physical lines, but unit test has two methods sharing one line: physical1/1 versus producer1/2, distinct branch slots kept. Full selected assembly includes uninvoked Flat/Crash/Fatal/Hang/Count/Filter methods, so denominator is not Fuzz-only. `IsReachable:false` excluded exactly upstream10.1.1 semantics.

Final HTML: `evidence/final/{js,ts,net}-{a,b}-report/index.html`; JSON in same report directory. Original JS/TS measurements: `evidence/{js,ts}-{a,b}-measure`, including compiler map, separately resolved map, generated JS, raw pre-remap JSON, final original-TS coverage, upstream summary, provenance. Final .NET raw+logs/build+loaded evidence: `evidence/final/net-{a,b}-measure`.

`tools/audit-evidence.cjs --final` independently recomputed raw .NET counts and compared JS/TS upstream summaries, exact embedded asset hashes and source snapshot hashes across six reports; `evidence/final/verified-totals.json` records results. No fake replay array/counts in native report metadata; `execution_owned_by:external` always explicit.

## Loaded assembly proof

`tools/replay-host` is developer evidence tooling, not runtime dependency or release payload. It uses `AssemblyLoadContext.Default.LoadFromAssemblyPath`, records `Assembly.Location` and SHA256 while instrumented, then invokes the exact same public static Fuzz(byte[]) for each corpus input. `tools/net-provenance.cjs` asserts instrumented loaded hash differs from restored original and pre/post Coverlet SHA256 identical. Loaded evidence and observed per-input return statuses belong to the external producer; native importer copies provenance and never invents them.

Initial Coverlet invocation without `--include-test-assembly` produced `{}` and instrumented0 modules despite successful replay. Importer rejected it as no source files; no zero/100% report was manufactured. Corrected invocation includes primary target assembly and real line/branch counts. Original and final invocations use isolated target DLL copies, not mutate owner original.

## Safety / semantics tests

- All critical maps/IDs/counter arities, negative/null/unsafe integer counts rejected; JSON duplicate keys, excessive depth, invalid UTF8, case-shadow critical fields rejected.
- Coverlet required branch tuple fields cannot silently default; positive-hit and missed branch line bounds checked; same-module source aliases rejected; method identity retained.
- UTF16→UTF8 conversion tests include tabs/cyrillic/astral characters and surrogate-splitting rejection. Real TS remap reports original `.ts`, not generated JS label.
- True implicit else absent location allowed narrowly only second arm of two-arm `if`; remains denominator but has no invented highlight.
- Legacy tests retain traversal/symlink/output collision, snapshot `.txt` bytes, escaping/local links and asset hash invariants. Report input never follows embedded source-map URLs or executes JS/XML.
- No Cobertura/OpenCover support advertised; source research shows Coverlet OpenCover synthetic sc1/ec2, so no false C# statement ranges.

## Reproduction

Install pinned producer tools locally (`tools/package-lock.json`, `.tools` Coverlet) then `sh tools/final-validate.sh` in the prepared container; outputs must be new/empty. Script intentionally will not overwrite existing report directories. `tools/validate-js.sh` creates original measurement examples; `tools/final-validate.sh` renders frozen JS/TS measurements and measures .NET anew. To replay from scratch use a fresh isolated output tree, not delete someone else's reports.

Independent QA is separate; this document reports owner tests, not an independent passing verdict. Binaries-only packaging/integration remains Lead responsibility.
