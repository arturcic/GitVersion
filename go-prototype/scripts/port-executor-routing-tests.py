#!/usr/bin/env python3
"""Extract the pinned executor routing and branch-configuration cases."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
targets = {
    'src/GitVersion.App.Tests/GitVersionExecutorRoutingTests.cs': (
        'DisplayUsesVersionSpecificOverridesWithoutCalculating',
        'PropertyQueryUsesScalarSerializerAndSkipsDynamicRepositoryVerification',
        'LegacyPropertyQueryStillThrowsBeforeLoadingConfiguration',
        'NormalExecutionDiscoversCalculatesAndWritesOutputsInOrder',
        'ExpectedCalculationFailureReturnsOneAndWritesStderr',
    ),
    'src/GitVersion.Configuration.Tests/Configuration/ConfigurationExtensionsTests.cs': (
        'EnsureGetEffectiveConfigurationWithoutBranchUsesEmptyBranchConfiguration',
        'EnsureIsReleaseBranchWithReferenceNameWorksAsExpected',
    ),
}
suites = {}
values = None
for source, selected in targets.items():
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    text = raw.decode()
    methods = {}
    for attrs, method, body in re.findall(
        r'((?:^    \[Test(?:Case\(.*?\))?\]\n)+)    public void (\w+)\(.*?\)\n    \{(.*?)^    \}', text, re.M | re.S
    ):
        if method not in selected:
            continue
        rows = [json.loads(f'[{args}]') for args in re.findall(r'^    \[TestCase\((.*?)\)\]$', attrs, re.M | re.S)]
        if method == 'NormalExecutionDiscoversCalculatesAndWritesOutputsInOrder':
            constructor = re.search(r'new GitVersionVariables\((.*?)\);', body, re.S)[1]
            values = {name: json.loads(value) for name, value in re.findall(r'(\w+): ("[^"]*"|null)', constructor)}
            assert len(values) == 28
        methods[method] = dict(cases=[dict(id=f'case-{i:03d}', arguments=args) for i, args in enumerate(rows or [[]])])
    assert set(methods) == set(selected), (source, methods.keys())
    suites[Path(source).stem] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)
assert values is not None and sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 14
case_file = 'testdata/csharp-executor-routing.json'
data = json.dumps(dict(revision=revision, version_variables=values, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, details in suites.items():
    for method, item in details['methods'].items():
        key = f'{suite}.{method}'
        routing = suite == 'GitVersionExecutorRoutingTests'
        adaptation = ('Exercise the production Go routing functions through recording collaborators. Return Go errors from the executor and verify status and stderr at the process boundary. Go represents the resolved configuration and version variables with its own types.'
                      if routing else 'Use Go effective output and branch settings and the canonical reference constructor. Preserve the original format, weight and all reference and classification assertions.')
        entry = dict(source=details['source'], source_sha256=details['sha256'], method=method,
                     go_test='TestCSharpExecutorRouting' if routing else 'TestCSharpConfigurationExtensionRouting',
                     case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in item['cases']], adaptations=[adaptation],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert mappings[key] == entry, key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2)+'\n')
