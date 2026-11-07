#!/usr/bin/env python3
"""Extract pinned repository and build-server execution assertions."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
suites = {}
specs = {
    'src/GitVersion.Core.Tests/Core/RepositoryStoreTests.cs': ('TestCSharpRepositoryExecution', (
        'FindsCorrectMergeBaseForForwardMerge',
        'FindsCorrectMergeBaseForForwardMergeMovesOn',
        'FindsCorrectMergeBaseForMultipleForwardMerges',
    )),
    'src/GitVersion.Core.Tests/Core/RepositoryStoreEqualTimestampTests.cs': ('TestCSharpRepositoryExecution', (
        'DerivedCommitLogReturnsTheSameCommitsWhenTimestampsAreEqual',
        'DerivedCommitLogIsAValidTopologicalOrderWhenTimestampsAreEqual',
    )),
    'src/GitVersion.Core.Tests/Core/GitVersionExecutorTests.cs': ('TestCSharpRepositoryExecutor', (
        'WorkingDirectoryWithoutGit', 'WorkingDirectoryWithoutCommits',
        'GetDotGitDirectoryNoWorktree', 'GetDotGitDirectoryWorktree',
        'CalculateVersionFromWorktreeHead', 'CustomVersionUsesBranchSpecificFormat',
    )),
    'src/GitVersion.App.Tests/JsonOutputOnBuildServerTest.cs': ('TestCSharpBuildServerJSON', (
        'BeingOnBuildServerDoesntOverrideOutputJson',
        'BeingOnBuildServerWithOutputJsonDoesNotFail',
        'BeingOnBuildServerWithOutputJsonAndOutputFileDoesNotFail',
    )),
}

for source, (go_test, selected) in specs.items():
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    suite = Path(source).stem
    methods = {}
    pattern = r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\([^{}]*?\)\n    \{(.*?)^    \}'
    for attrs, method, body in re.findall(pattern, raw.decode(), re.M | re.S):
        if method not in selected:
            continue
        cases = []
        for index, args in enumerate(re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M) or ['']):
            case = dict(id=f'case-{index:03d}', arguments=json.loads(f'[{args}]'))
            if suite == 'RepositoryStoreTests':
                case['expected_release_base'] = 'initial'
                releases = re.findall(r'MakeACommit\("release (\d)', body)
                case['expected_develop_base'] = 'release-' + releases[-2 if len(releases) == 3 else -1]
            if suite == 'RepositoryStoreEqualTimestampTests':
                bound = re.search(r'for \(var seed = 0; seed < (\d+); seed\+\+\)', body)
                case['seeds'] = list(range(int(bound[1])))
            if suite == 'GitVersionExecutorTests':
                expected = re.findall(r'\.Should(?:Be|Contain|StartWith)\("([^"]+)"\)', body)
                if expected:
                    case['expected'] = expected
            if suite == 'JsonOutputOnBuildServerTest':
                case['expected'] = re.search(r'"(0\.0\.1-5)"', body)[1]
            cases.append(case)
        methods[method] = dict(cases=cases)
    assert set(methods) == set(selected), (suite, methods.keys())
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), go_test=go_test, methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 14
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 15
case_file = 'testdata/csharp-repository-execution.json'
data = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
adaptations = {
    'RepositoryStoreTests': 'Build the original forward-merge histories with go-git and compare both argument orders through the branch-aware merge-base finder.',
    'RepositoryStoreEqualTimestampTests': 'Use all 25 original generator seeds and parent choices, with a fixed common timestamp. Check the entire set or every parent/child position for each base.',
    'GitVersionExecutorTests': 'Use the Go calculation and layout paths with temporary repositories. Check Git errors in place of .NET exception classes.',
    'JsonOutputOnBuildServerTest': 'Recreate the five initial commits, local checkout, and later remote tag and commit without contacting a server. Compare the original version, service message, and JSON file.',
}
for suite, details in suites.items():
    for method, item in details['methods'].items():
        key = f'{suite}.{method}'
        entry = dict(source=details['source'], source_sha256=details['sha256'], method=method,
                     go_test='TestCSharpRepositoryExecutorLayout' if method.startswith('GetDotGitDirectory') else details['go_test'], case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in item['cases']], adaptations=[adaptations[suite]],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert entry == mappings[key], key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
