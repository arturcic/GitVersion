#!/usr/bin/env python3
"""Extract reviewed literal NUnit increment cases from the immutable C# pin.

This is not a C# interpreter. Unknown TestCase syntax is rejected, not skipped.
Expected strings are copied from C#, never evaluated through the Go candidate.
"""
import hashlib
import json
import re
import subprocess
from pathlib import Path
root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
source = 'src/GitVersion.Core.Tests/VersionCalculation/SemanticVersionTests.cs'
raw = subprocess.check_output(['git', 'show', revision + ':' + source], cwd=root.parent)
pattern = re.compile(r'\s*\[TestCase\("([^"]+)", VersionField\.(None|Patch|Minor|Major), (null|"[^"]*"), ExpectedResult = "([^"]+)"\)\]')
methods, attributes = {}, []
for line in raw.decode().splitlines():
    if line.strip().startswith('[TestCase('):
        attributes.append(line)
    match = re.search(r'public (?:void|string) (\w+)\(', line)
    if not match:
        continue
    method = match[1]
    if method.startswith('WhenIncrementing'):
        cases = []
        mode = 'Force' if method.endswith('InModeForce') else 'EnsureIntegrity' if method.endswith('InModeEnsureIntegrity') else 'Standard'
        for ordinal, attribute in enumerate(attributes):
            match = pattern.fullmatch(attribute)
            assert match, 'Unrecognized TestCase: ' + attribute
            value, increment, label, expected = match.groups()
            cases.append({'id': 'case-%03d' % ordinal, 'value': value, 'increment': increment,
                          'label': json.loads(label), 'mode': mode, 'expected': expected})
        assert len(cases) == 96, (method, len(cases))
        methods[method] = cases
    attributes = []
assert len(methods) == 12
out = {'revision': revision, 'source': source, 'source_sha256': hashlib.sha256(raw).hexdigest(), 'methods': methods}
(root / 'testdata/csharp-semver-increments.json').write_text(json.dumps(out, indent=2) + '\n')
print(len(methods), 'methods;', sum(map(len, methods.values())), 'literal parameter cases')
# These six assertions are reviewed directly from ValidateInvalidVersionParsing
# and VersionSorting; guards prevent silently importing a changed source contract.
for snippet in ['[TestCase("someText")]', '[TestCase("some-T-ext")]', '[TestCase("v.1.2.3", RegexPatterns.Configuration.DefaultTagPrefixRegexPattern)]',
                'SemanticVersion.Parse("1.0.0", null).ShouldBeGreaterThan(SemanticVersion.Parse("1.0.0-beta", null));',
                'SemanticVersion.Parse("1.0.0-beta.2", null).ShouldBeGreaterThan(SemanticVersion.Parse("1.0.0-beta.1", null));',
                'SemanticVersion.Parse("1.0.0-beta.1", null).ShouldBeLessThan(SemanticVersion.Parse("1.0.0-beta.2", null));']:
    assert snippet in raw.decode(), snippet
basic = {'revision': revision, 'source': source, 'source_sha256': out['source_sha256'], 'methods': {
    'ValidateInvalidVersionParsing': [
        {'id': 'text', 'value': 'someText', 'tag_prefix': None},
        {'id': 'hyphenated', 'value': 'some-T-ext', 'tag_prefix': None},
        {'id': 'prefixed-invalid', 'value': 'v.1.2.3', 'tag_prefix': '[vV]?'},
    ],
    'VersionSorting': [
        {'id': 'stable-after-beta', 'left': '1.0.0', 'right': '1.0.0-beta', 'sign': 1},
        {'id': 'higher-prerelease', 'left': '1.0.0-beta.2', 'right': '1.0.0-beta.1', 'sign': 1},
        {'id': 'lower-prerelease', 'left': '1.0.0-beta.1', 'right': '1.0.0-beta.2', 'sign': -1},
    ]}}
(root / 'testdata/csharp-semver-basic.json').write_text(json.dumps(basic, indent=2) + '\n')
