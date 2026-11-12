"""Honest corpus-replay coverage reporting, distinct from fuzzing feedback."""

from .report import generate_native_report, generate_python_report

__all__ = ["generate_native_report", "generate_python_report"]
__version__ = "0.1.1"
