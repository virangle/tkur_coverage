"""Replay a corpus and retain upstream source reports and measured data.

No fuzzing edge counters are converted into source coverage percentages.
"""
from __future__ import annotations

import hashlib
import html
import importlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from typing import Any, Sequence


ASSETS = Path(__file__).with_name("assets")
LAB_NAME = 'Испытательная лаборатория ООО "ТЕХКОНСУР"'
PRODUCT = "tkur_coverage 0.1.1"
# Compatibility export for integrations inspecting the real shared stylesheet.
STYLE = (ASSETS / "report.css").read_text(encoding="utf-8")


def _escape(value: Any) -> str:
    return html.escape(str(value), quote=True)


def _header(generated_at: str, assets: str = "assets") -> str:
    timestamp = datetime.fromisoformat(generated_at).astimezone().strftime("%d.%m.%Y %H:%M:%S %z")
    return (f'<div class="tkur-header"><img class="tkur-logo" src="{_escape(assets)}/tkur.svg" '
            'alt="Логотип ТЕХКОНСУР">'
            f'<span class="tkur-wordmark">{_escape(LAB_NAME)}</span><div class="tkur-product">'
            f'<span class="tkur-product-name">{PRODUCT}</span>'
            '<span class="tkur-product-description">ОТЧЁТ О ПОКРЫТИИ КОДА</span>'
            f'<span class="tkur-created">Сформирован: {_escape(timestamp)}</span></div></div>')


def _legend() -> str:
    return ('<div class="tkur-legend"><span class="tkur-covered">Покрыто</span>'
            '<span class="tkur-partial">Частично</span><span class="tkur-missed">Не покрыто</span></div>')


def _footer(engine: str = "") -> str:
    tool = "coverage.py" if "Python" in engine else "LLVM" if "LLVM" in engine else "tkur_coverage"
    return ('<div class="footer"><span class="right">Создано с помощью '
            f'{_escape(tool)}</span><span class="tkur-owner">{_escape(LAB_NAME)} · '
            'tkur_coverage</span></div>')


def _asset_links(assets: str) -> str:
    return (f'<link rel="stylesheet" href="{_escape(assets)}/report.css">'
            f'<link rel="stylesheet" href="{_escape(assets)}/adapter.css">'
            f'<link rel="icon" type="image/svg+xml" href="{_escape(assets)}/report.svg">')


def _prepare(corpus: Path | str, output: Path | str) -> tuple[list[Path], Path]:
    corpus = Path(corpus).resolve()
    output = Path(output).resolve()
    if not corpus.is_dir():
        raise ValueError(f"Corpus directory does not exist: {corpus}")
    # Never include previous outputs in a replay, and never overwrite user files.
    if output == corpus or corpus in output.parents or output in corpus.parents:
        raise ValueError("Output and corpus directories must be disjoint")
    if output.exists() and any(output.iterdir()):
        raise ValueError(f"Output directory must be new or empty: {output}")
    inputs = sorted(p for p in corpus.rglob("*") if p.is_file() and not p.is_symlink())
    if not inputs:
        raise ValueError("Corpus must contain at least one regular input file")
    output.mkdir(parents=True, exist_ok=True)
    return inputs, output


def _input_metadata(inputs: list[Path], corpus: Path) -> list[dict[str, Any]]:
    return [{"path": str(p.relative_to(corpus)), "bytes": p.stat().st_size,
             "sha256": hashlib.sha256(p.read_bytes()).hexdigest()} for p in inputs]


def _run(command: Sequence[str], *, env: dict[str, str] | None = None,
         timeout: float = 120) -> subprocess.CompletedProcess[str]:
    return subprocess.run(list(command), env=env, text=True, encoding="utf-8",
                          errors="replace", stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          timeout=timeout, check=True)


def _tool_version(tool: str) -> str:
    return _run([tool, "--version"]).stdout.splitlines()[0]


def _snapshot_sources(output: Path, paths: Sequence[str]) -> list[dict[str, str]]:
    """Retain the exact source bytes used to render line views."""
    snapshots = output / "source-files"
    snapshots.mkdir()
    manifest = []
    for number, name in enumerate(sorted(paths)):
        source = Path(name).resolve()
        data = source.read_bytes()
        # ASCII artifact names are portable even when original source paths are not.
        saved = snapshots / f"{number:04d}{source.suffix}"
        saved.write_bytes(data)
        manifest.append({"original": str(source), "snapshot": saved.relative_to(output).as_posix(),
                         "sha256": hashlib.sha256(data).hexdigest()})
    return manifest


def _metric(hit: int, total: int) -> dict[str, Any]:
    # Empty denominators are not represented as measured 100% coverage.
    return {"covered": hit, "total": total,
            "percent": round(100 * hit / total, 2) if total else None}


