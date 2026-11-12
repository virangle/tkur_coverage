"""Command-line entry point for corpus replay reports."""
import argparse
import sys

from .report import generate_native_report, generate_python_report


def main(argv=None):
    parser = argparse.ArgumentParser(description="ТЕХКОНСУР: измеренное покрытие replay корпуса")
    commands = parser.add_subparsers(dest="command", required=True)
    native = commands.add_parser("native", help="LLVM source-based coverage for C/C++")
    native.add_argument("--binary", required=True)
    native.add_argument("--corpus", required=True)
    native.add_argument("--output", required=True)
    native.add_argument("--sources-dir", required=True)
    native.add_argument("--language", choices=["C", "C++", "C/C++"], default="C/C++")
    native.add_argument("--llvm-cov")
    native.add_argument("--llvm-profdata")
    native.add_argument("--binary-arg", action="append", default=[], help="Argument before INPUT (use --binary-arg=-flag)")
    native.add_argument("--timeout", type=float, default=30)
    python = commands.add_parser("python", help="coverage.py branch and line coverage")
    python.add_argument("--target", required=True, help="MODULE:CALLABLE receiving bytes")
    python.add_argument("--corpus", required=True)
    python.add_argument("--source", required=True)
    python.add_argument("--output", required=True)
    args = vars(parser.parse_args(argv))
    command = args.pop("command")
    if command == "native":
        args["binary_args"] = args.pop("binary_arg")
    try:
        index = (generate_native_report if command == "native" else generate_python_report)(**args)
    except Exception as exc:
        print(f"tkur_coverage: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    print(index)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
