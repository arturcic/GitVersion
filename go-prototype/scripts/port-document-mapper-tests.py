#!/usr/bin/env python3
"""Record original mapper declarations, parameter rows and independent schema expectations."""
from pathlib import Path
import hashlib
import json
import re
import subprocess

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
source_hashes = {}


def read_source(path):
    raw = subprocess.check_output(['git', 'show', revision + ':' + path], cwd=root.parent)
    source_hashes[path] = hashlib.sha256(raw).hexdigest()
    return raw.decode('utf-8-sig')


def serialized_properties(class_name):
    source = read_source('src/GitVersion.Configuration/' + class_name + '.cs')
    attributes = list(re.finditer(r'\[JsonPropertyName\("([^"]+)"\)\]', source))
    properties = {}
    for i, attribute in enumerate(attributes):
        end = attributes[i + 1].start() if i + 1 < len(attributes) else len(source)
        declaration = re.search(r'\bpublic\s+[^\n{;]+?\s+(\w+)\s*\{', source[attribute.end():end])
        assert declaration, (class_name, attribute[1])
        properties[declaration[1]] = attribute[1]
    return properties


def interface_properties(name):
    source = read_source('src/GitVersion.Core/Configuration/' + name + '.cs')
    return set(re.findall(r'\b(\w+)\s*\{\s*get;\s*\}', source))


branch = serialized_properties('BranchConfiguration')
root_fields = branch | serialized_properties('GitVersionConfiguration')
calc_branch = interface_properties('ICalculationBranchConfiguration')
out_branch = interface_properties('IOutputBranchConfiguration')
calc = calc_branch | interface_properties('ICalculationConfiguration')
out = out_branch | interface_properties('IOutputConfiguration')
calc.remove('VersionStrategy')
calc.add('VersionStrategies')
assert len(root_fields) == 37 and len(branch) == 15
assert calc & out == {'Branches'} and set(root_fields) == calc | out | {'Workflow'}
assert not calc_branch & out_branch and set(branch) == calc_branch | out_branch

source = 'src/GitVersion.Configuration.Tests/Configuration/ConfigurationDocumentMapperTests.cs'
text = read_source(source)
methods = {}
for match in re.finditer(r'((?:^    \[Test(?:Case\([^\n]*\))?\]\n)+)    public void (\w+)\(', text, re.M):
    arguments = [json.loads('[' + row + ']') for row in re.findall(r'\[TestCase\(([^\n]*)\)\]', match[1])]
    methods[match[2]] = [
        {'id': f'case-{index:03d}', 'arguments': row}
        for index, row in enumerate(arguments or [[]])
    ]
assert len(methods) == 10 and sum(map(len, methods.values())) == 19
corpus = {
    'revision': revision, 'source': source, 'source_sha256': source_hashes[source],
    'methods': methods, 'schema_source_sha256': source_hashes,
    'schema': {
        'root': sorted(root_fields.values()),
        'branch': sorted(branch.values()),
        'calculation': sorted(root_fields[name] for name in calc),
        'output': sorted(root_fields[name] for name in out),
        'calculation_branch': sorted(branch[name] for name in calc_branch),
        'output_branch': sorted(branch[name] for name in out_branch),
    },
}
target = 'testdata/csharp-document-mapper.json'
data = json.dumps(corpus, indent=2) + '\n'
(root / target).write_text(data)
path = root / 'testdata/csharp-unit-port-mappings.json'
manifest = json.loads(path.read_text())
for method, cases in methods.items():
    key = 'ConfigurationDocumentMapperTests.' + method
    item = {
        'source': source, 'source_sha256': corpus['source_sha256'], 'method': method,
        'go_test': 'TestCSharpConfigurationDocumentMapper', 'case_file': target,
        'case_file_sha256': hashlib.sha256(data.encode()).hexdigest(),
        'case_ids': [case['id'] for case in cases],
        'adaptations': [
            'Original dictionary values and assertions are transcribed in Go. Typed and object-keyed input maps both encode to the production YAML node representation.',
            'Pinned serialized properties and inherited interface fields independently supply the original reflection expectations for the Go ownership registry.',
            'Go errors replace configuration exceptions, preserving the original asserted diagnostic substrings.',
        ],
        'status': 'ported-awaiting-platform-verification',
    }
    if key in manifest:
        previous = manifest[key]
        assert all(previous[k] == value for k, value in item.items() if k != 'status'), key
        item['status'] = previous['status']
    manifest[key] = item
path.write_text(json.dumps(manifest, indent=2) + '\n')
print('10 methods / 19 original cases')
