#!/usr/bin/env python3
"""Inventory the pinned repository commit log tests and their source."""
import hashlib
import json
import re
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
suite = 'RepositoryStoreCommitLogTests'
source = f'src/GitVersion.Core.Tests/Core/{suite}.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
declarations = re.findall(r'^    \[Test\]\n    public void (\w+)\(\)', raw.decode(), re.M)
assert len(declarations) == 9
methods = {name: [dict(id='case-000', arguments=[])] for name in declarations}
sha = hashlib.sha256(raw).hexdigest()
target = 'testdata/csharp-repository-commit-log.json'
payload = json.dumps(dict(revision=revision, source=source, source_sha256=sha, methods=methods), indent=2) + '\n'
(root / target).write_text(payload)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
for method, cases in methods.items():
    key = f'{suite}.{method}'
    item = dict(source=source, source_sha256=sha, method=method, go_test='TestCSharpRepositoryStoreCommitLog',
                case_file=target, case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
                case_ids=[case['id'] for case in cases],
                adaptations=['The deterministic Go fixture preserves the original two merges, branch tips, commit order, and optional unmerged branch. The independent Go revision walk supplies the original oracle expression. A counting walk callback verifies the production projection and increment path reuse and eviction.'],
                status='ported-awaiting-platform-verification')
    if key in mapping:
        old = mapping[key]
        assert all(old[k] == v for k, v in item.items() if k != 'status'), key
        item['status'] = old['status']
    mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('9 original methods and cases, with full base/ignored/excluded commit matrices')
