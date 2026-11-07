#!/usr/bin/env python3
"""Port the pinned built-in patterns and every literal RegexPatternsTests case."""
import hashlib,json,re,subprocess,textwrap
from pathlib import Path
root=Path(__file__).resolve().parents[1]
revision=json.loads((root/'baseline.json').read_text())['revision']
source='src/GitVersion.Core.Tests/Core/RegexPatternTests.cs'
pattern_source='src/GitVersion.Core/Core/RegexPatterns.cs'
def read(path):return subprocess.check_output(['git','show',revision+':'+path],cwd=root.parent).decode('utf-8-sig')
src=read(source);patterns=read(pattern_source)
def args(text):
    out=[];start=0;quote=False;escape=False;depth=0
    for i,c in enumerate(text):
        if quote:
            if escape:escape=False
            elif c=='\\':escape=True
            elif c=='"':quote=False
        elif c=='"':quote=True
        elif c in '{[':depth+=1
        elif c in '}]':depth-=1
        elif c==',' and depth==0:out.append(text[start:i].strip());start=i+1
    out.append(text[start:].strip());return out
def literal(s):
    if s=='new string[0]':return []
    if s.startswith('new[] {'):return [literal(x) for x in args(s[7:-1])]
    return json.loads(s)
def get_pattern(key):
    name=key.rsplit('.',1)[-1]+'Pattern';body=patterns
    if key.startswith('AssemblyVersion.'):
        lang=key.split('.')[1]
        if lang=='FSharp' and name=='TriviaRegexPattern':lang='CSharp'
        body=patterns.split('internal static partial class '+lang,1)[1].split('internal static partial class',1)[0]
    match=re.search(r'(?:internal|private) const string '+name+r'\s*=\s*(""".*?"""|@"(?:[^"]|"")*"|"(?:\\.|[^"\\])*")\s*;',body,re.S)
    assert match,key
    raw=match[1]
    if raw.startswith('"""'):value=textwrap.dedent(raw[3:-3]).strip('\n')
    elif raw.startswith('@'):value=raw[2:-1].replace('""','"')
    else:value=json.loads(raw)
    options=re.search(r'\[GeneratedRegex\('+name+r', ([^\]]+)\)\]',body)
    assert options,key
    flags=[]
    for option in options[1].split('|'):
        option=option.strip()
        if option=='Options':flags.append('IgnoreCase')  # shared IgnoreCase | Compiled
        else:
            assert option.startswith('RegexOptions.'),option
            flag=option.removeprefix('RegexOptions.')
            assert flag in ['Multiline','Singleline','IgnorePatternWhitespace'],flag
            flags.append(flag)
    return {'pattern':value,'options':flags}
methods={};builtins={};pending=[]
for line in src.splitlines():
    m=re.match(r'\s*\[TestCase\((.*)\)\]',line)
    if m:pending.append([literal(x) for x in args(m[1])])
    m=re.search(r'public void (\w+)\(',line)
    if not m:continue
    name=m[1];start=src.index(line);body=src[start:];end=body.find('\n    [TestCase');body=body if end<0 else body[:end]
    ref=re.search(r'(?:RegexPatterns\.([\w.]+)|var regex = (CSharp|FSharp)\.(\w+))',body)
    assert ref,name
    key=ref[1] if ref[1] else 'AssemblyVersion.'+ref[2]+'.'+ref[3]
    # Match/Matches are invocation members, not part of the pattern name.
    key=key.removesuffix('.Match').removesuffix('.Matches')
    builtins[key]=get_pattern(key)
    operation='all' if 'CapturesInvalidCharacters' in name else 'group' if 'TriviaRegex' in name else 'first'
    cases=[]
    for i,a in enumerate(pending):
        c={'id':'case-%03d'%i,'pattern':key,'operation':operation,'input':a[0],'match':a[1]}
        if operation=='all':c['captures']=a[2]
        elif operation=='group':c.update(group={'block':'1','line':'2','str':'3','':''}[a[2]],capture=a[3])
        elif len(a)>2:c['capture']=a[2]
        cases.append(c)
    assert cases,name
    methods[name]=cases;pending=[]
assert len(methods)==35,len(methods)
# Raw source SHA matches inventory, including any original BOM.
raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent)
corpus={'revision':revision,'source':source,'source_sha256':hashlib.sha256(raw).hexdigest(),'pattern_source':pattern_source,'pattern_source_sha256':hashlib.sha256(subprocess.check_output(['git','show',revision+':'+pattern_source],cwd=root.parent)).hexdigest(),'methods':methods}
(root/'testdata/csharp-regex-patterns.json').write_text(json.dumps(corpus,indent=2)+'\n')
p=root/'internal/patterns';p.mkdir(exist_ok=True)
for name in ('AssemblyVersion.VisualBasic.TriviaRegex', 'AssemblyVersion.VisualBasic.AttributeRegex'):
    builtins[name] = get_pattern(name)
(p/'builtins.json').write_text(json.dumps(builtins,indent=2)+'\n')
print(len(methods),'methods;',sum(map(len,methods.values())),'cases;',len(builtins),'patterns')
