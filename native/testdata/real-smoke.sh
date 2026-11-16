#!/bin/sh
# SPDX-License-Identifier: EPL-2.0
# Run from project root; output directory must be new.
set -eu
ROOT=$(pwd)
GO=${GO:-/opt/tkur/go-bootstrap/bin/go}
RUST=${RUST:-/opt/tkur/rust-nightly-2026-11-12/bin/rustc}
LLVM_BIN=${LLVM_BIN:-/opt/tkur/rust-nightly-2026-11-12/lib/rustlib/x86_64-unknown-linux-gnu/bin}
OUT=${OUT:-$ROOT/artifacts/real-1}
mkdir -p "$OUT/profiles" "$OUT/corpus"
printf A > "$OUT/corpus/A"
printf B > "$OUT/corpus/B"
GOTOOLCHAIN=local "$GO" build -o "$OUT/tkur_coverage" .
"$RUST" -vV > "$OUT/rust-version.txt"
"$LLVM_BIN/llvm-cov" --version > "$OUT/llvm-version.txt"
"$GO" version > "$OUT/go-version.txt"
"$RUST" -C instrument-coverage -C opt-level=0 -C debuginfo=2 testdata/rust/target.rs -o "$OUT/rust-target"
LLVM_PROFILE_FILE="$OUT/profiles/A-%m-%p.profraw" "$OUT/rust-target" "$OUT/corpus/A"
LLVM_PROFILE_FILE="$OUT/profiles/B-%m-%p.profraw" "$OUT/rust-target" "$OUT/corpus/B"
"$LLVM_BIN/llvm-profdata" merge -sparse "$OUT"/profiles/*.profraw -o "$OUT/merged.profdata"
"$LLVM_BIN/llvm-cov" export "$OUT/rust-target" -instr-profile="$OUT/merged.profdata" > "$OUT/llvm-export.json"
printf '{"execution":"external replay","inputs":["A","B"],"toolchain":"nightly-2026-11-12"}\n' > "$OUT/replay.json"
# Explicit no-Python PATH for production importer calls.
PATH=/nonexistent "$OUT/tkur_coverage" rust --llvm-json "$OUT/llvm-export.json" --source-root "$ROOT/testdata/rust" --output "$OUT/rust-report" --provenance "$OUT/replay.json"
(cd testdata/go && GOTOOLCHAIN=local "$GO" test . -run='^FuzzClassify$' -count=1 -timeout=60s -covermode=atomic -coverprofile="$OUT/coverage.out" -json > "$OUT/go-replay.jsonl")
(cd testdata/go && GOTOOLCHAIN=local "$GO" tool cover -func="$OUT/coverage.out") > "$OUT/go-functions.txt"
printf '{"execution":"go test seed replay without fuzz","target":"FuzzClassify","seeds":["inline A","inline B"]}\n' > "$OUT/go-replay.jsonl.manifest.json"
PATH=/nonexistent "$OUT/tkur_coverage" go-import --profile "$OUT/coverage.out" --module-path example.org/tkur-coverage-fixture --source-root "$ROOT/testdata/go" --output "$OUT/go-report" --provenance "$OUT/go-replay.jsonl.manifest.json"
