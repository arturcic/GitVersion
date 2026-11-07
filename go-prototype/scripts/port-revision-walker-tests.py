#!/usr/bin/env python3
"""Inventory every original revision-walker case from the pinned C# source."""
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
suite = 'GitRevisionWalkerTests'
source = f'src/GitVersion.Git.Managed.Tests/{suite}.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
code = raw.decode()
sorts = re.findall(r'"([^"]+)"', re.search(r'SortCombinations =\s*\[(.*?)\];', code, re.S)[1])
methods = {}
for attributes, method in re.findall(r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\(', code, re.M):
    if 'TestCaseSource(nameof(SortCombinations))' in attributes:
        arguments = sorts
    elif 'TestCase(' in attributes:
        arguments = re.findall(r'TestCase\((true|false)\)', attributes)
    else:
        assert attributes.strip() == '[Test]'
        arguments = ['']
    methods[method] = {'cases': [dict(id=f'case-{i:03}', argument=arg) for i, arg in enumerate(arguments)]}
assert len(methods) == 16 and sum(len(m['cases']) for m in methods.values()) == 53
sha = hashlib.sha256(raw).hexdigest()
doc = dict(revision=revision, source=source, source_sha256=sha, methods=methods)
target = 'testdata/csharp-revision-walker.json'
if not inputs_only:
    reference_path = 'testdata/csharp-revision-walker-reference.json'
    reference = (root / reference_path).read_bytes()
    assert json.loads(reference)['revision'] == revision
    assert len(json.loads(reference)['cases']) == 47
    doc.update(reference=reference_path, reference_sha256=hashlib.sha256(reference).hexdigest())
payload = json.dumps(doc, indent=2) + '\n'
(root / target).write_text(payload)
if inputs_only:
    raise SystemExit(0)

path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
for method, spec in methods.items():
    key = f'{suite}.{method}'
    item = dict(source=source, source_sha256=sha, method=method, go_test='TestCSharpRevisionWalker',
                case_file=target, case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
                case_ids=[case['id'] for case in spec['cases']],
                adaptations=['Deterministic go-git object graphs preserve original parent order, reference targets, and timestamp ties. The original LibGit2Sharp sort and merge-base expressions capture ordered SHA results. Shallow and missing-object cases retain the original storage assertions.'],
                status='ported-awaiting-platform-verification')
    if key in mapping:
        old = mapping[key]
        assert all(old[k] == v for k, v in item.items() if k != 'status'), key
        item['status'] = old['status']
    mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('16 original methods, 53 original cases and 47 captured oracle cases (48 queries)')
