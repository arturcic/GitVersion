#!/usr/bin/env python3
"""Extract the pinned C# migration and configuration-version cases."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parent.parent
revision = json.loads((root / 'baseline.json').read_text())['revision']
app = 'src/GitVersion.App.Tests/'
config = 'src/GitVersion.Configuration.Tests/Configuration/'
sources = {
    config + 'ConfigurationMigrationServiceTests.cs': None,
    app + 'ConfigurationMigrationExecutorTests.cs': None,
    app + 'ConfigurationVersionIntegrationTests.cs': None,
    app + 'ArgumentParserTests.cs': {
        'ConfigMigrateParsesItsInputAndOutputOptions', 'ConfigMigrateUsesPositionalTargetPath',
        'ConfigMigrateRejectsOutputAndInPlaceTogether', 'ConfigRequiresASubcommand',
    },
    app + 'LegacyArgumentParserTests.cs': {'ConfigMigrateIsRejectedAsASubcommand'},
}
suites = {}
for source, selected in sources.items():
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    text = raw.decode()
    methods = {}
    for match in re.finditer(r'((?:^    \[[^\n]+\]\n)+)    public (?:async )?(?:void|Task) (\w+)\(', text, re.M):
        attrs, name = match.groups()
        if '[Test' not in attrs or (selected is not None and name not in selected):
            continue
        params = [json.loads('[' + row + ']') for row in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M)]
        body = text[match.end():].split('\n    }', 1)[0]
        block = re.search(r'const string input\s*=\s*"""\n(.*?)\n( +)""";', body, re.S)
        literal = re.search(r'const string input\s*=\s*("(?:[^"\\]|\\.)*");', body)
        document = None
        if block:
            document = '\n'.join(line.removeprefix(block[2]) for line in block[1].splitlines())
        elif literal:
            document = json.loads(literal[1])
        methods[name] = {'document': document, 'cases': [
            {'id': f'case-{i:03d}', 'arguments': args} for i, args in enumerate(params or [[]])
        ]}
    suites[Path(source).stem] = {
        'source': source, 'sha256': hashlib.sha256(raw).hexdigest(), 'methods': methods,
    }
assert [len(s['methods']) for s in suites.values()] == [12, 2, 17, 4, 1]
target = 'testdata/csharp-configuration-migration.json'
data = json.dumps({'revision': revision, 'suites': suites}, indent=2) + '\n'
(root / target).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
manifest = json.loads(path.read_text())
for name, suite in suites.items():
    for method, item in suite['methods'].items():
        key = name + '.' + method
        entry = {
            'source': suite['source'], 'source_sha256': suite['sha256'], 'method': method,
            'go_test': 'TestCSharpConfigurationMigration' if name == 'ConfigurationMigrationServiceTests' else 'TestCSharpConfigurationMigrationCLI',
            'case_file': target, 'case_file_sha256': hashlib.sha256(data.encode()).hexdigest(),
            'case_ids': [case['id'] for case in item['cases']],
            'adaptations': [
                'Go error types represent C# YAML and layout exceptions. Configured fields are inspected through normalized YAML nodes.',
            ] if name == 'ConfigurationMigrationServiceTests' else [
                'Go command dispatch exposes returned errors and captured stderr in place of the C# program runner. Equivalent go-git fixtures supply HEAD and exercise the production command.',
            ],
            'status': 'ported-awaiting-platform-verification',
        }
        if key in manifest:
            previous = manifest[key]
            assert all(previous[k] == value for k, value in entry.items() if k != 'status'), key
            entry['status'] = previous['status']
        manifest[key] = entry
path.write_text(json.dumps(manifest, indent=2) + '\n')
print('36 methods and', sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()), 'original cases')
