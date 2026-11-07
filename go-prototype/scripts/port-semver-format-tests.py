#!/usr/bin/env python3
"""Extract every remaining SemanticVersionTests parsing/formatting assertion."""
import hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1]
revision=json.loads((root/'baseline.json').read_text())['revision']
source='src/GitVersion.Core.Tests/VersionCalculation/SemanticVersionTests.cs'
raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent)
methods={};pending=[]
formats={'ToStringTests':'','ToStringWithSFormatTests':'s','ToStringWithFormatJTests':'j','ToStringWithFormatFTests':'f','ToStringWithFormatITests':'i'}
for line in raw.decode().splitlines():
    match=re.match(r'\s*\[TestCase\((.*)\)\]',line)
    if match:pending.append(match[1])
    match=re.search(r'public (?:void|string) (\w+)\(',line)
    if not match:continue
    name=match[1]
    if name=='ValidateVersionParsing':
        cases=[]
        for i,a in enumerate(pending):
            a=a.replace('SemanticVersionFormat.Strict','"Strict"').replace('SemanticVersionFormat.Loose','"Loose"').replace('RegexPatterns.Configuration.DefaultTagPrefixRegexPattern','"[vV]?"')
            fields=json.loads('['+a+']');assert len(fields)==13
            value,major,minor,patch,label,number,count,branch,sha,other,formatted,prefix,mode=fields
            cases.append(dict(id='case-%03d'%i,operation='parse',value=value,major=major,minor=minor,patch=patch,label=label,number=number,count=count,branch=branch,sha=sha,other=other,expected=formatted if formatted is not None else value,prefix=prefix,mode=mode))
        assert len(cases)==29,len(cases)
        methods[name]=cases
    elif name in formats:
        cases=[]
        for i,a in enumerate(pending):
            values,expected=a.split(', ExpectedResult = ')
            fields=json.loads('['+values+']');assert len(fields)==9
            major,minor,patch,label,number,count,branch,sha,other=fields
            cases.append(dict(id='case-%03d'%i,operation='format',major=major,minor=minor,patch=patch,label=label,number=number,count=count,branch=branch,sha=sha,other=other,expected=json.loads(expected),format=formats[name]))
        assert len(cases)==(3 if name=='ToStringWithFormatJTests' else 4)
        methods[name]=cases
    elif name=='ToStringWithInvalidFormatTest':
        assert 'var semVer = BuildSemVer(1, 2, 3, "rc", 1, 1);' in raw.decode()
        assert 'Should.Throw<FormatException>(() => semVer.ToString("invalid"));' in raw.decode()
        methods[name]=[dict(id='invalid-format',operation='format',major=1,minor=2,patch=3,label='rc',number=1,count=1,format='invalid',error=True)]
    pending=[]
assert len(methods)==7
out={'revision':revision,'source':source,'source_sha256':hashlib.sha256(raw).hexdigest(),'methods':methods}
(root/'testdata/csharp-semver-formats.json').write_text(json.dumps(out,indent=2)+'\n')
print(len(methods),'methods;',sum(map(len,methods.values())),'cases')
