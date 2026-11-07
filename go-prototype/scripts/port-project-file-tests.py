#!/usr/bin/env python3
"""Extract NUnit parameters and expected XML from the pinned project updater suite."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import textwrap

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
source = 'src/GitVersion.Output.Tests/Output/ProjectFileUpdaterTests.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
source_sha = hashlib.sha256(raw).hexdigest()
framework = re.search(r'const string TargetFramework = "([^"]+)"', raw.decode())[1]
raw_literal = re.compile(r'\$?"""\n(.*?)\n +"""', re.S)

def xml_text(literal, sdk):
    match = raw_literal.search(literal)
    if not match:
        raise ValueError(f'missing XML literal in {literal}')
    return textwrap.dedent(match[1]).replace('{TargetFramework}', framework).replace('{sdk}', sdk)

methods = {}
pattern = r'^    (\[TestCase\(.*?)(?=^    public void )    public void (\w+)\([^\n]*\)\n    \{(.*?)^    \}'
for attributes, method, body in re.findall(pattern, raw.decode(), re.M | re.S):
    cases = []
    for i, args in enumerate(re.findall(r'\[TestCase\((.*?)\)\]', attributes, re.S)):
        case = dict(id=f'case-{i:03d}')
        if raw_literal.search(args):
            case['xml'] = xml_text(args, '')
        else:
            values = json.loads(f'[{args}]')
            case['xml'] = xml_text(body, values[0])
            if len(values) == 2:
                case['file_name'] = values[1]
        if 'canUpdate.ShouldBe(' in body:
            case['can_update'] = 'canUpdate.ShouldBe(true)' in body
            warning = re.search(r'\.ShouldContain\("([^"]+)"\)', body)
            case['warning'] = warning[1] if warning else ''
        elif 'xmlRoot.ToString().ShouldBe' in body:
            case['expected_xml'] = xml_text(body, '')
        elif 'transformedXml.ShouldBe' in body:
            case['expected_xml'] = xml_text(body[body.index('var expectedXml'):], values[0])
        else:
            raise ValueError(f'missing assertions for {method}')
        cases.append(case)
    methods[method] = cases
assert len(methods) == 10 and sum(map(len, methods.values())) == 17
case_file = 'testdata/csharp-project-file.json'
data = json.dumps(dict(revision=revision, source=source, source_sha256=source_sha, methods=methods), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for method, cases in methods.items():
    key = f'ProjectFileUpdaterTests.{method}'
    entry = dict(
        source=source, source_sha256=source_sha, method=method,
        go_test='TestCSharpProjectFileUpdater', case_file=case_file,
        case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
        case_ids=[case['id'] for case in cases],
        adaptations=['Use the Go variable provider and temporary project files in place of mocked filesystem calls. '
                     'Retain the original eligibility, warning, and rendered XML assertions.'],
        status='ported-awaiting-platform-verification',
    )
    if key in mappings:
        entry['status'] = mappings[key]['status']
        assert entry == mappings[key], key
    mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
