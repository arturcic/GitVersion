#!/usr/bin/env python3
"""Extract pinned configuration queries, display tests, and their cases."""
import hashlib
import itertools
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parent.parent
revision = json.loads((root / 'baseline.json').read_text())['revision']
prefix = 'src/GitVersion.Configuration.Tests/Configuration/'
sources = {
    prefix + 'ConfigurationPropertyQueryTests.cs': None,
    prefix + 'ConfigurationSerializerTests.cs': {
        'SerializesV7CalculationAndOutputSectionsAndRoundTrips',
        'SerializesFlatConfigurationWhenV6IsSelected',
        'Serialize_OrdersConfigurationPropertiesAndPreservesBranchNames',
    },
    prefix + 'ConfigurationProviderTests.cs': {'CanWriteOutEffectiveConfiguration'},
    'src/GitVersion.App.Tests/ConfigurationQueryIntegrationTests.cs': None,
}
skip = {'QueryNeverCallsVersionCalculationOrVersionOutput'}


def read(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


suites = {}
for source, selected in sources.items():
    raw = read(source)
    suite = Path(source).stem
    methods = {}
    # Read attributes and method names separately from multiline signatures.
    for match in re.finditer(r'((?:^    \[[^\n]+\]\n)+)    public (?:async )?(?:void|Task) (\w+)\(', raw.decode(), re.M):
        attrs, method = match.groups()
        if '[Test' not in attrs or method in skip or (selected is not None and method not in selected):
            continue
        cases = [json.loads('[' + row + ']') for row in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M)]
        if '[Combinatorial]' in attrs:
            signature = raw.decode()[match.end():].split('\n    {', 1)[0]
            dimensions = [json.loads('[' + row + ']') for row in re.findall(r'\[Values\((.*?)\)\]', signature)]
            assert len(dimensions) == 2
            cases = list(itertools.product(*dimensions))
        methods[method] = {'cases': [
            {'id': f'case-{i:03d}', 'arguments': args} for i, args in enumerate(cases or [[]])
        ]}
    suites[suite] = {'source': source, 'sha256': sha(raw), 'methods': methods}
assert [len(s['methods']) for s in suites.values()] == [8, 3, 1, 11]
approval = prefix + 'ConfigurationProviderTests.CanWriteOutEffectiveConfiguration.approved.txt'
expected = read(approval)
target = 'testdata/csharp-configuration-query.json'
data = json.dumps({
    'revision': revision, 'suites': suites,
    'approval': {'source': approval, 'sha256': sha(expected), 'yaml': expected.decode()},
}, indent=2) + '\n'
(root / target).write_text(data)
manifest_path = root / 'testdata/csharp-unit-port-mappings.json'
manifest = json.loads(manifest_path.read_text())
for name, suite in suites.items():
    for method, item in suite['methods'].items():
        key = name + '.' + method
        cli = name == 'ConfigurationQueryIntegrationTests'
        entry = {
            'source': suite['source'], 'source_sha256': suite['sha256'], 'method': method,
            'go_test': 'TestCSharpConfigurationQueryCLI' if cli else 'TestCSharpConfigurationQuery',
            'case_file': target, 'case_file_sha256': sha(data.encode()),
            'case_ids': [case['id'] for case in item['cases']],
            'adaptations': [
                'CLI execution uses Go command dispatch and Go Git fixtures. Both original backend environment values are exercised through the single pure-Go runtime.',
            ] if cli else [
                'Ordered YAML nodes stand in for the C# configuration objects. JSON scalar types, properties, nulls, errors, display order, and approved YAML are asserted.',
            ],
            'status': 'ported-awaiting-platform-verification',
        }
        if key in manifest:
            previous = manifest[key]
            assert all(previous[k] == value for k, value in entry.items() if k != 'status'), key
            entry['status'] = previous['status']
        manifest[key] = entry
manifest_path.write_text(json.dumps(manifest, indent=2) + '\n')
print('23 methods and', sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()), 'original cases')
