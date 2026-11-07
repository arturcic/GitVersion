#!/usr/bin/env python3
"""Extract original C# executor cache and worktree discovery cases."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import textwrap

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
targets = {
    'src/GitVersion.Core.Tests/Core/GitVersionExecutorTests.cs': (
        'ConfigurationVersionChangeInvalidatesCache', 'ResolvedConfiguration_IsCreatedOncePerExecution',
        'CacheFileExistsOnDisk', 'CacheFileExistsOnDiskWhenOverrideConfigIsSpecifiedVersionShouldBeDynamicallyCalculatedWithoutSavingInCache',
        'CacheFileIsMissing', 'ConfigChangeInvalidatesCache', 'NoCacheBypassesCache',
        'GetProjectRootDirectoryWorkingDirectoryWithWorktree', 'GetProjectRootDirectoryNoWorktree',
    ),
    'src/GitVersion.App.Tests/ExecCmdLineArgumentTest.cs': ('CommitArgumentUsesRequestedCommitWhenCacheExists',),
}
def show(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

locator_source = 'src/GitVersion.Configuration/ConfigurationFileLocator.cs'
locator = show(locator_source)
file_names = dict(re.findall(r'const string (\w+) = "([^"]+)";', locator.decode()))
for name, base in re.findall(r'const string (\w+) = \$"\.\{(\w+)\}";', locator.decode()):
    file_names[name] = '.' + file_names[base]
suites = {}
cache_data = None
for source, selected in targets.items():
    raw = show(source)
    text = raw.decode()
    suite = Path(source).stem
    if suite == 'GitVersionExecutorTests':
        cache_data = json.loads(textwrap.dedent(re.search(r'versionCacheFileContent =\s*"""(.*?)"""', text, re.S)[1]))
    methods = {}
    for attrs, method, body in re.findall(
        r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\([^\n]*\)\n    \{(.*?)^    \}', text, re.M | re.S
    ):
        if method not in selected:
            continue
        rows = []
        for args in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M):
            if args.startswith('ConfigurationFileLocator.'):
                rows.append([file_names[args.split('.')[1]]])
            else:
                rows.append(json.loads(f'[{args}]'))
        assemblies = re.findall(r'\.AssemblySemVer\.ShouldBe\("([^"]+)"\)', body)
        methods[method] = dict(cases=[dict(id=f'case-{i:03d}', arguments=args, assemblies=assemblies)
                                    for i, args in enumerate(rows or [[]])])
    assert set(methods) == set(selected), source
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 10
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 14
case_file = 'testdata/csharp-executor-cache.json'
data = json.dumps(dict(revision=revision, locator_source=locator_source, locator_source_sha256=hashlib.sha256(locator).hexdigest(),
                       cache_payload=cache_data, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, details in suites.items():
    for method, cases in details['methods'].items():
        key = f'{suite}.{method}'
        if method.startswith('GetProjectRootDirectory'):
            adaptation = 'Run go-git repository discovery on ordinary and linked checkout paths. The original supplies a URL but performs no remote preparation or access.'
        elif method == 'ResolvedConfiguration_IsCreatedOncePerExecution':
            adaptation = 'Record the Go configuration loader call and assert that calculation and build-number output use the single resolved configuration.'
        else:
            adaptation = 'Use real go-git histories and the root CLI. Preserve the original C# cache payload, configuration names, log assertions, override behavior and both explicit commit parser cases. Go uses independent cache filenames.'
        entry = dict(source=details['source'], source_sha256=details['sha256'], method=method,
                     go_test='TestCSharpExecutorCache', case_file=case_file,
                     case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in cases['cases']], adaptations=[adaptation],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert mappings[key] == entry, key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
