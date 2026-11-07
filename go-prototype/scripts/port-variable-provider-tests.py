#!/usr/bin/env python3
"""Extract original cases and approvals from the pinned C# revision."""
from pathlib import Path
import hashlib
import json
import re
import subprocess

root = Path(__file__).resolve().parent.parent
revision = json.loads((root / 'baseline.json').read_text())['revision']
directory = 'src/GitVersion.Core.Tests/VersionCalculation'
source = directory + '/VariableProviderTests.cs'


def read(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


raw = read(source)
methods = {}
for match in re.finditer(r'((?:^    \[Test(?:Case\([^\n]*\))?\]\n)+)    public void (\w+)\(', raw.decode(), re.M):
    arguments = [json.loads('[' + value + ']') for value in re.findall(r'\[TestCase\(([^\n]*)\)\]', match[1])]
    methods[match[2]] = [
        {'id': f'case-{i:03d}', 'arguments': value}
        for i, value in enumerate(arguments or [[]])
    ]
assert len(methods) == 18 and sum(map(len, methods.values())) == 21
approvals = {}
for method in methods:
    if 'ShouldMatchApproved' not in raw.decode().split('public void ' + method + '(', 1)[1].split('    [Test', 1)[0]:
        continue
    path = directory + f'/Approved/VariableProviderTests.{method}.approved.txt'
    approved = read(path)
    approvals[method] = {'path': path, 'sha256': sha(approved), 'variables': json.loads(approved)}
assert len(approvals) == 8
target = 'testdata/csharp-variable-provider.json'
corpus = {'revision': revision, 'source': source, 'source_sha256': sha(raw), 'methods': methods, 'approvals': approvals}
data = json.dumps(corpus, indent=2) + '\n'
(root / target).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
manifest = json.loads(path.read_text())
for method, cases in methods.items():
    key = 'VariableProviderTests.' + method
    entry = {
        'source': source, 'source_sha256': sha(raw), 'method': method,
        'go_test': 'TestCSharpVariableProvider', 'case_file': target,
        'case_file_sha256': sha(data.encode()), 'case_ids': [case['id'] for case in cases],
        'adaptations': [
            'Repository-independent variable metadata preserves the original inputs, and approved JSON is compared field by field with JSON types.',
            'C# string variables and record enumeration use Go map lookup and decimal conversion for numbers. JSON round trips use UseNumber, and record copies use maps.Clone.',
        ],
        'status': 'ported-awaiting-platform-verification',
    }
    if key in manifest:
        previous = manifest[key]
        assert all(previous[k] == value for k, value in entry.items() if k != 'status'), key
        entry['status'] = previous['status']
    manifest[key] = entry
path.write_text(json.dumps(manifest, indent=2) + '\n')
print('18 methods, 21 original cases and eight approvals')
