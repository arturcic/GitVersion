#!/usr/bin/env python3
"""Record the pinned layout, Git config, and object ID test cases."""
import hashlib
import json
import re
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
target = 'testdata/csharp-managed-repository.json'
suites = {}
for name, method_count, case_count in [
    ('GitObjectIdTests', 17, 23), ('GitRepositoryLayoutTests', 8, 8),
    ('GitConfigurationFileTests', 6, 6),
]:
    source = f'src/GitVersion.Git.Managed.Tests/{name}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    code = raw.decode()
    constants = {key: json.loads(value) for key, value in re.findall(r'const string (\w+) = ("[^"]*");', code)}
    methods = {}
    for match in re.finditer(r'((?:^    \[[^\n]*\]\n)+)    public void (\w+)\(', code, re.M):
        attrs, method = match.groups()
        args = [json.loads('[' + re.sub(r'\bSha1Hex\b', json.dumps(constants.get('Sha1Hex', '')), row) + ']')
                for row in re.findall(r'\[TestCase\(([^\n]+)\)\]', attrs)]
        methods[method] = {'cases': [
            {'id': f'case-{i:03d}', 'arguments': values}
            for i, values in enumerate(args or [[]])
        ]}
    assert len(methods) == method_count
    assert sum(len(method['cases']) for method in methods.values()) == case_count
    suites[name] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(),
                        constants=constants, methods=methods)

payload = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / target).write_text(payload)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
adaptations = {
    'GitObjectIdTests': 'Go comparable values and dictionary lookups preserve equality, inequality, and equal-key hashing. Typed Go errors distinguish length, format, and prefix bounds.',
    'GitRepositoryLayoutTests': 'Go-git creates the commit graph, clone, and stores. Linked worktree metadata and unsupported format markers reproduce the original on-disk layouts without requiring a Git executable on Ubuntu.',
    'GitConfigurationFileTests': 'The repository reader uses go-git config decoding and the same file lookup path used by the tests. Both original assertions are retained in the comment and whitespace methods.',
}
for name, suite in suites.items():
    for method, spec in suite['methods'].items():
        key = f'{name}.{method}'
        item = dict(source=suite['source'], source_sha256=suite['sha256'], method=method,
                    go_test='TestCSharpManagedRepository', case_file=target,
                    case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
                    case_ids=[case['id'] for case in spec['cases']],
                    adaptations=[adaptations[name]], status='ported-awaiting-platform-verification')
        if key in mapping:
            old = mapping[key]
            assert all(old[k] == v for k, v in item.items() if k != 'status'), key
            item['status'] = old['status']
        mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('31 methods, 37 original cases')
