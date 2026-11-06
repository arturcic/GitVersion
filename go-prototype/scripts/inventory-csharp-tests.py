#!/usr/bin/env python3
"""Inventory attributed test declarations in the pinned Git tree (not discovery).

Never loads the working C# tree or evaluates TestCaseSource providers. Parameter
expansion, inherited tests and generated tests require .NET discovery separately.
"""
import hashlib
import json
import re
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
entries = subprocess.check_output(
    ['git', 'ls-tree', '-r', revision, '--', 'src', 'new-cli'], cwd=root.parent, text=True
).splitlines()
files = []
for entry in entries:
    metadata, path = entry.split('\t', 1)
    if path.endswith('.cs') and any(part.endswith('.Tests') for part in Path(path).parts):
        files.append((metadata.split()[2], path))
# One batch reads exact pinned blobs without checking out or running .NET.
raw = subprocess.check_output(['git', 'cat-file', '--batch'], cwd=root.parent,
                              input=('\n'.join(sha for sha, _ in files) + '\n').encode())
pos = 0
rows = []
attribute = re.compile(r'^\s*\[(?:Test|TestCase|TestCaseSource|Theory|Fact|DataTestMethod|TestMethod)(?:\(|\])')
method = re.compile(r'^\s*(?:public|internal|protected|private)\s+(?:(?:static|async|virtual|override|new)\s+)*(?:[\w.<>,?\[\]]+\s+)(\w+)\s*\(')
for sha, path in files:
    end = raw.index(b'\n', pos)
    header = raw[pos:end].decode().split()
    size = int(header[2]); content = raw[end + 1:end + 1 + size]; pos = end + size + 2
    assert header[0] == sha
    pending = []
    for number, line in enumerate(content.decode('utf-8-sig').splitlines(), 1):
        if attribute.match(line):
            pending.append({'line': number, 'text': line.strip()})
        match = method.match(line)
        if match:
            if pending:
                rows.append({'id': path + '#' + match[1], 'source': path, 'line': number,
                             'method': match[1], 'attributes': pending,
                             'source_sha256': hashlib.sha256(content).hexdigest(),
                             'category': 'integration' if '/IntegrationTests/' in path else 'unit-or-component',
                             'status': 'pending', 'go_test': None, 'fixtures': []})
            pending = []
# Explicit reviewed mappings: each fixture retains the original FullSemVer assertion.
mappings = {
 'VersionInCurrentBranchNameScenarios.TakesVersionFromNameOfReleaseBranch': ['csharp-release-name--head'],
 'VersionInCurrentBranchNameScenarios.DoesNotTakeVersionFromNameOfNonReleaseBranch': ['csharp-feature-name--head'],
 'VersionInCurrentBranchNameScenarios.TakesVersionFromNameOfBranchThatIsReleaseByConfig': ['csharp-support-name--head'],
 'VersionInMergedBranchNameScenarios.TakesVersionFromNameOfReleaseBranch': ['csharp-release-merged--head'],
 'VersionInMergedBranchNameScenarios.TakesVersionFromNameOfBranchThatIsReleaseByConfig': ['csharp-support-merged--head'],
 'VersionInMergedBranchNameScenarios.DoesNotTakeVersionFromBranchWithAccidentalVersion': ['csharp-release-accidental--head', 'csharp-hotfix-accidental--head'],
}
found = set()
for row in rows:
    key = Path(row['source']).stem + '.' + row['method']
    if key in mappings:
        row.update(status='ported-awaiting-platform-verification', go_test='TestConfigurationEdgeFrozenReference', fixtures=mappings[key])
        found.add(key)
assert found == set(mappings), 'A mapped C# test is absent from the pin'
out = {'schema': 1, 'source_revision': revision,
       'kind': 'static attributed method inventory; not expanded NUnit discovery',
       'declarations': len(rows), 'mapped_declarations': len(found), 'tests': rows}
(root / 'testdata/csharp-test-inventory.json').write_text(json.dumps(out, indent=2) + '\n')
print(f'{len(rows)} attributed declarations; {len(found)} mapped to seven fixtures')
