#!/usr/bin/env python3
"""Check the installer workflow's target sets without launching builds."""
import json
import pathlib
import re

workflow = pathlib.Path(__file__).resolve().parents[1] / '.github/workflows/desktop.yml'
expression = next(line for line in workflow.read_text().splitlines() if 'include: ${{ fromJSON(' in line)
windows, pilot, all_targets = [json.loads(value) for value in re.findall(r"'(\[.*?\])'", expression)]
assert 'inputs.windows_only &&' in expression
assert 'inputs.windows_linux_only &&' in expression
assert windows == [all_targets[1]], 'Existing Windows-only selection changed'
assert pilot == all_targets[:2], 'Pilot must reuse the normal Linux and Windows target definitions'
assert {target['target'] for target in pilot} == {'x86_64-unknown-linux-gnu', 'x86_64-pc-windows-msvc'}
assert len(all_targets) == 4 and all(target['os'].startswith('macos-') for target in all_targets[2:]), 'Default Mac targets must remain present'
print('PASS: Windows-only, two-platform pilot, and four-platform default target sets')
