#!/usr/bin/env python3
import hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1];revision=json.loads((root/'baseline.json').read_text())['revision']
source='src/GitVersion.Core.Tests/Core/ReferenceNameTests.cs'
raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root);text=raw.decode();methods={}
for m in re.finditer(r'((?:^    \[[^\n]+\]\n)+)    public void (\w+)\(',text,re.M):
 attrs,name=m.groups();cases=[json.loads('['+value+']') for value in re.findall(r'\[TestCase\(([^\n]+)\)\]',attrs)]
 methods[name]=[dict(id='case-%03d'%i,arguments=args) for i,args in enumerate(cases or [[]])]
assert len(methods)==6 and sum(map(len,methods.values()))==18
target='testdata/csharp-reference-names.json';doc=dict(revision=revision,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),methods=methods);data=json.dumps(doc,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for name,cases in methods.items():
 key='ReferenceNameTests.'+name
 item=dict(source=source,source_sha256=doc['source_sha256'],method=name,go_test='TestCSharpReferenceName',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['Canonical/friendly/origin-stripped/remote-stripped names and all literal tag prefixes retain their exact C# assertions. The same production reference type is used by configuration matching, tag observation and PR target equivalence.','Typed Go ReferenceArgumentError carries ParamName and a distinct null/empty Kind in place of the two C# exception classes.'],status='ported-awaiting-platform-verification')
 if key in manifest:
  old=manifest[key];assert all(old[k]==v for k,v in item.items() if k!='status');item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n');print('6 methods; 18 original cases')
