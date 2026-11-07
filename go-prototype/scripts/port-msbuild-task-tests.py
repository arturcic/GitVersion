#!/usr/bin/env python3
"""Extract MSBuild assembly, version-property, and build-log cases."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import textwrap

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']

def show(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

property_source = 'src/GitVersion.MsBuild/Tasks/GetVersion.cs'
property_bytes = show(property_source)
properties = re.findall(r'\[Output\]\s+public string (\w+) ', property_bytes.decode())
assert len(properties) == 33
suites = {}
for suite in ('UpdateAssemblyInfoTaskTest', 'WriteVersionInfoTest', 'GetVersionTaskTests'):
    source = f'src/GitVersion.MsBuild.Tests/Tasks/{suite}.cs'
    raw = show(source)
    text = raw.decode()
    methods = {}
    for attrs, method, body in re.findall(
        r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\([^\n]*\)\n    \{(.*?)^    \}', text, re.M | re.S
    ):
        rows = []
        for args in re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M):
            args = re.sub(r'nameof\(GitVersionVariables\.(\w+)\)', r'"\1"', args)
            rows.append(json.loads(f'[{args}]'))
        if 'nameof(Languages)' in attrs:
            block = re.search(r'Languages =\s*\[(.*?)\];', text, re.S)[1]
            rows = [[json.loads(v)] for v in re.findall(r'new object\[\] \{ ("[^"]+") \}', block)]
        cases = []
        for i, values in enumerate(rows or [[]]):
            case = dict(id=f'case-{i:03d}', arguments=values,
                        azure='InAzurePipeline' in body, github='InGitHubActions' in body)
            if suite == 'UpdateAssemblyInfoTaskTest':
                case['attribute'] = re.search(r'fileContent.ShouldContain\("""(.*?)"""\)', body)[1]
            elif suite == 'WriteVersionInfoTest':
                case['contains'] = re.findall(r'(?:Log|Output|content)\.ShouldContain\("([^"]+)"\)', body)
                case['excludes'] = re.findall(r'(?:Log|Output)\.ShouldNotContain\("([^"]+)"\)', body)
                config = re.search(r'const string content = """\n(.*?)\n +""";', body, re.S)
                if config:
                    case['config'] = textwrap.dedent(config[1])
                if method == 'WriteVersionInfoTaskShouldUpdateBuildNumberInAzurePipeline':
                    case['contains'] = ['##vso[build.updatebuildnumber]' + values[1]]
            else:
                case['properties'] = dict(re.findall(r'result\.Task\.(\w+)\.ShouldBe\("([^"]*)"\)', body))
                case['properties'].update({name: '' for name in re.findall(r'result\.Task\.(\w+)\.ShouldBe\(string.Empty\)', body)})
                case['equal_properties'] = [list(pair) for pair in re.findall(r'result\.Task\.(\w+)\.ShouldBe\(result\.Task\.(\w+)\)', body)]
                case['nonempty_properties'] = re.findall(r'result\.Task\.(\w+)\.ShouldNotBeNullOrEmpty\(', body)
            cases.append(case)
        methods[method] = dict(cases=cases)
    suites[suite] = dict(source=source, sha256=hashlib.sha256(raw).hexdigest(), methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 20
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 51
case_file = 'testdata/csharp-msbuild-tasks.json'
data = json.dumps(dict(revision=revision, property_source=property_source,
                       property_source_sha256=hashlib.sha256(property_bytes).hexdigest(),
                       properties=properties, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for suite, details in suites.items():
    for method, item in details['methods'].items():
        key = f'{suite}.{method}'
        engine = 'WhenRunWithMsBuild' in method
        entry = dict(
            source=details['source'], source_sha256=details['sha256'], method=method,
            go_test='TestPlatformCSharpMSBuildTasks' if engine else 'TestCSharpMSBuildTasks',
            case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
            case_ids=[c['id'] for c in item['cases']],
            adaptations=['Run the actual MSBuild engine through the imported Go target and preserve the version file and task output assertions.']
            if engine else ['Use a GitVersion JSON file, go-git fixture histories, and the Go task command. Retain the original version, path, output, and failure assertions.'],
            status='ported-awaiting-platform-verification',
        )
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert entry == mappings[key], key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2)+'\n')
