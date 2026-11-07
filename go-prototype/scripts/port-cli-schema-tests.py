#!/usr/bin/env python3
"""Extract pinned CLI schema cases and their OpenCLI validation schema."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']

def show(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

suites = {}
for suite in ('CliSchemaTests', 'CliSchemaExporterTests'):
    source = f'src/GitVersion.App.Tests/{suite}.cs'
    raw = show(source)
    methods = {}
    for attrs, method in re.findall(
        r'((?:^    \[Test[^\n]*\]\n)(?:^    \[[^\n]*\]\n)*)    public (?:async Task|void) (\w+)\(',
        raw.decode(), re.M
    ):
        rows = [json.loads(f'[{args}]') for args in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M)]
        methods[method] = dict(cases=[dict(id=f'case-{i:03d}', arguments=args) for i, args in enumerate(rows or [[]])])
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 13
assert sum(len(method['cases']) for s in suites.values() for method in s['methods'].values()) == 47
schema_source = 'src/GitVersion.App.Tests/Fixtures/OpenCli/schema.json'
schema = show(schema_source)
case_file = 'testdata/csharp-cli-schema.json'
data = json.dumps(dict(revision=revision, schema_source=schema_source,
                       schema_source_sha256=hashlib.sha256(schema).hexdigest(),
                       schema=json.loads(schema), suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)

path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, details in suites.items():
    for method, entry_cases in details['methods'].items():
        key = f'{suite}.{method}'
        process = method.startswith('Process')
        adaptation = 'Run the pinned C# executable and the Go CLI in empty directories. Preserve parser, selector, output, and filesystem assertions. Compare the full schema with each executable reporting its own informational version.'
        if not process:
            adaptation = 'Use the Go command tree, syntax classifier, writer, and host entry point in place of System.CommandLine and the .NET host. Preserve schema structure and early exit assertions. Go represents option metadata as data without parsing callbacks or culture-dependent formatting.'
        entry = dict(source=details['source'], source_sha256=details['sha256'], method=method,
                     go_test='TestPlatformCSharpCLISchema' if process else 'TestCSharpCLISchema',
                     case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in entry_cases['cases']], adaptations=[adaptation],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert mappings[key] == entry, key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
