#!/usr/bin/env python3
"""Extract the original configuration validation command cases."""
import hashlib
import json
import re
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
suite = 'ConfigurationValidationIntegrationTests'
source = f'src/GitVersion.App.Tests/{suite}.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
methods = {}
for attrs, method in re.findall(
    r'((?:^    \[Test[^\n]*\]\n)+)    public (?:async )?(?:void|Task) (\w+)\(', raw.decode(), re.M
):
    cases = [
        json.loads('[' + row + ']')
        for row in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M)
    ]
    assert cases or attrs.strip() == '[Test]'
    methods[method] = [
        dict(id=f'case-{i:03}', arguments=args)
        for i, args in enumerate(cases or [[]])
    ]
assert tuple(map(len, methods.values())) == (6, 15, 7, 1, 2, 1, 1, 2, 12, 5, 1, 1, 4, 2, 1, 1)
sha = hashlib.sha256(raw).hexdigest()
target = 'testdata/csharp-configuration-validation.json'
data = json.dumps(dict(revision=revision, source=source, source_sha256=sha, methods=methods), indent=2) + '\n'
(root / target).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
for name, cases in methods.items():
    key = f'{suite}.{name}'
    entry = dict(
        source=source, source_sha256=sha, method=name, go_test='TestCSharpConfigurationValidationCLI',
        case_file=target, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
        case_ids=[c['id'] for c in cases],
        adaptations=['Go command dispatch and isolated streams stand in for the C# application fixture. Hash snapshots include repository and untracked files. Explicit validation injects a failing discovery callback and uses the original backend environment values with the pure-Go runtime.'],
        status='ported-awaiting-platform-verification',
    )
    if key in mapping:
        old = mapping[key]
        assert all(old[k] == v for k, v in entry.items() if k != 'status'), key
        entry['status'] = old['status']
    mapping[key] = entry
path.write_text(json.dumps(mapping, indent=2) + '\n')
print(f'{len(methods)} original methods and {sum(map(len, methods.values()))} cases')
