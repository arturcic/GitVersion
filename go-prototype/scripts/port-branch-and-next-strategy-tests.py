#!/usr/bin/env python3
"""Extract pinned branch-name and configured next-version strategy cases."""
import hashlib
import json
import re
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
suites = {}
suites_to_counts = {
    'VersionInBranchNameBaseVersionStrategyTests': (2, 8, 8, 2),
    'ConfiguredNextVersionVersionStrategyTests': (1, 7, 1),
}
for suite, counts in suites_to_counts.items():
    source = f'src/GitVersion.Core.Tests/VersionCalculation/Strategies/{suite}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    methods = {}
    for attributes, method in re.findall(r'((?:^    \[Test[^\n]+\n)+)    public void (\w+)\(', raw.decode(), re.M):
        if attributes.strip() == '[Test]':
            args = [[]]
        else:
            args = [
                json.loads('[' + re.sub(r'SemanticVersionFormat\.(Strict|Loose)', r'"\1"', line) + ']')
                for line in re.findall(r'\[TestCase\((.*?)\)\]', attributes)
            ]
        methods[method] = dict(cases=[dict(id=f'case-{i:03}', arguments=value) for i, value in enumerate(args)])
    assert tuple(len(record['cases']) for record in methods.values()) == counts
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)

# The seven valid configured versions also run through the two real executables.
next_methods = suites['ConfiguredNextVersionVersionStrategyTests']['methods']
recipes = []
for case in next_methods['ConfiguredNextVersionTest']['cases']:
    value, _, version_format, _ = case['arguments']
    recipes.append(dict(
        id='strategy-next-' + case['id'],
        description='ConfiguredNextVersionVersionStrategyTests.ConfiguredNextVersionTest ' + case['id'],
        config=f'calculation:\n  semantic-version-format: {version_format}\n  next-version: {json.dumps(value)}\n',
        steps=[dict(op='commit', id='root'), dict(op='checkpoint', id='head')],
    ))
(root / 'testdata/csharp-configured-next-strategy-recipes.json').write_text(json.dumps(recipes, indent=2) + '\n')

target = 'testdata/csharp-branch-and-next-strategies.json'
payload = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / target).write_text(payload)
for suite, entry in suites.items():
    for method, record in entry['methods'].items():
        cases = record['cases']
        key = f'{suite}.{method}'
        item = dict(source=entry['source'], source_sha256=entry['sha256'], method=method,
                    go_test='TestCSharpBranchAndNextStrategies', case_file=target,
                    case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
                    case_ids=[case['id'] for case in cases],
                    adaptations=[
                        'A deterministic go-git fixture creates real local branches and reproduces the remote-tracking reference created by fetch. The calculator workflow candidate retains the original semantic version and cardinality.'
                        if suite.startswith('VersionInBranchName') else
                        'The calculator represents the version operator by its applied increment and resulting version. Assert the absent source, increment kind, label and number change or identity, exact resulting version, and typed safe diagnostic.',
                    ], status='ported-awaiting-platform-verification')
        if key in mapping:
            old = mapping[key]
            assert all(old[k] == v for k, v in item.items() if k != 'status'), key
            item['status'] = old['status']
        mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('7 original methods and 29 parameter cases')
