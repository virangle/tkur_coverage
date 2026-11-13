# SPDX-License-Identifier: EPL-2.0
"""Standalone bytes callback; no fuzzer runtime is required for replay."""


def TestOneInput(data):
    if data == b"A":
        result = 10
    elif data == b"B":
        result = 20
    else:
        result = 30
    return result
