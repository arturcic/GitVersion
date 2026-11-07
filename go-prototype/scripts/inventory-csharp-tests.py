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
# Pinned declaration lines disambiguate overloads/nested-class name collisions.
# The denominator counts declarations and does not change.
from collections import Counter
identities = Counter(row['id'] for row in rows)
for row in rows:
    if identities[row['id']] > 1:
        row['source_method'] = row['method']
        row['method'] += '@' + str(row['line'])
        row['id'] = row['source'] + '#' + row['method']
assert len({row['id'] for row in rows}) == len(rows)
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
        row.update(status='verified-macos-ubuntu', go_test='TestConfigurationEdgeFrozenReference', fixtures=mappings[key])
        found.add(key)
assert found == set(mappings), 'A mapped C# test is absent from the pin'
# New ports carry explicit parameter/checkpoint inventories and source hashes.
ports = json.loads((root / 'testdata/csharp-port-mappings.json').read_text())
recipes = json.loads((root / 'testdata/csharp-calculation-ports.json').read_text())
recipe_by_id = {r['id']: r for r in recipes}
config_cases = {c['id']: c for c in json.loads((root / 'testdata/csharp-configuration-ports.json').read_text())}
used_recipes, used_configs, found_ports = set(), set(), set()
for row in rows:
    key = Path(row['source']).stem + '.' + row['method']
    if key not in ports:
        continue
    port = ports[key]
    assert row['source'] == port['source'] and row['source_sha256'] == port['source_sha256'], key
    assert port['cases'] and port['adaptations'], key
    checkpoints = []
    for case in port['cases']:
        if 'recipe' in case:
            recipe = recipe_by_id[case['recipe']]
            assert recipe['description'] == key, key
            expected = [recipe['id'] + '--' + step['id'] for step in recipe['steps'] if step['op'] == 'checkpoint']
            assert expected == case['checkpoints'] and all(step.get('semver') for step in recipe['steps'] if step['op'] == 'checkpoint'), key
            assert recipe['id'] not in used_recipes, key
            used_recipes.add(recipe['id']); checkpoints.extend(expected)
        else:
            assert case['configuration'] in config_cases, key
            assert config_cases[case['configuration']]['expected'], key
            assert case['configuration'] not in used_configs, key
            used_configs.add(case['configuration'])
    row.update(status=port['status'], go_test=port['go_test'], fixtures=checkpoints,
               port_cases=port['cases'], adaptations=port['adaptations'])
    found_ports.add(key)
assert found_ports == set(ports), 'A port source method is absent from the pin'
assert used_recipes == set(recipe_by_id) and used_configs == set(config_cases), 'Unmapped port cases'
found.update(found_ports)
unit_ports = json.loads((root / 'testdata/csharp-unit-port-mappings.json').read_text())
found_units = set()
for row in rows:
    key = Path(row['source']).stem + '.' + row['method']
    if key not in unit_ports:
        continue
    port = unit_ports[key]
    assert row['category'] == 'unit-or-component' and row['source'] == port['source'], key
    assert row['source_sha256'] == port['source_sha256'], key
    data = (root / port['case_file']).read_bytes()
    assert hashlib.sha256(data).hexdigest() == port['case_file_sha256'], key
    corpus = json.loads(data)
    assert corpus['revision'] == revision, key
    if 'suites' in corpus:
        suite = corpus['suites'][Path(row['source']).stem]
        assert suite['source'] == row['source'] and suite['sha256'] == row['source_sha256'], key
        cases = suite['methods'][row['method']]['cases']
    else:
        assert corpus['source_sha256'] == row['source_sha256'], key
        cases = corpus['methods'][row['method']]
    assert [case['id'] for case in cases] == port['case_ids'], key
    assert len(port['case_ids']) == len(set(port['case_ids'])) and port['case_ids'], key
    row.update(status=port['status'], go_test=port['go_test'], port_cases=port['case_ids'],
               case_file=port['case_file'], adaptations=port['adaptations'])
    found_units.add(key)
assert found_units == set(unit_ports), 'Missing unit test declaration'
found.update(found_units)
out = {'schema': 1, 'source_revision': revision,
       'kind': 'static attributed method inventory; not expanded NUnit discovery',
       'declarations': len(rows), 'mapped_declarations': len(found),
       'categories': {category: {'declarations': sum(r['category'] == category for r in rows),
                                'mapped': sum(r['category'] == category and r['go_test'] is not None for r in rows),
                                'verified': sum(r['category'] == category and r['status'] == 'verified-macos-ubuntu' for r in rows)}
                      for category in ['unit-or-component', 'integration']},
       'unit_target_percent': 90, 'unit_target_methods': 1365, 'tests': rows}
(root / 'testdata/csharp-test-inventory.json').write_text(json.dumps(out, indent=2) + '\n')
print(f'{len(rows)} attributed declarations; {len(found)} mapped methods; {len(used_recipes)} new recipes and {len(used_configs)} configuration cases')