def _brand_source_pages(output: Path, generated_at: str, engine: str = "") -> None:
    """Apply the real JaCoCo shell and adapt engine tables without changing data."""
    for page in (output / "source").rglob("*.html"):
        content = page.read_text(encoding="utf-8")
        back = os.path.relpath(output / "index.html", page.parent).replace(os.sep, "/")
        assets = os.path.relpath(output / "assets", page.parent).replace(os.sep, "/")
        # Replace legacy favicon/keyboard artwork; retain existing behavior/element IDs.
        content = re.sub(r'<link\b[^>]*rel=[\"\'](?:shortcut )?icon[\"\'][^>]*>', '', content, flags=re.I)
        content = re.sub(r'(<img\b[^>]*id="keyboard_icon"[^>]*src=")[^"]+("[^>]*>)',
                         lambda m: m.group(1) + assets + '/session.svg' + m.group(2), content)
        # These are upstream UI help/time labels, not copyright/license notices.
        if "LLVM" in engine:
            content = re.sub(r'<h4>Created:.*?</h4>', '', content, flags=re.S)
            content = re.sub(r'<p>Click <a[^>]*>here</a> for information about interpreting this report\.</p>', '', content)
            content = content.replace('<h2>Coverage Report</h2>', '')
            if "class='code'" in content:
                content = content.replace('<table>', '<table class="llvm-source">', 1)
        content = content.replace("</head>", _asset_links(assets) + "</head>", 1)
        title = "Исходный код" if page.name != "index.html" else "Файлы"
        navigation = (f'<div class="breadcrumb" id="breadcrumb"><a class="el_report" href="{_escape(back)}">'
                      f'Покрытие кода</a> &gt; <span class="el_source">{title}</span></div>')
        content = re.sub(r"(<body\b[^>]*>)", lambda m: m.group(1) + _header(generated_at, assets)
                         + navigation + f'<h1>{title}</h1>' + _legend(), content, count=1, flags=re.I)
        content = content.replace("</body>", _footer(engine) + "</body>", 1)
        page.write_text(content, encoding="utf-8")


