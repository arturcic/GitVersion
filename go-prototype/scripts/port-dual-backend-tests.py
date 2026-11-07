#!/usr/bin/env python3
"""Inventory the pinned dual-backend repository cases."""
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
suite = 'DualBackendParityTests'
source = f'src/GitVersion.Core.Tests/Core/{suite}.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
methods = re.findall(r'^    \[Test\]\n    public void (\w+)\(\)', raw.decode(), re.M)
assert len(methods) == 19
sha = hashlib.sha256(raw).hexdigest()
target = 'testdata/csharp-dual-backend.json'
doc = dict(revision=revision, source=source, source_sha256=sha,
           methods={name: [dict(id='case-000')] for name in methods})
if not inputs_only:
    reference = 'testdata/csharp-dual-backend-reference.json'
    captured = (root / reference).read_bytes()
    decoded = json.loads(captured)
    assert decoded['revision'] == revision and set(decoded['cases']) == set(methods)
    doc.update(reference=reference, reference_sha256=hashlib.sha256(captured).hexdigest())
payload = json.dumps(doc, indent=2) + '\n'
(root / target).write_text(payload)
if inputs_only:
    raise SystemExit(0)

path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
for method in methods:
    key = f'{suite}.{method}'
    row = dict(source=source, source_sha256=sha, method=method, go_test='TestCSharpDualBackendParity',
               case_file=target, case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
               case_ids=['case-000'],
               adaptations=['The original assertions run on a deterministic go-git fixture with the same history and layout. Both pinned C# adapters run against it and must agree before their result is used as the Go reference. Temporary paths are expressed relative to the fixture root.'],
               status='ported-awaiting-platform-verification')
    if key in mapping:
        old = mapping[key]
        assert all(old[k] == v for k, v in row.items() if k != 'status'), key
        row['status'] = old['status']
    mapping[key] = row
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('19 original dual-backend methods and cases')
