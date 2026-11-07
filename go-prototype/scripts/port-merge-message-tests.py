#!/usr/bin/env python3
"""Port pinned MergeMessage assertions; literal expectations never come from Go."""
import hashlib,json,re,subprocess,textwrap
from pathlib import Path
root=Path(__file__).resolve().parents[1];revision=json.loads((root/'baseline.json').read_text())['revision'];source='src/GitVersion.Core.Tests/MergeMessageTests.cs'
raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig');methods={}
token=re.compile(r'\$?""".*?"""|\$?"(?:\\.|[^"\\])*"|new SemanticVersion\([^)]*\)|\bnull\b|\bMainBranch\b|\d+',re.S)
def literal(v):
 if v=='null':return None
 if v=='MainBranch':return 'main'
 if v.startswith('new SemanticVersion'):
  nums=list(map(int,re.findall(r'\d+',v)));return '.'.join(map(str,nums+[0]*(3-len(nums))))
 if v.startswith('$'):v=v[1:].replace('{MainBranch}','main')
 if v.startswith('"""'):return textwrap.dedent(v[3:-3]).strip('\n')
 if v.startswith('"'):return json.loads(v)
 return int(v)
def case(message,fmt=None,branch=None,target=None,version=None,pr=None,custom=None,error=False):
 return dict(message=message,format=fmt,branch=branch,target=target,version=version,pr=pr,custom=custom or [],error=error)
for m in re.finditer(r'private static readonly object\??\[\] (\w+) =\s*\[(.*?)\n    \];',s,re.S):
 array,body=m.groups();method=re.search(r'\[TestCaseSource\(nameof\('+array+r'\)\)\]\s*public void (\w+)',s)[1]
 fmt=re.search(r'FormatName.ShouldBe\("([^"]+)"\)',s[s.index('public void '+method):]);fmt=fmt[1] if method!='ParsesInvalidMergeMessage' else None
 rows=[]
 for q in re.finditer(r'new object\??\[\] \{ (.*?) \}',body,re.S):
  txt=q[1];matches=list(token.finditer(txt));assert re.sub(token,'',txt).replace(',','').strip()=='',txt
  args=[literal(t[0]) for t in matches]
  if fmt is None:assert len(args)==1;rows.append(case(args[0]));continue
  assert len(args) in [4,5];rows.append(case(args[0],fmt,args[1],args[2],args[3],args[4] if len(args)==5 else None))
 assert rows,array;methods[method]=rows
methods['NullMessageStringThrows']=[case(None,error=True)]
methods['EmptyMessageString']=[case(''),case('\t\t  ')]
methods['EmptyTagPrefix']=[dict(case('Updated some code.'),tag_prefix=p) for p in ['', '\t\t  ',None]]
methods['MatchesSingleCustomMessage']=[case('My custom message','MyCustom','',custom=[dict(Name='MyCustom',Pattern='My custom message')])]
methods['MatchesMultipleCustomMessages']=[case('My custom message','MyCustom','',custom=[dict(Name=n,Pattern=p) for n,p in [('Default2','some example'),('Default3','another example'),('MyCustom','My custom message')]])]
custom=re.search(r'public void MatchesCaptureGroupsFromCustomMessages\(\)(.*?)\n    }',s,re.S)[1];pattern=re.search(r'const string format = @"([^"]+)";',custom)[1]
methods['MatchesCaptureGroupsFromCustomMessages']=[case('Merged PR #1234 into main from feature/2.0.0/example','MyCustom','feature/2.0.0/example','main','2.0.0',1234,[dict(Name='MyCustom',Pattern=pattern)])]
custom=re.search(r'public void ReturnsAfterFirstMatchingPattern\(\)(.*?)\n    }',s,re.S)[1];pattern=re.search(r'const string format = @"([^"]+)";',custom)[1]
methods['ReturnsAfterFirstMatchingPattern']=[case("Merge branch 'this'",'MyCustom','this',custom=[dict(Name=n,Pattern=pattern) for n in ['MyCustom','Default2','Default3']])]
assert set(methods)==set(re.findall(r'public void (\w+)\(',s));assert len(methods)==15
for name,rows in methods.items():
 for i,c in enumerate(rows):c['id']='case-%03d'%i
out=dict(revision=revision,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),methods=methods)
target='testdata/csharp-merge-messages.json';data=json.dumps(out,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';mapping=json.loads(p.read_text())
for name,rows in methods.items():
 key='MergeMessageTests.'+name;old=mapping.get(key)
 item=dict(source=source,source_sha256=out['source_sha256'],method=name,go_test='TestCSharpMergeMessages',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in rows],adaptations=['Go pointers retain C# null versus empty capture values; Go errors replace null argument exceptions.','All original format, branch, target, pull-request and semantic version assertions retained. EmptyTagPrefix has an unused tag prefix on an unmatched message; original arguments retained.','Custom builder dictionaries map to an ordered format list, preserving first-match precedence.'],status='ported-awaiting-platform-verification')
 if old:
  assert all(old[k]==v for k,v in item.items() if k!='status');item['status']=old['status']
 mapping[key]=item
p.write_text(json.dumps(mapping,indent=2)+'\n');print(len(methods),'methods',sum(map(len,methods.values())),'cases')
