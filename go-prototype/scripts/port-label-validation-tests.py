#!/usr/bin/env python3
"""Extract complete pinned label-tokenizer and sanitizer test methods."""
import hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1]
revision=json.loads((root/'baseline.json').read_text())['revision']
def literal(value):
    # C# regular strings add NUL and variable-width hex escapes to JSON syntax.
    value=re.sub(r'\\x([0-9a-fA-F]{1,4})',lambda m:'\\u%04x'%int(m[1],16),value)
    value=value.replace('\\0','\\u0000')
    return json.loads(value)
def attributes(text):
    values=[]
    for line in text.splitlines():
        m=re.match(r'\s*\[TestCase\((.*)\)\]',line)
        if not m:continue
        raw=re.sub(r', TestName = "[^"]*"$','',m[1])
        args=re.findall(r'"(?:\\.|[^"\\])*"',raw)
        assert re.sub(r'"(?:\\.|[^"\\])*"','',raw).replace(',','').strip()=='',raw
        values.append([literal(x) for x in args])
    return values
files={'LabelTokenizerTests':4,'SanitizeFormatTests':7,'SanitizeEnvVarNameTests':6,'SanitizeMemberNameTests':7}
for cls,count in files.items():
    source='src/GitVersion.Core.Tests/Formatting/'+cls+'.cs'
    raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent)
    text=raw.decode('utf-8-sig');methods={}
    matches=list(re.finditer(r'public void (\w+)\([^\n]*',text))
    previous=0
    for i,m in enumerate(matches):
        name=m[1];prefix=text[previous:m.start()];end=matches[i+1].start() if i+1<len(matches) else len(text)
        # Body ends before the next Test/TestCase attribute, not at its method.
        tail=text[m.end():end];body=re.split(r'\n\s*\[Test(?:Case)?(?:\(|\])',tail)[0]
        attrs=attributes(prefix);previous=m.end()
        cases=[]
        if cls=='LabelTokenizerTests':
            assert attrs
            for a in attrs:
                c={'input':a[0],'operation':'tokens','error':'Throws' in name}
                if not c['error']:c['expected_tokens']=a[1:]
                cases.append(c)
        else:
            operation=cls.removesuffix('Tests')
            error='Throws' in name
            error_match=re.search(r'ShouldThrowWithMessage<\w+>\(("(?:\\.|[^"\\])*")\)',body)
            assert bool(error_match)==error,name
            expected_error=literal(error_match[1]) if error else None
            if attrs:
                assert all(len(a)==1 for a in attrs),name
                values=[a[0] for a in attrs]
                if 'ControlCharacters' in name:values=['valid'+x+'format' for x in values]
            else:
                repeated=re.search(r"new string\('(.)', (\d+)\)",body)
                constant=re.search(r'const string \w+ = ("(?:\\.|[^"\\])*");',body)
                assert bool(repeated)!=bool(constant),name
                values=[repeated[1]*int(repeated[2])] if repeated else [literal(constant[1])]
            for value in values:
                cases.append({'operation':operation,'input':value,'error':error,'expected':expected_error if error else value})
        for i,c in enumerate(cases):c['id']='case-%03d'%i
        assert cases,name
        methods[name]=cases
    assert len(methods)==count,(cls,len(methods))
    out={'revision':revision,'source':source,'source_sha256':hashlib.sha256(raw).hexdigest(),'methods':methods}
    target='testdata/csharp-'+re.sub(r'(?<!^)(?=[A-Z])','-',cls.removesuffix('Tests')).lower()+'.json'
    (root/target).write_text(json.dumps(out,indent=2)+'\n')
    print(cls,len(methods),'methods',sum(map(len,methods.values())),'cases')
