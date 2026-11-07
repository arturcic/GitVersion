#!/usr/bin/env python3
"""Extract the pinned provider, resolver, and override helper cases."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parent.parent
revision = json.loads((root / 'baseline.json').read_text())['revision']
manifest_path = root / 'testdata/csharp-unit-port-mappings.json'
manifest = json.loads(manifest_path.read_text())
target = 'testdata/csharp-configuration-resolver.json'
suites = {}
excluded = {
    # Exact YAML approval and CLR alias reflection are still pending.
    'CanWriteOutEffectiveConfiguration', 'VerifyAliases',
    # These six already have source mappings.
    'OverwritesDefaultsWithProvidedConfig', 'CombineVersionStrategyConfigNextAndTaggedCommit',
    'CanRemoveLabel', 'NextVersionCanHavePatch',
    'ShouldUseDefaultSourceBranchesWhenNotSpecifiedForDevelop',
    'ShouldUseDefaultSourceBranchesWhenNotSpecifiedForFeature',
}
for suite in ['ConfigurationProviderTests', 'ConfigurationResolverTests', 'ConfigurationHelperTests']:
    source = f'src/GitVersion.Configuration.Tests/Configuration/{suite}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    text = raw.decode()
    digest = hashlib.sha256(raw).hexdigest()
    methods = {}
    pattern = r'((?:^    \[[^\n]+\]\n)+)    public void (\w+)\(([^\n]*)\)\n    \{'
    for match in re.finditer(pattern, text, re.M):
        attrs, method, signature = match.groups()
        if '[Test' not in attrs or method in excluded:
            continue
        params = []
        for row in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M):
            row = row.replace('ConfigurationConstants.MainBranchKey', '"main"').replace('ConfigurationConstants.DevelopBranchKey', '"develop"')
            params.append(json.loads('[' + row + ']'))
        if '[Values] bool' in signature:
            params = [[True], [False]]
        body = text[match.end():].split('\n    }', 1)[0]
        block = re.search(r'(?:const string|var) text\s*=\s*\$?"""\n(.*?)\n( +)""";', body, re.S)
        literal = re.search(r'const string text\s*=\s*(@?)("(?:[^"\\]|\\.)*");', body)
        yaml_text = None
        if block:
            yaml_text = '\n'.join(line.removeprefix(block[2]) for line in block[1].splitlines())
        elif literal:
            yaml_text = literal[2][1:-1] if literal[1] else json.loads(literal[2])
        methods[method] = {'document': yaml_text, 'cases': [
            {'id': f'case-{i:03d}', 'arguments': args} for i, args in enumerate(params or [[]])
        ]}
    suites[suite] = {'source': source, 'sha256': digest, 'methods': methods}

assert [len(x['methods']) for x in suites.values()] == [31, 6, 3]
presets = {}
for name in ['GitFlow/v1', 'GitHubFlow/v1', 'TrunkBased/preview1']:
    source = 'docs/input/docs/workflows/' + name + '.yml'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    path = 'internal/config/presets/' + name + '.yml'
    assert (root / path).read_bytes() == raw
    presets[path] = {'source': source, 'sha256': hashlib.sha256(raw).hexdigest()}
data = json.dumps({'revision': revision, 'suites': suites, 'presets': presets}, indent=2) + '\n'
(root / target).write_text(data)
for name, suite in suites.items():
    for method, item in suite['methods'].items():
        key = name + '.' + method
        entry = {
            'source': suite['source'], 'source_sha256': suite['sha256'], 'method': method,
            'go_test': 'TestCSharpConfigurationResolution', 'case_file': target,
            'case_file_sha256': hashlib.sha256(data.encode()).hexdigest(),
            'case_ids': [case['id'] for case in item['cases']],
            'adaptations': [
                'Ordered YAML nodes represent public aliases and C# typed dictionaries. The typed-enum and collection case uses Go enum and set values encoded as runtime overrides.',
                'The Go serializer checks deterministic legacy equivalence and round trips. Branch collection assertions preserve source order.',
            ],
            'status': 'ported-awaiting-platform-verification',
        }
        if key in manifest:
            previous = manifest[key]
            assert all(previous[k] == value for k, value in entry.items() if k != 'status'), key
            entry['status'] = previous['status']
        manifest[key] = entry
manifest_path.write_text(json.dumps(manifest, indent=2) + '\n')
print('40 methods and', sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()), 'original cases')
