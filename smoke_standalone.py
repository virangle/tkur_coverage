"""Developer-only installed-wheel smoke test; no monorepo imports or fixtures."""
from pathlib import Path
import json
import os
import subprocess
import sys

root = Path(__file__).resolve().parent
work = root / "standalone-check"
work.mkdir(exist_ok=True)
corpus = work / "corpus"
corpus.mkdir(exist_ok=True)
(corpus / "A").write_bytes(b"A")
(corpus / "B").write_bytes(b"B")
python = Path(sys.executable)
if "site-packages" not in __import__("tkur_coverage").__file__:
    raise RuntimeError("Smoke check must run with an installed wheel outside the source working directory")
for language, compiler, name in [("C", "clang-18", "c"), ("C++", "clang++-18", "cpp")]:
    binary = work / ("target-" + name)
    command = [compiler]
    if language == "C++":
        command += ["-x", "c++"]
    subprocess.run([*command, "-O0", "-g", "-fprofile-instr-generate", "-fcoverage-mapping",
                    str(root / "examples/native/target.c"), "-o", str(binary)], check=True)
    report = work / ("report-" + name)
    subprocess.run([str(python), "-m", "tkur_coverage", "native", "--binary", str(binary),
                    "--corpus", str(corpus), "--sources-dir", str(root / "examples/native"),
                    "--language", language, "--output", str(report)], check=True)
    meta = json.loads((report / "report.json").read_text())
    raw = json.loads((report / "llvm-export.json").read_text())["data"][0]["totals"]
    for key, metric in meta["metrics"].items():
        assert metric["covered"] == raw[key]["covered"]
        assert metric["total"] == raw[key]["count"]
    assert meta["source_snapshots"]
    print(language, meta["metrics"])
report = work / "report-python"
env = dict(os.environ, PYTHONPATH=str(root / "examples/python"))
subprocess.run([str(python), "-m", "tkur_coverage", "python", "--target", "target:TestOneInput",
                "--corpus", str(corpus), "--source", str(root / "examples/python"),
                "--output", str(report)], check=True, env=env)
meta = json.loads((report / "report.json").read_text())
raw = json.loads((report / "coverage.json").read_text())["totals"]
assert meta["metrics"]["lines"]["covered"] == raw["covered_lines"]
assert meta["metrics"]["lines"]["total"] == raw["num_statements"]
assert meta["metrics"]["branches"]["covered"] == raw["covered_branches"] == 3
assert meta["metrics"]["branches"]["total"] == raw["num_branches"] == 4
assert all(item["status"] == "ok" for item in meta["replay"])
print("Python", meta["metrics"])
print("Installed wheel standalone smoke: C/C++/Python PASS")
