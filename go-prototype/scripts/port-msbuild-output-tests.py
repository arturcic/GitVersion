#!/usr/bin/env python3
"""Extract MSBuild output fixtures and approvals from the pinned C# source."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import textwrap

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
sources = {
    'src/GitVersion.MsBuild.Tests/InvalidFileCheckerTests.cs': 'TestCSharpMSBuildOutput',
    'src/GitVersion.MsBuild.Tests/AssemblyInfoFileHelperTests.cs': 'TestCSharpMSBuildOutput',
    'src/GitVersion.MsBuild.Tests/Tasks/GenerateGitVersionInformationTest.cs': 'TestCSharpMSBuildOutput',
    'src/GitVersion.Output.Tests/Output/GitVersionInfoGeneratorTests.cs': 'TestCSharpMSBuildOutput',
    'src/GitVersion.Core.Tests/VersionCalculation/JsonVersionBuilderTests.cs': 'TestCSharpVersionJSON',
}

def show(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

suites = {}
for source, test in sources.items():
    raw = show(source)
    text = raw.decode()
    suite = Path(source).stem
    methods = {}
    pattern = r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\(([^\n]*)\)\n    \{(.*?)^    \}'
    for attrs, method, params, body in re.findall(pattern, text, re.M | re.S):
        cases = [json.loads(f'[{line}]') for line in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M)]
        if suite == 'InvalidFileCheckerTests':
            values = re.search(r'\[Values\((.*)\)\]', params)
            if values:
                cases = [[value] for value in json.loads('[' + values[1] + ']')]
        if suite == 'GenerateGitVersionInformationTest':
            language_source = re.search(r'Languages =\s*\[(.*?)\];', text, re.S)[1]
            cases = [[json.loads(value)] for value in re.findall(r'new object\[\] \{ ("[^"]+") \}', language_source)]
        data = []
        for index, arguments in enumerate(cases or [[]]):
            case = dict(id=f'case-{index:03d}', arguments=arguments)
            if suite == 'InvalidFileCheckerTests':
                name = re.search(r'ItemSpec = "([^"]+)"', body)[1]
                raw_content = re.search(r'writer.Write\("""\n(.*?)\n +"""', body, re.S)[1]
                content = textwrap.dedent(raw_content)
                if arguments:
                    content = content.replace('{0}', arguments[0]).replace('{{', '{').replace('}}', '}')
                warning = re.search(r'Is.EqualTo\("([^"]+)"\)', body)
                case.update(file_name=name, content=content, expected_error=warning[1] if warning else '')
            elif suite == 'GitVersionInfoGeneratorTests':
                approval = Path(source).parent / 'Approved' / arguments[0] / f'{suite}.{method}.approved.txt'
                case['approved'] = show(approval).decode()
            elif suite == 'GenerateGitVersionInformationTest':
                case['build_server'] = bool(re.search(r'ExecuteMsBuild(?:Task|Exe)InAzurePipeline', body))
                case['variables'] = dict(re.findall(r'nameof\(GitVersionVariables\.(\w+)\), "([^"]+)"', body))
                assert len(case['variables']) == 5
                namespace = re.search(r'ShouldContain\("namespace ([^"]+)"\)', body)
                if namespace:
                    case['namespace'] = namespace[1]
            elif suite == 'JsonVersionBuilderTests':
                approval = Path(source).parent / 'Approved' / f'{suite}.{method}.approved.txt'
                case['approved'] = show(approval).decode()
            data.append(case)
        methods[method] = dict(cases=data)
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), go_test=test, methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 37
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 110
case_file = 'testdata/csharp-msbuild-output.json'
data = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, details in suites.items():
    for method, item in details['methods'].items():
        key = f'{suite}.{method}'
        test = details['go_test']
        if 'WhenRunWithMsBuild' in method:
            test = 'TestPlatformCSharpMSBuildOutput'
        adaptation = 'Use equivalent go-git histories and temporary source files. Retain the original output and failure assertions.'
        if test == 'TestPlatformCSharpMSBuildOutput':
            adaptation = 'Run the actual MSBuild engine with the imported Go target. The target invokes the Go CLI and exposes the generated path.'
        entry = dict(source=details['source'], source_sha256=details['sha256'], method=method,
                     go_test=test, case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
                     case_ids=[c['id'] for c in item['cases']], adaptations=[adaptation],
                     status='ported-awaiting-platform-verification')
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert entry == mappings[key], key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2)+'\n')