def _finish(output: Path, metadata: dict[str, Any]) -> Path:
    now = datetime.now(timezone.utc).isoformat(timespec="seconds")
    metadata.update({"schema_version": 1, "generated_at": now,
                     "measurement": "corpus-replay-source-coverage",
                     "fuzzing_feedback": False,
                     "localization": "Russian wrapper and branding; upstream source views retain their original language"})
    (output / "report.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    shutil.copytree(ASSETS, output / "assets", dirs_exist_ok=True)
    _brand_source_pages(output, now, metadata["engine"])
    labels = {"lines": "Строки", "branches": "Ветви", "functions": "Функции", "regions": "Регионы"}
    headings = '<td>Элемент</td>'
    counters = '<td><a class="el_source" href="source/index.html">Исходный код</a></td>'
    for key, metric in metadata["metrics"].items():
        percent = f'{metric["percent"]:.2f}%' if metric["percent"] is not None else "н/д"
        headings += f'<td class="ctr1">{labels[key]}: пропущено</td><td class="ctr2">Всего</td><td class="ctr2">Покрытие</td>'
        counters += (f'<td class="ctr1">{metric["total"] - metric["covered"]}</td>'
                     f'<td class="ctr2">{metric["total"]}</td><td class="ctr2">{percent}</td>')
    page = ('<!doctype html><html lang="ru"><head><meta charset="utf-8">'
            '<meta name="viewport" content="width=device-width, initial-scale=1">'
            '<title>Покрытие кода</title>' + _asset_links("assets") + '</head><body>'
            + _header(now) + '<div class="breadcrumb" id="breadcrumb">'
            '<span class="info"><a class="el_source" href="source/index.html">Исходные файлы</a></span>'
            '<span class="el_report">Покрытие кода</span></div><h1>Покрытие кода</h1>' + _legend()
            + '<table class="coverage" cellspacing="0"><thead><tr>' + headings
            + '</tr></thead><tfoot><tr>' + counters.replace('<td><a class="el_source" href="source/index.html">Исходный код</a></td>', '<td>Всего</td>', 1) + '</tr></tfoot><tbody><tr>' + counters
            + '</tr></tbody></table>' + _footer(metadata["engine"]) + '</body></html>')
    (output / "index.html").write_text(page, encoding="utf-8")
    return output / "index.html"


def generate_native_report(*, binary: Path | str, corpus: Path | str, output: Path | str,
                           sources_dir: Path | str, language: str = "C/C++",
                           llvm_cov: str | None = None, llvm_profdata: str | None = None,
                           binary_args: Sequence[str] = (), timeout: float = 30) -> Path:
    """Replay binary [binary_args] INPUT, then render LLVM's actual source coverage."""
    if not math.isfinite(timeout) or timeout <= 0:
        raise ValueError("Replay timeout must be a finite positive number of seconds")
    binary = Path(binary).resolve()
    source_root = Path(sources_dir).resolve()
    if not binary.is_file() or not source_root.is_dir():
        raise ValueError("Binary and source directory must exist")
    cov = llvm_cov or os.environ.get("LLVM_COV", "llvm-cov")
    profdata = llvm_profdata or os.environ.get("LLVM_PROFDATA", "llvm-profdata")
    versions = {"llvm_cov": _tool_version(cov), "llvm_profdata": _tool_version(profdata)}
    inputs, output = _prepare(corpus, output)
    profiles = output / "profiles"
    profiles.mkdir()
    replay = []
    for number, item in enumerate(inputs):
        env = dict(os.environ, LLVM_PROFILE_FILE=str(profiles / f"input-{number}-%p.profraw"))
        command = [str(binary), *binary_args, str(item)]
        try:
            result = _run(command, env=env, timeout=timeout)
            replay.append({"input": str(item), "status": "ok", "returncode": 0,
                           "stdout": result.stdout, "stderr": result.stderr})
        except subprocess.CalledProcessError as exc:
            replay.append({"input": str(item), "status": "error", "returncode": exc.returncode,
                           "stdout": exc.stdout, "stderr": exc.stderr})
        except subprocess.TimeoutExpired:
            replay.append({"input": str(item), "status": "timeout", "timeout_seconds": timeout})
    raw_profiles = sorted(profiles.glob("*.profraw"))
    if not raw_profiles:
        raise RuntimeError("Replay produced no LLVM profiles; compile with -fprofile-instr-generate -fcoverage-mapping")
    merged = profiles / "merged.profdata"
    _run([profdata, "merge", "-sparse", *map(str, raw_profiles), "-o", str(merged)])
    base = [cov, "export", str(binary), f"-instr-profile={merged}"]
    initial = json.loads(_run(base).stdout)
    selected = sorted({entry["filename"] for data in initial["data"] for entry in data["files"]
                       if Path(entry["filename"]).resolve().is_relative_to(source_root)})
    if not selected:
        raise RuntimeError(f"No instrumented sources under {source_root}")
    exported = json.loads(_run([*base, *selected]).stdout)
    if len(exported["data"]) != 1:
        raise RuntimeError("Expected a single LLVM coverage object")
    (output / "llvm-export.json").write_text(json.dumps(exported, indent=2) + "\n", encoding="utf-8")
    _run([cov, "show", str(binary), f"-instr-profile={merged}", "-format=html",
          f"-output-dir={output / 'source'}", "-show-line-counts-or-regions", "-show-branches=count", *selected])
    totals = exported["data"][0]["totals"]
    metrics = {name: _metric(int(totals[name]["covered"]), int(totals[name]["count"]))
               for name in ("lines", "branches", "functions", "regions") if name in totals}
    return _finish(output, {"engine": f"{language} / LLVM source-based coverage", "tools": versions,
                           "source_root": str(source_root), "sources": selected,
                           "source_snapshots": _snapshot_sources(output, selected), "metrics": metrics,
                           "inputs": _input_metadata(inputs, Path(corpus).resolve()), "replay": replay,
                           "raw_data": "llvm-export.json", "binary": str(binary),
                           "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest()})


def generate_python_report(*, target: str, corpus: Path | str, output: Path | str,
                           source: Path | str) -> Path:
    """Import MODULE:CALLABLE under coverage and call it with each input's bytes.

    Target runs in this process. Use the CLI in an isolated process/container for untrusted code.
    """
    import coverage
    if ":" not in target:
        raise ValueError("Target must be MODULE:CALLABLE")
    source = Path(source).resolve()
    if not source.is_dir():
        raise ValueError("Source directory must exist")
    module_name, callable_name = target.split(":", 1)
    if module_name in sys.modules:
        raise ValueError("Target module is already imported; use a fresh process for honest import coverage")
    inputs, output = _prepare(corpus, output)
    collector = coverage.Coverage(data_file=str(output / ".coverage"), source=[str(source)], branch=True)
    replay = []
    collector.start()
    try:
        callback = getattr(importlib.import_module(module_name), callable_name)
        if not callable(callback):
            raise ValueError("Target must be callable")
        for item in inputs:
            try:
                callback(item.read_bytes())
                replay.append({"input": str(item), "status": "ok"})
            except Exception as exc:
                replay.append({"input": str(item), "status": "exception", "type": type(exc).__name__,
                               "message": str(exc)})
    finally:
        collector.stop()
        collector.save()
    collector.json_report(outfile=str(output / "coverage.json"))
    collector.html_report(directory=str(output / "source"), title="Python source coverage")
    raw = json.loads((output / "coverage.json").read_text(encoding="utf-8"))
    totals = raw["totals"]
    metrics = {"lines": _metric(totals["covered_lines"], totals["num_statements"]),
               "branches": _metric(totals["covered_branches"], totals["num_branches"])}
    return _finish(output, {"engine": "Python / coverage.py", "tools": {"coverage": coverage.__version__},
                           "source_root": str(source), "target": target,
                           "source_snapshots": _snapshot_sources(output, list(raw["files"])), "metrics": metrics,
                           "inputs": _input_metadata(inputs, Path(corpus).resolve()), "replay": replay,
                           "raw_data": "coverage.json"})
