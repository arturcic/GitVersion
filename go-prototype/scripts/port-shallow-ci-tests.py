#!/usr/bin/env python3
"""Extract shallow and detached CI checkout contracts from the pinned C# suite."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
sources = [
    'src/GitVersion.Core.Tests/Core/GitVersionExecutorTests.cs',
    'src/GitVersion.App.Tests/PullRequestInBuildAgentTest.cs',
    'src/GitVersion.App.Tests/TagCheckoutInBuildAgentTests.cs',
]

def show(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

agent_constants = {}
for agent in ['AzurePipelines', 'BitBucketPipelines', 'CodeBuild', 'ContinuaCi', 'Drone',
              'GitHubActions', 'GitLabCi', 'Jenkins', 'MyGet', 'TeamCity', 'TravisCI']:
    raw = show(f'src/GitVersion.BuildAgents/Agents/{agent}.cs')
    for key, value in re.findall(r'const string (\w+) = ("[^"]*");', raw.decode()):
        agent_constants[agent + '.' + key] = json.loads(value)
agent_constants['TravisCi.EnvironmentVariableName'] = agent_constants['TravisCI.EnvironmentVariableName']

suites = {}
for source in sources:
    suite = Path(source).stem
    raw = show(source)
    text = raw.decode()
    methods = {}
    pr_values = {}
    if suite == 'PullRequestInBuildAgentTest':
        for name in ('PrMergeRefs', 'GitLabMergeRequestRefs'):
            group = re.search(rf'{name} =\s*\[(.*?)\];', text, re.S)
            pr_values[name] = [json.loads(s) for s in re.findall(r'"[^"]*"', group[1])]
    for attr, method, body in re.findall(
        r'^    (\[Test[^\n]*\])\n    public (?:async Task|void) (\w+)\([^\n]*\)\n    \{(.*?)^    \}', text, re.M | re.S
    ):
        if suite == 'GitVersionExecutorTests' and not (
            method.startswith('CalculateVersionVariables_') and ('Shallow' in method or 'Detached' in method)
        ):
            continue
        if suite == 'GitVersionExecutorTests':
            args = [{'expected': re.findall(r'\.ShouldBe\("([^"]+)"\)', body)}]
        elif method == 'VerifyPullRequestInput':
            args = []
            for row in re.findall(r'new object\[\] \{ (.*?) \}', text):
                args.append({'arguments': json.loads(f'[{row}]')})
        else:
            environment = {}
            variables = re.findall(r'\{\s*([\w.]+|"[^"]*"),\s*([\w.]+|"[^"]*")\s*\}', body)
            for key, value in variables:
                key = json.loads(key) if key.startswith('"') else agent_constants[key]
                if value == 'PullRequestBranchName':
                    value = '"PR-5"'
                environment[key] = json.loads(value) if value.startswith('"') else value
            expected = re.search(r'FullSemVer\.ShouldBe\("([^"]+)"\)', text)[1]
            refs = pr_values[re.search(r'nameof\((\w+)\)', attr)[1]] if pr_values else ['']
            args = [dict(ref=ref, expected=[expected], environment={
                key: ref if value in ('pullRequestRef', 'mergeRequestRef') else value
                for key, value in environment.items()
            }) for ref in refs]
        methods[method] = dict(cases=[dict(id=f'case-{index:03d}', **case) for index, case in enumerate(args)])
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 19
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 63
case_file = 'testdata/csharp-shallow-ci.json'
data = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, details in suites.items():
    for method, item in details['methods'].items():
        key = f'{suite}.{method}'
        test = 'TestCSharpShallowAndCI'
        if method == 'VerifyPullRequestInput':
            test = 'TestCSharpPullRequestInput'
        entry = dict(source=details['source'], source_sha256=details['sha256'], method=method,
                     go_test=test, case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in item['cases']],
                     adaptations=['Use local go-git repositories and preserve the original CI environment and version assertions. '
                                  'Go resolves the effective branch in memory and retains the existing checkout.'],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert entry == mappings[key], key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
