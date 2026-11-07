#!/usr/bin/env python3
"""Extract WiX, variable formatting and tag/branch filtering cases from C#."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
source_root = 'src/'
suites = {}

def pinned(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

for source, go_test in {
    'GitVersion.App.Tests/UpdateWixVersionFileTests.cs': 'TestCSharpWixOutput',
    'GitVersion.Output.Tests/Output/WixFileTests.cs': 'TestCSharpWixOutput',
    'GitVersion.Output.Tests/Output/AssemblyFileVersionTests.cs': 'TestCSharpOutputProjections',
    'GitVersion.Output.Tests/Output/InformationalVersionBuilderTests.cs': 'TestCSharpOutputProjections',
    'GitVersion.Core.Tests/CommitDateTests.cs': 'TestCSharpOutputProjections',
    'GitVersion.Core.Tests/Core/TaggedSemanticVersionRepositoryTests.cs': 'TestCSharpTaggedVersionFiltering',
    'GitVersion.Core.Tests/Core/BranchRepositoryTests.cs': 'TestCSharpCanonicalBranchIgnore',
}.items():
    path = source_root + source
    raw = pinned(path)
    text = raw.decode()
    suite = Path(source).stem
    methods = {}
    pattern = r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\([^{}]*?\)\n    \{(.*?)^    \}'
    for attrs, method, body in re.findall(pattern, text, re.M | re.S):
        if suite == 'BranchRepositoryTests' and not method.startswith('IsBranchIgnored_'):
            continue
        cases = []
        for i, args in enumerate(re.findall(r'^    \[TestCase\((.*)\)\]$', attrs, re.M) or ['']):
            args = re.sub(r'AssemblyFileVersioningScheme\.(\w+)',r'"\1"',args)
            args = args.replace('" + MainBranch + "', 'main').replace('MainBranch,', '"main",')
            case = dict(id=f'case-{i:03d}', arguments=json.loads(f'[{args}]'))
            if suite == 'WixFileTests':
                approval = Path(path).parent / 'Approved' / f'{suite}.{method}.approved.txt'
                case['approved'] = pinned(approval).decode()
            elif suite == 'TaggedSemanticVersionRepositoryTests':
                tags = re.findall(r'(?:ApplyTag|MakeATaggedCommit)\((".*?")\)', body)
                expected = re.search(r'actual.ShouldBe\(\[(.*?)\]\)', body)[1]
                patterns = re.search(r'Tags = \[(.*?)\]', body)
                case.update(tags=[json.loads(tag) for tag in tags], expected_tags=json.loads(f'[{expected}]'))
                case['patterns'] = json.loads(f'[{patterns[1]}]') if patterns else [p for p in case['arguments'] if p is not None]
            elif suite == 'BranchRepositoryTests':
                case['patterns'] = json.loads('[' + re.search(r'Branches = \[(.*?)\]', body)[1] + ']')
            cases.append(case)
        methods[method] = dict(cases=cases)
    suites[suite] = dict(source=path, sha256=hashlib.sha256(raw).hexdigest(), go_test=go_test, methods=methods)

assert sum(len(s['methods']) for s in suites.values()) == 12
assert sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()) == 34
case_file = 'testdata/csharp-wix-and-output.json'
data = json.dumps(dict(revision=revision, suites=suites), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
adaptations = {
    'WixFileTests': 'Use the Go variable provider and temporary files. Remove the UTF-8 BOM for the original approved-text assertion.',
    'UpdateWixVersionFileTests': 'Run the Go CLI on the original commit/tag topology. Check file existence and compare its variable count or every value with the independent calculation.',
    'AssemblyFileVersionTests': 'Exercise the Go formatter. The None scheme is represented internally by an empty string.',
    'CommitDateTests': 'Use the four parameterized NUnit cases and the original UTC timestamp. The Test marker adds no arguments.',
    'InformationalVersionBuilderTests': 'Project the original semantic version and build metadata through the Go output provider.',
    'BranchRepositoryTests': 'Match the original canonical local and remote names with the Go ignore matcher.',
    'TaggedSemanticVersionRepositoryTests': 'Generate the original Git histories with go-git. Check the entire ordered tag-name set returned by the calculation tag reader.',
}
for suite, detail in suites.items():
    for method, item in detail['methods'].items():
        cases = item['cases']
        key = f'{suite}.{method}'
        entry = dict(
            source=detail['source'], source_sha256=detail['sha256'], method=method,
            go_test=detail['go_test'], case_file=case_file, case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
            case_ids=[case['id'] for case in cases], adaptations=[adaptations[suite]],
            status='ported-awaiting-platform-verification',
        )
        if key in mappings:
            entry['status'] = mappings[key]['status']
            assert entry == mappings[key], key
        mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
