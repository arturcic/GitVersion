#!/usr/bin/env python3
"""Inventory the pinned reference, status, and tree-diff tests."""
import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('--inputs-only', action='store_true')
inputs_only = parser.parse_args().inputs_only
root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
target = 'testdata/csharp-repository-contracts.json'
suites = {}
for name, count in [('GitReferenceStoreTests', 12), ('GitStatusCalculatorTests', 12), ('GitTreeDiffTests', 6)]:
    source = f'src/GitVersion.Git.Managed.Tests/{name}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    code = raw.decode()
    declarations = list(re.finditer(r'^    \[Test\]\n    public void (\w+)\(\)', code, re.M))
    assert len(declarations) == count
    methods = {}
    for index, declaration in enumerate(declarations):
        case = dict(id='case-000', arguments=[])
        if name == 'GitStatusCalculatorTests':
            end = declarations[index + 1].start() if index + 1 < count else len(code)
            expected = re.findall(r'AssertParity\(repository, expectedCount: (\d+)\)', code[declaration.end():end])
            assert len(expected) == 1
            case['expected_count'] = int(expected[0])
        methods[declaration[1]] = dict(cases=[case])
    suites[name] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)
doc = dict(revision=revision, suites=suites)
if not inputs_only:
    reference_path = 'testdata/csharp-repository-contracts-reference.json'
    raw_reference = (root / reference_path).read_bytes()
    reference = json.loads(raw_reference)
    assert reference['revision'] == revision and len(reference['cases']) == 19
    doc.update(reference=reference_path, reference_sha256=hashlib.sha256(raw_reference).hexdigest())
payload = json.dumps(doc, indent=2) + '\n'
(root / target).write_text(payload)
if inputs_only:
    raise SystemExit(0)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
adaptations = {
    'GitReferenceStoreTests': 'The same go-git reference access used by the calculator exposes direct, symbolic, and packed state. Git for-each-ref and rev-parse expectations are captured from the deterministic fixtures.',
    'GitStatusCalculatorTests': 'Deterministic go-git commits and staged state preserve the original edits, including the v4 index and symlinks. Exact original counts and the independent pinned LibGit2Sharp status results are asserted.',
    'GitTreeDiffTests': 'The original nested paths, file type changes, raw byte ordering, modes, and merge parents are retained. Ordered expected paths for every commit are captured from the original LibGit2Sharp expression.',
}
for name, suite in suites.items():
    for method in suite['methods']:
        key = f'{name}.{method}'
        item = dict(source=suite['source'], source_sha256=suite['sha256'], method=method,
                    go_test='TestCSharpRepositoryContracts', case_file=target,
                    case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(), case_ids=['case-000'],
                    adaptations=[adaptations[name]], status='ported-awaiting-platform-verification')
        if key in mapping:
            old = mapping[key]
            assert all(old[k] == v for k, v in item.items() if k != 'status'), key
            item['status'] = old['status']
        mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('30 methods, 30 original cases and 19 captured oracle cases')
