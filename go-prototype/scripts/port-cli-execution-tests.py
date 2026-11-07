#!/usr/bin/env python3
"""Extract pinned CLI execution cases and validation JSON approval strings."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
suites = {}
for suite, go_test in {
    'ExecCmdLineArgumentTest': 'TestCSharpCLIExecution',
    'ConfigurationValidationJsonTests': 'TestCSharpValidationJSON',
}.items():
    source = f'src/GitVersion.App.Tests/{suite}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    methods = {}
    pattern = r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\([^{}]*?\)\n    \{(.*?)^    \}'
    for attributes, method, body in re.findall(pattern, raw.decode(), re.M | re.S):
        cases = []
        for i, args in enumerate(re.findall(r'^    \[TestCase\((.*)\)\]$', attributes, re.M) or ['']):
            case = dict(id=f'case-{i:03d}', arguments=json.loads(f'[{args}]'))
            if suite == 'ConfigurationValidationJsonTests':
                case['approved'] = re.search(r'json.ShouldBe\("""(.*?)"""\)', body, re.S)[1]
            else:
                literal = re.search(r'ShouldContain\(\$?(".*?")', body)
                if literal:
                    case['contains'] = json.loads(literal[1])
            cases.append(case)
        methods[method] = dict(cases=cases)
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), go_test=go_test, methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 12
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 20
case_file = 'testdata/csharp-cli-execution.json'
data = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, detail in suites.items():
    for method, item in detail['methods'].items():
        if method == 'CommitArgumentUsesRequestedCommitWhenCacheExists':
            # The selection cases run in Go, but the persistent-cache precondition is pending.
            continue
        key = f'{suite}.{method}'
        entry = dict(source=detail['source'], source_sha256=detail['sha256'], method=method,
                     go_test=detail['go_test'], case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in item['cases']],
                     adaptations=['Use the Go CLI and equivalent temporary files or go-git histories.']
                     if suite == 'ExecCmdLineArgumentTest' else ['Use the Go validation envelope and retain the original byte-level assertion.'],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert entry == mappings[key], key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
