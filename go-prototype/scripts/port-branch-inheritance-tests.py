#!/usr/bin/env python3
"""Extract the pinned branch-configuration finder's complete NUnit case matrix."""
import hashlib,itertools,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1]
revision=json.loads((root/'baseline.json').read_text())['revision']
source='src/GitVersion.Core.Tests/VersionCalculation/EffectiveBranchConfigurationFinderTests.cs'
def read(path):return subprocess.check_output(['git','show',revision+':'+path],cwd=root)
raw=read(source);text=raw.decode();enums={}
for name in ['IncrementStrategy','DeploymentMode']:
 path='src/GitVersion.Core/VersionCalculation/'+name+'.cs';data=read(path)
 values=re.findall(r'^    (\w+),?$',data.decode(),re.M)
 assert len(values)==(5 if name=='IncrementStrategy' else 3)
 enums[name]=dict(source=path,sha256=hashlib.sha256(data).hexdigest(),values=values)
methods={}
for m in re.finditer(r'((?:^    \[[^\n]+\]\n)+)    public void (\w+)\((.*?)\)\s*\{',text,re.M|re.S):
 attrs,name,parameters=m.groups();cases=[]
 if '[Theory]' in attrs:
  names=re.findall(r'(IncrementStrategy|DeploymentMode)\s+\w+',parameters)
  assert names and len(names)==len(parameters.split(',')),name
  cases=list(itertools.product(*(enums[n]['values'] for n in names)))
 else:
  for value in re.findall(r'\[TestCase\(([^\n]+)\)\]',attrs):
   tokens=re.findall(r'"(?:[^"\\]|\\.)*"|IncrementStrategy\.\w+',value)
   assert len(tokens)==len(parameters.split(',')),(name,value)
   cases.append([v.split('.')[1] if v.startswith('IncrementStrategy.') else json.loads(v) for v in tokens])
 methods[name]=[dict(id='case-%03d'%i,arguments=list(args)) for i,args in enumerate(cases or [[]])]
assert len(methods)==17 and sum(map(len,methods.values()))==63,(len(methods),sum(map(len,methods.values())))
target='testdata/csharp-branch-inheritance.json'
doc=dict(revision=revision,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),theory_enums=enums,methods=methods)
data=json.dumps(doc,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for method,cases in methods.items():
 key='EffectiveBranchConfigurationFinderTests.'+method
 item=dict(source=source,source_sha256=doc['source_sha256'],method=method,go_test='TestCSharpEffectiveBranchConfigurationFinder',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['Original IRepositoryStore source-branch mocks become a typed callback in the same recursive finder used by production calculation. Branch identities and ordered result cardinalities remain explicit.','Every NUnit Theory expands over the pinned C# enum declaration order. Typed Go options retain builder null modes/labels, inherited increments, and original fallback settings. Ordered builder definitions preserve WithoutBranches for overlapping regex rules.','Go errors replace exceptions. No version-output oracle is supplied to the independent calculator.'],status='ported-awaiting-platform-verification')
 if key in manifest:
  old=manifest[key];assert all(old[k]==v for k,v in item.items() if k!='status'),key;item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n')
print('17 methods; 63 original enum/parameter cases')
