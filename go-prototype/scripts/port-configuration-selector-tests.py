#!/usr/bin/env python3
from pathlib import Path
import json,hashlib,re,subprocess
root=Path(__file__).resolve().parents[1];rev=json.loads((root/'baseline.json').read_text())['revision'];source='src/GitVersion.Core.Tests/Core/ConfigurationVersionSelectorTests.cs';raw=subprocess.check_output(['git','show',rev+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig');methods={};last=0
for m in re.finditer(r'public void (\w+)\(',s):
 name=m[1];attrs=re.findall(r'\[TestCase\(([^\n]*)\)\]',s[last:m.start()]);last=m.end();rows=[]
 for i,text in enumerate(attrs):
  values=json.loads('['+text+']');rows.append(dict(id='case-%03d'%i,value=values[0],expected=values[1] if len(values)>1 else None,error=name=='FailsFastOnUnknownValues'))
 if not rows:continue
 methods[name]=rows
assert len(methods)==3 and sum(map(len,methods.values()))==17
out=dict(revision=rev,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),methods=methods);target='testdata/csharp-configuration-selector.json';data=json.dumps(out,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for method,cases in methods.items():
 key='ConfigurationVersionSelectorTests.'+method;old=manifest.get(key)
 item=dict(source=source,source_sha256=out['source_sha256'],method=method,go_test='TestCSharpConfigurationVersionSelector',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['Explicit nullable string input replaces a process environment scope, retaining every original unset/empty/case/whitespace value. The same selector is called by configuration loading.','Go version names replace C# enum values; Go errors replace WarningException with every original diagnostic containment assertion retained.'],status='ported-awaiting-platform-verification')
 if old:assert all(old[k]==v for k,v in item.items() if k!='status');item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n');print('3 methods / 17 original cases')
