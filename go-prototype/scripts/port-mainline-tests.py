#!/usr/bin/env python3
"""Fail-closed translation of pinned declarative Mainline NUnit scenarios.

Expected versions are always copied from TestCase literals. No Go calculation,
C# execution, or synthesized expected-version formula participates in extraction.
"""
import copy,hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1]
revision=json.loads((root/'baseline.json').read_text())['revision']
prefix='src/GitVersion.Core.Tests/Mainline/MainlineScenariosWithAGitHubFlow+'
paths=subprocess.check_output(['git','ls-tree','-r','--name-only',revision,'--','src/GitVersion.Core.Tests/Mainline'],cwd=root.parent,text=True).splitlines()
paths=[p for p in paths if p.startswith(prefix) and p.endswith('.cs')]
assert len(paths)==79,len(paths)
outdir=root/'testdata/csharp-mainline';outdir.mkdir(exist_ok=True)
manifest=json.loads((root/'testdata/csharp-unit-port-mappings.json').read_text())
index=[]
strpat=r'"(?:\\.|[^"\\])*"'
def literal(s,args):
 s=s.strip()
 if s in args:return args[s]
 if s=='string.Empty':return ''
 if s.startswith(('IncrementStrategy.','DeploymentMode.','CommitMessageIncrementMode.')):return s.split('.')[1]
 return json.loads(s)

def branches(chain,args,config):
 # Each WithBranch contains a lambda composed exclusively of these scalar setters.
 while chain.strip():
  chain=chain.strip()
  m=re.match(r'\.WithBranch\(("main"|"feature"),\s*b\s*=>\s*b',chain);assert m,chain
  name=json.loads(m[1]);chain=chain[m.end():];branch=config['calculation']['branches'].setdefault(name,{})
  while True:
   chain=chain.lstrip()
   if chain.startswith(')'):chain=chain[1:];break
   m=re.match(r'\.With(DeploymentMode|Increment|Label|CommitMessageIncrementing|PreventIncrementWhenCurrentCommitTagged|PreventIncrementOfMergedBranch)\(([^()]*)\)',chain);assert m,chain
   field,val=m[1],literal(m[2],args);chain=chain[m.end():]
   if field.startswith('PreventIncrement'):
    key='when-current-commit-tagged' if field.endswith('WhenCurrentCommitTagged') else 'of-merged-branch'
    branch.setdefault('prevent-increment',{})[key]=val
   else:
    key={'DeploymentMode':'mode','Increment':'increment','Label':'label','CommitMessageIncrementing':'commit-message-incrementing'}[field]
    # Preserve builder null separately from an explicit empty label.
    branch[key]=val
 return config

