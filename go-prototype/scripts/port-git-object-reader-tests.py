#!/usr/bin/env python3
"""Inventory pinned loose object, pack, annotated tag and tree tests."""
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
target = 'testdata/csharp-git-object-readers.json'
suites = {}
for suite, count in [('LooseObjectTests', 11), ('PackedObjectTests', 5), ('TagTests', 4), ('TreeTests', 3)]:
    source = f'src/GitVersion.Git.Managed.Tests/{suite}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    methods = re.findall(r'^    \[Test\]\n    public void (\w+)\(\)', raw.decode(), re.M)
    assert len(methods) == count
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods={
        name: dict(cases=[dict(id='case-000')]) for name in methods
    })
doc = dict(revision=revision, suites=suites)
if not inputs_only:
    path = 'testdata/csharp-git-object-readers-reference.json'
    reference = (root / path).read_bytes()
    assert json.loads(reference)['revision'] == revision
    doc.update(reference=path, reference_sha256=hashlib.sha256(reference).hexdigest())
payload = json.dumps(doc, indent=2) + '\n'
(root / target).write_text(payload)
if inputs_only:
    raise SystemExit(0)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
adaptations = {
    'LooseObjectTests': 'Go-git fixtures preserve Git object identities, exact commit and blob bytes, distinct signatures and time zone, type checks and alternate storage. The original Git expressions are captured for offline comparison.',
    'PackedObjectTests': 'Go-git creates and indexes real packs, and the fixtures verify loose removal and the requested offset/reference delta types. The original commit lineage, messages and blob lines are asserted.',
    'TagTests': 'Go-git tag objects preserve the tagger, one-hop target, target type and full message. The application resolves nested tags via the same object storage.',
    'TreeTests': 'Go-git tree entry lookup preserves the original found and missing node behavior; a missing entry returns its typed error instead of a zero ID. Root metadata is compared in Git ls-tree order.',
}
for suite, entry in suites.items():
    for method in entry['methods']:
        key = f'{suite}.{method}'
        item = dict(source=entry['source'], source_sha256=entry['sha256'], method=method, go_test='TestCSharpGitObjectReaders',
                    case_file=target, case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(), case_ids=['case-000'],
                    adaptations=[adaptations[suite]], status='ported-awaiting-platform-verification')
        if key in mapping:
            old = mapping[key]
            assert all(old[k] == v for k, v in item.items() if k != 'status'), key
            item['status'] = old['status']
        mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('23 original object reader methods and cases')
