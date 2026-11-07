#!/usr/bin/env python3
"""Extract the pinned NUnit format and dotenv cases."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
source = 'src/GitVersion.Output.Tests/Output/FormatArgumentTests.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
source_sha = hashlib.sha256(raw).hexdigest()
methods = {}
for attributes, method in re.findall(
    r'((?:^    \[TestCase[^\n]*\]\n)+)    public void (\w+)\(', raw.decode(), re.M
):
    args = [
        json.loads('[' + line + ']')
        for line in re.findall(r'^    \[TestCase\((.*)\)\]$', attributes, re.M)
    ]
    methods[method] = [
        {'id': f'case-{i:03d}', 'arguments': values}
        for i, values in enumerate(args or [[]])
    ]

assert len(methods) == 6 and sum(map(len, methods.values())) == 29
case_file = 'testdata/csharp-format-arguments.json'
data = json.dumps(dict(revision=revision, source=source, source_sha256=source_sha,
                       methods=methods), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for method, cases in methods.items():
    key = f'FormatArgumentTests.{method}'
    entry = dict(
        source=source, source_sha256=source_sha, method=method,
        go_test='TestCSharpFormatArguments', case_file=case_file,
        case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
        case_ids=[case['id'] for case in cases],
        adaptations=['Go runs the CLI output path using equivalent deterministic go-git histories. '
                     'The dotenv line count is compared with the calculated JSON variable count.'],
        status='ported-awaiting-platform-verification',
    )
    if key in mappings:
        entry['status'] = mappings[key]['status']
        assert entry == mappings[key], key
    mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