total_methods=total_cases=0
for number,source in enumerate(paths):
 raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig');sha=hashlib.sha256(raw).hexdigest()
 scenario=Path(source).stem.split('+')[1]
 builder=re.search(r'private static (Git(?:Hub)?Flow)ConfigurationBuilder MainlineBuilder =>\s*\1ConfigurationBuilder.New(.*?);',s,re.S);assert builder,source
 chain=builder[2]
 for exact in ['.WithVersionStrategy(VersionStrategies.Mainline)','.WithLabel(null)']:
  assert chain.count(exact)==1,(source,exact);chain=chain.replace(exact,'')
 cfg=branches(chain,{},dict(workflow=builder[1]+'/v1',calculation=dict(strategies=['Mainline'],branches={})))
 setup=re.search(r'public void OneTimeSetUp\(\)\s*\{(.*?)\n        \}',s,re.S);assert setup
 setup_lines=[x.strip() for x in setup[1].splitlines() if x.strip() and not x.strip().startswith('//')]
 init=setup_lines.pop(0);m=re.fullmatch(r'this.fixture = new EmptyRepositoryFixture\(('+strpat+r')?\);',init);assert m,init
 initial=json.loads(m[1]) if m[1] else 'main';current=initial;known={initial};steps=[]
 for line in setup_lines:
  m=re.fullmatch(r'this.fixture\.(MakeACommit|BranchTo|Checkout|ApplyTag|MergeTo)\(('+strpat+r')(, removeBranchAfterMerging: true)?\);',line);assert m,line
  op,arg,remove=m[1],json.loads(m[2]),m[3]
  assert not remove or op=='MergeTo'
  if op=='MakeACommit':steps.append(dict(op='commit',id='commit-'+str(len(steps)),message=arg))
  elif op=='ApplyTag':steps.append(dict(op='tag',name=arg))
  elif op=='BranchTo':
   assert arg not in known;known.add(arg);steps.extend([dict(op='branch',name=arg),dict(op='checkout',name=arg)]);current=arg
  elif op=='Checkout':assert arg in known;steps.append(dict(op='checkout',name=arg));current=arg
  else:
   assert arg in known
   prev=current;steps.extend([dict(op='checkout',name=arg),dict(op='merge',id='merge-'+str(len(steps)),**{'from':prev},message="Merge branch '"+prev+"'")]);current=arg
   if remove:steps.append(dict(op='delete-branch',name=prev));known.remove(prev)
 steps.append(dict(op='checkpoint',id='head'))
 recipe=dict(id='csharp-mainline-%02d'%number,description=scenario,initial_branch=initial,config='workflow: '+builder[1]+'/v1\n',steps=steps)
 methods={};last=setup.end()
 for m in re.finditer(r'public string (GetVersion\w*)\((.*?)\)\s*\{(.*?)\n        \}',s,re.S):
  name=m[1];params=[]
  for param in m[2].split(','):
   q=re.fullmatch(r'\s*(IncrementStrategy|string\?)\s+(\w+)\s*',param);assert q,param;params.append(q[2])
  attrs=re.findall(r'\[TestCase\(([^\n]*)\)\]',s[last:m.start()]);assert attrs,(source,name);last=m.end()
  body=re.fullmatch(r'\s*var mainline = MainlineBuilder(.*?)\.Build\(\);\s*return this.fixture!\.GetVersion\(mainline\).FullSemVer;\s*',m[3],re.S);assert body,(source,name,m[3])
  cases=[]
  for i,attr in enumerate(attrs):
   q=re.fullmatch(r'(.*), ExpectedResult = ('+strpat+r')',attr);assert q,attr
   args=[literal(x,{}) for x in q[1].split(',')];assert len(args)==len(params)
   casecfg=branches(body[1],dict(zip(params,args)),copy.deepcopy(cfg))
   null_labels=[k for k,v in casecfg['calculation']['branches'].items() if 'label' in v and v['label'] is None]
   for k in null_labels:casecfg['calculation']['branches'][k]['label']=''
   cases.append(dict(id='case-%03d'%i,arguments=args,null_labels=null_labels,config=json.dumps(casecfg,separators=(',',':')),expected=json.loads(q[2])))
  methods[name]=cases
 assert sum(map(len,methods.values()))==len(re.findall(r'\[TestCase\(',s)),source
 assert len(methods)==len(re.findall(r'public string GetVersion',s)),source
 target='testdata/csharp-mainline/'+scenario+'.json'
 doc=dict(revision=revision,source=source,source_sha256=sha,recipe=recipe,methods=methods)
 data=json.dumps(doc,indent=2)+'\n';(root/target).write_text(data)
 for method,cases in methods.items():
  key=Path(source).stem+'.'+method
  old=manifest.get(key)
  item=dict(source=source,source_sha256=sha,method=method,go_test='TestCSharpMainlineScenarios',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['Fixture commits use deterministic times, identities and independent text files; original parent order, branch/tag positions and messages are retained.','MergeTo creates a true no-fast-forward merge and deletes the source only when the original requests it.','Explicit builder null labels are retained as nullable effective labels; null and empty original arguments remain distinct, including tag matching and preservation.','The same production calculator accepts the parsed builder projection without modifying the fixture configuration file. Expected FullSemVer values are original C# TestCase literals.'],status='ported-awaiting-platform-verification')
  if old:
   assert old['status']=='ported-awaiting-platform-verification' or all(old[k]==v for k,v in item.items() if k!='status'),key
   item['status']=old['status']
  manifest[key]=item
 index.append(dict(file=target,source=source,methods=len(methods),cases=sum(map(len,methods.values()))))
 total_methods+=len(methods);total_cases+=sum(map(len,methods.values()))
(root/'testdata/csharp-mainline-index.json').write_text(json.dumps(dict(revision=revision,scenarios=index,methods=total_methods,cases=total_cases),indent=2)+'\n')
(root/'testdata/csharp-unit-port-mappings.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(len(index),'scenarios',total_methods,'methods',total_cases,'original cases')
