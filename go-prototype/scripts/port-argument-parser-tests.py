#!/usr/bin/env python3
"""Extract argument parser inputs and assertions from the pinned C# sources."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parent.parent
revision = json.loads((root / 'baseline.json').read_text())['revision']
target = 'testdata/csharp-argument-parsers.json'
manifest_path = root / 'testdata/csharp-unit-port-mappings.json'
manifest = json.loads(manifest_path.read_text())
suites = {}
# Configuration command binding and the typed whole-object override equality
# cases remain separate from the root parser field contracts.
pending = {
    'ConfigGetMapsOnlyQueryOptions',
    'OverrideConfigWithSingleOptions', 'OverrideConfigWithMultipleOptions',
}
special = {
    'ConfigValidateDefersInvalidTargetPathToItsExecutor',
    'ConfigGetRejectsUnrelatedOptionsBeforeAndAfterCommand',
    'ConfigGetRequiresExactlyOneProperty', 'ConfigGetRejectsV6BeforeConfigFileValidation',
    'OverrideConfigSupportsNestedV7RootAndBranchPaths', 'OverrideConfigAcceptsWorkflowOnlyInEitherVersion',
    'OverrideConfigRejectsNestedWorkflowWithRootReplacement', 'OverrideConfigPreservesV7BranchNameCasing',
    'OverrideConfigRejectsDottedV7BranchNameWithClearDiagnostic', 'OverrideConfigRejectsV6PathInV7WithReplacement',
    'OverrideConfigRejectsPropertyInWrongV7SectionWithReplacement', 'OverrideConfigSupportsV6BranchPath',
    'OverrideConfigPreservesV6BranchNameCasing', 'OverrideConfigRejectsV7PathInV6WithReplacement',
    'OverrideConfigBatchValidationDoesNotApplyAnyValues', 'OverrideconfigWithInvalidOption',
    'UpdateAssemblyInfoWithFilename', 'UpdateAssemblyInfoWithMultipleFilenames',
    'UpdateProjectFilesWithMultipleFilenames', 'UpdateAssemblyInfoWithMultipleFilenamesMatchingGlobbing',
    'UpdateAssemblyInfoWithRelativeFilename', 'ThrowIfConfigurationFileDoesNotExist',
    'EnsureConfigurationFileIsSet', 'ShortAliasCIsConfigNotCommit', 'CheckVerbosityParsing',
}
string = r'"(?:[^"\\]|\\.)*"'
attributes = re.compile(r'((?:^    \[[^\n]+\][^\n]*\n)+)    public (?:void|string) (\w+)\([^\n]*\)(?: =>)?', re.M)
for suite in ['ArgumentParserTests', 'LegacyArgumentParserTests']:
    source = f'src/GitVersion.App.Tests/{suite}.cs'
    raw = subprocess.check_output(['git', 'show', f'{revision}:{source}'], cwd=root)
    text = raw.decode()
    matches = list(attributes.finditer(text))
    methods = {}
    for index, match in enumerate(matches):
        attrs, name = match.groups()
        if '[Test' not in attrs or name in pending or (
            name.startswith('ConfigMigrate') or name == 'ConfigRequiresASubcommand'
        ):
            continue
        body = text[match.end():matches[index+1].start() if index+1<len(matches) else len(text)]
        body = body.split('\n    private ',1)[0]
        params = [json.loads('[' + re.sub(r'Verbosity\.(\w+)', r'"\1"', row) + ']')
                  for row in re.findall(r'^    \[TestCase\((.*)\)\]', attrs, re.M)]
        if name == 'OverrideconfigWithInvalidOption':
            data = text.split('private static IEnumerable<TestCaseDataWithReturn<string, string>> OverrideconfigWithInvalidOptionTestData()',1)[1].split('\n    }',1)[0]
            params = [[json.loads(option), json.loads(expected)] for option, expected in
                      re.findall(r'TestCaseData.Create\((' + string + r')\).Returns\((' + string + r')\);', data)]
            assert len(params) == 2
        item = {'cases': [{'id': f'case-{i:03d}', 'arguments': args} for i, args in enumerate(params or [[]])]}
        if name not in special:
            cmd = re.search(r'ParseArguments\((' + string + r'|string.Empty|\w+)\)', body)
            if not cmd:
                raise AssertionError((suite,name,body[:400]))
            command = cmd[1]
            item['command'] = json.loads(command) if command.startswith('"') else '' if command == 'string.Empty' else None
            env = re.findall(r'SetEnvironmentVariable\("([^"]+)",\s*(' + string + r')\);', body)
            item['environment'] = {key:json.loads(value) for key,value in env}
            checks = []
            for field, operation, expression in re.findall(
                r'arguments\.([\w.]+)\.(ShouldBe|ShouldContain|ShouldNotContain|ShouldBeEmpty)\(([^;]*?)\);', body):
                if expression.startswith('"'):
                    expected = json.loads(expression)
                elif expression in ('true','false','null'):
                    expected = json.loads(expression)
                elif expression == 'SysEnv.CurrentDirectory':
                    expected = '$cwd'
                elif expression.startswith('OutputType.'):
                    expected = expression.split('.')[1].lower()
                elif expression == 'outputFile':
                    expected = '$arg1'
                elif operation == 'ShouldBeEmpty':
                    expected = None
                else:
                    raise AssertionError((suite,name,field,expression))
                checks.append([field,operation,expected])
            for field in re.findall(r'Assert.That\(arguments\.(\w+), Is.Null\);', body):
                checks.append([field,'ShouldBe',None])
            error = re.search(r'exception.Message\.(ShouldBe|ShouldContain|ShouldStartWith)\((' + string + r')\);',body)
            if error:
                item['error'] = [error[1],json.loads(error[2])]
            elif not checks:
                raise AssertionError((suite,name,body[:400]))
            item['checks'] = checks
        methods[name] = item
    suites[suite] = {'source':source,'sha256':hashlib.sha256(raw).hexdigest(),'methods':methods}
source = 'src/GitVersion.App.Tests/QuotedStringHelpersTests.cs'
raw = subprocess.check_output(['git','show',f'{revision}:{source}'],cwd=root)
text = raw.decode()
methods = {}
for name, data_source in [
    ('SplitUnquotedTests','SplitUnquotedTestData'),
    ('SplitUnquotedRemovesEmptyEntries','RemoveEmptyEntriesTestData'),
    ('UnquoteTextTests','UnquoteTextTestData'),
]:
    body = text.split('>> '+data_source+'()',1)[1].split('\n    }',1)[0]
    cases = []
    for source_case in re.findall(r'yield return TestCaseData.Create(?:<[^>]*>)?\((.*?)\).Returns\((.*?)\);',body,re.S):
        source_args, expected = source_case
        if name == 'UnquoteTextTests':
            values = [json.loads(source_args),json.loads(expected)]
        else:
            input_text, separator = re.fullmatch(r'(' + string + r"|null), '(.)'",source_args).groups()
            expected_values = [json.loads(s) for s in re.findall(string,expected)]
            values = [json.loads(input_text),separator,expected_values]
        cases.append({'id':f'case-{len(cases):03d}','arguments':values})
    methods[name] = {'cases':cases}
assert [len(m['cases']) for m in methods.values()] == [11,1,2]
suites['QuotedStringHelpersTests'] = {'source':source,'sha256':hashlib.sha256(raw).hexdigest(),'methods':methods}
data = json.dumps({'revision':revision,'suites':suites},indent=2)+'\n'
(root / target).write_text(data)
for suite, details in suites.items():
    for name, item in details['methods'].items():
        key = suite+'.'+name
        entry = {
            'source': details['source'], 'source_sha256': details['sha256'], 'method': name,
            'go_test': 'TestCSharpQuotedStringHelpers' if suite == 'QuotedStringHelpersTests' else 'TestCSharpArgumentParsers', 'case_file': target,
            'case_file_sha256': hashlib.sha256(data.encode()).hexdigest(),
            'case_ids': [c['id'] for c in item['cases']],
            'adaptations': ['C# parser fields and warning exceptions are checked against the production Go parser, captured diagnostics, and temporary filesystem fixtures.'],
            'status': 'ported-awaiting-platform-verification',
        }
        if key in manifest:
            old = manifest[key]
            assert all(old[k] == value for k,value in entry.items() if k != 'status'), key
            entry['status'] = old['status']
        manifest[key] = entry
manifest_path.write_text(json.dumps(manifest,indent=2)+'\n')
print([(name,len(s['methods']),sum(len(m['cases']) for m in s['methods'].values())) for name,s in suites.items()])
