#!/usr/bin/env python3
"""Inventory the pinned Git index and stream cases."""
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
target = 'testdata/csharp-git-index-and-streams.json'
suites = {}
for suite, count in [('GitIndexTests', 4), ('DeltaStreamReaderTests', 3), ('RandomAccessStreamTests', 5), ('GitZLibStreamTests', 2)]:
    source = f'src/GitVersion.Git.Managed.Tests/{suite}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    methods = {}
    for attrs, name in re.findall(r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\(', raw.decode(), re.M):
        args = [json.loads(value) for value in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M)]
        assert args or attrs.strip() == '[Test]'
        methods[name] = dict(cases=[dict(id=f'case-{i:03}', argument=arg) for i, arg in enumerate(args or [None])])
    assert len(methods) == count
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)
assert sum(len(m['cases']) for entry in suites.values() for m in entry['methods'].values()) == 19
doc = dict(revision=revision, suites=suites)
if not inputs_only:
    ref = 'testdata/csharp-git-index-reference.json'
    raw = (root / ref).read_bytes()
    assert json.loads(raw)['revision'] == revision
    doc.update(reference=ref, reference_sha256=hashlib.sha256(raw).hexdigest())
payload = json.dumps(doc, indent=2) + '\n'
(root / target).write_text(payload)
if inputs_only:
    raise SystemExit(0)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
adaptations = {
    'GitIndexTests': 'The production index storage supplies entries. The supplemental assume-valid reader preserves the bit omitted by go-git. Independent Git ls-files expectations retain version, mode, SHA, stage and path checks.',
    'DeltaStreamReaderTests': 'The production go-git delta reader exposes output instead of instruction records. A base with unique bytes proves exact copy offset and size; a valid empty target and truncated instruction retain EOF and rejection behavior.',
    'RandomAccessStreamTests': 'The Go pack implementation uses bounded io.SectionReader views. A caller-owned one-byte buffer replaces ReadByte; Go EOF, independent offsets, a live shared file and zero measured allocations preserve the original behavior.',
    'GitZLibStreamTests': 'The exact C# payload and three invalid headers exercise the go-git zlib reader used by loose and packed objects.',
}
for suite, entry in suites.items():
    for method, item in entry['methods'].items():
        key = f'{suite}.{method}'
        row = dict(source=entry['source'], source_sha256=entry['sha256'], method=method, go_test='TestCSharpGitIndexAndStreams',
                   case_file=target, case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
                   case_ids=[c['id'] for c in item['cases']], adaptations=[adaptations[suite]],
                   status='ported-awaiting-platform-verification')
        if key in mapping:
            old = mapping[key]
            assert all(old[k] == v for k, v in row.items() if k != 'status'), key
            row['status'] = old['status']
        mapping[key] = row
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('14 original methods and 19 cases')
