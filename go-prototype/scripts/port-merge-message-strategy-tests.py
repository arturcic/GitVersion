#!/usr/bin/env python3
"""Extract the pinned merge-message strategy cases and literal assertions."""
import hashlib
import json
import re
import subprocess
import textwrap
from pathlib import Path

root = Path(__file__).resolve().parents[1]
revision = json.loads((root / 'baseline.json').read_text())['revision']
suite = 'MergeMessageBaseVersionStrategyTests'
source = f'src/GitVersion.Core.Tests/VersionCalculation/Strategies/{suite}.cs'
raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
code = raw.decode('utf-8-sig')
test_base = subprocess.check_output(
    ['git', 'show', f'{revision}:src/GitVersion.Core.Tests/Helpers/TestBase.cs'], cwd=root, text=True)
main = json.loads(re.search(r'const string MainBranch = ("[^"]+");', test_base)[1])
literal = re.compile(r'""".*?"""|"(?:\\.|[^"\\])*"|\b(?:MainBranch|true|false|null)\b', re.S)


def arguments(expression):
    pieces = list(literal.finditer(expression))
    assert re.sub(literal, '', expression).translate(str.maketrans('', '', '+,')).strip() == '', expression
    result = []
    end = 0
    for piece in pieces:
        value = piece[0]
        if value.startswith('"""'):
            value = textwrap.dedent(value[3:-3]).strip('\n')
        elif value == 'MainBranch':
            value = main
        else:
            value = json.loads(value)
        if '+' in expression[end:piece.start()]:
            result[-1] += value
        else:
            result.append(value)
        end = piece.end()
    return result


decls = list(re.finditer(r'^    public void (\w+)\(', code, re.M))
methods = {}
previous = 0
for i, match in enumerate(decls):
    method = match[1]
    body = code[match.end():decls[i + 1].start() if i + 1 < len(decls) else len(code)]
    attrs = code[previous:match.start()]
    attrs = attrs[attrs.rfind('\n    }') + 6:] if i else attrs
    previous = match.end()
    rows = []
    if method == 'ShouldNotAllowIncrementOfVersion':
        assert '[Test]' in attrs
        message = json.loads(re.search(r'Message.Returns\(("(?:\\.|[^"\\])*")\)', body)[1])
        version = json.loads(re.search(r'GetIncrementedVersion\(\).ToString\(\).ShouldBe\(("[^"]+")\)', body)[1])
        rows.append(dict(message=message, merge=True, incremented=version, suffixes=['']))
    else:
        # The method body ends at its first closing brace at declaration indentation.
        body = body.split('\n    }', 1)[0]
        suffixes = [
            '' if suffix == '' else json.loads(suffix)
            for suffix in re.findall(r'AssertMergeMessage\(\w+(?: \+ ("(?:\\.|[^"\\])*"))?,', body)
        ]
        assert len(suffixes) in (1, 8)
        cases = re.findall(r'\[TestCase\((.*?)\)\]', attrs, re.S)
        for expression in cases:
            args = arguments(expression)
            message = args[0]
            case = dict(message=message, merge=True, suffixes=suffixes)
            if method == 'TakesVersionFromMergeOfReleaseBranch':
                _, case['merge'], case['version'] = args
            elif method == 'TakesVersionFromMergeOfConfiguredReleaseBranch':
                _, case['release_branch'], case['version'] = args
            elif method == 'ShouldNotTakeVersionFromUnrelatedMerge':
                assert len(args) == 1
            else:
                _, case['merge'] = args
                if method == 'ShouldNotTakeVersionFromMergeOfNonReleaseBranch':
                    case["disable_hotfix"] = True
            rows.append(case)
    assert rows
    for j, case in enumerate(rows):
        case['id'] = f'case-{j:03}'
    methods[method] = rows

assert [len(cases) for cases in methods.values()] == [1, 14, 14, 3, 7, 3]
assert sum(len(case['suffixes']) for cases in methods.values() for case in cases) == 259
sha = hashlib.sha256(raw).hexdigest()
target = 'testdata/csharp-merge-message-strategy.json'
payload = json.dumps(dict(revision=revision, source=source, source_sha256=sha, methods=methods), indent=2) + '\n'
(root / target).write_text(payload)
path = root / 'testdata/csharp-unit-port-mappings.json'
mapping = json.loads(path.read_text())
for method, cases in methods.items():
    key = f'{suite}.{method}'
    item = dict(source=source, source_sha256=sha, method=method, go_test='TestCSharpMergeMessageStrategy',
                case_file=target, case_file_sha256=hashlib.sha256(payload.encode()).hexdigest(),
                case_ids=[case['id'] for case in cases],
                adaptations=['Synthetic commit metadata replaces C# interface substitutes. The calculator candidate path receives the original message, parent count, workflow, branch settings, and whitespace variations. The tests assert candidate cardinality and exact semantic or incremented version.'],
                status='ported-awaiting-platform-verification')
    if key in mapping:
        old = mapping[key]
        assert all(old[k] == v for k, v in item.items() if k != 'status'), key
        item['status'] = old['status']
    mapping[key] = item
path.write_text(json.dumps(mapping, indent=2) + '\n')
print('6 original methods, 42 parameter cases, 259 original candidate assertions')
