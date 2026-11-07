#!/usr/bin/env python3
"""Extract assembly updater parameters, assertions and approvals from pinned C#."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
source = 'src/GitVersion.Output.Tests/Output/AssemblyInfoFileUpdaterTests.cs'

def pinned(path):
    return subprocess.check_output(['git', 'show', f'{revision}:{path}'], cwd=root)

raw = pinned(source)
text = raw.decode()
sha = hashlib.sha256(raw).hexdigest()
methods = {}
pattern = r'((?:^    \[Test[^\n]*\]\n)+)    public void (\w+)\([^\n]*\)\n    \{(.*?)^    \}'
for attributes, method, body in re.findall(pattern, text, re.M | re.S):
    if method == 'ShouldNotCreateAssemblyInfoFileForUnknownSourceCodeAndEnsureAssemblyInfo':
        # The method name and production template lookup indicate no file. Its
        # mocked assertion asks for a write. Keep that declaration pending.
        continue
    cases = []
    for i, args in enumerate(re.findall(r'^    \[TestCase\((.*)\)\]$', attributes, re.M) or ['']):
        values = json.loads(f'[{args}]')
        case = dict(id=f'case-{i:03d}', arguments=values)
        if 'ShouldMatchApproved' in body:
            approved = f'{Path(source).parent}/Approved/{values[0]}/AssemblyInfoFileUpdaterTests.{method}.approved.txt'
            expected = pinned(approved)
            case.update(approved=expected.decode(), approved_sha256=hashlib.sha256(expected).hexdigest())
        else:
            case['contains'] = re.findall(r'(?<![!\w])s\.Contains\("""(.*?)"""\)', body)
            case['excludes'] = re.findall(r'!s\.Contains\("""(.*?)"""\)', body)
        if 'VerifyAssemblyInfoFile(' in body:
            scheme = re.search(r'AssemblyVersioningScheme\.(\w+)', body)
            case['scheme'] = scheme[1] if scheme else 'MajorMinorPatch'
        cases.append(case)
    methods[method] = cases
assert len(methods) == 19 and sum(map(len, methods.values())) == 55
case_file = 'testdata/csharp-assembly-info.json'
data = json.dumps(dict(revision=revision, source=source, source_sha256=sha, methods=methods), indent=2) + '\n'
(root / case_file).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
mappings = json.loads(path.read_text())
for method, cases in methods.items():
    key = f'AssemblyInfoFileUpdaterTests.{method}'
    entry = dict(
        source=source, source_sha256=sha, method=method,
        go_test='TestCSharpAssemblyInfoUpdater', case_file=case_file,
        case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),
        case_ids=[case['id'] for case in cases],
        adaptations=['Use a temporary filesystem and the Go variable provider. Verify observable file contents '
                     'and discovery boundaries in place of mocked filesystem calls. Approved text uses normalized line endings.'],
        status='ported-awaiting-platform-verification',
    )
    if key in mappings:
        entry['status'] = mappings[key]['status']
        assert entry == mappings[key], key
    mappings[key] = entry
path.write_text(json.dumps(mappings, indent=2) + '\n')
