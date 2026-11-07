#!/usr/bin/env python3
"""Port pinned NextVersionCalculator real histories; retain all literal assertions.

The three mocked-context tests remain pending: their empty branch logs do not
correspond to real commit histories and require a separate calculator unit seam.
"""
import hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1]
revision=json.loads((root/'baseline.json').read_text())['revision']
source='src/GitVersion.Core.Tests/VersionCalculation/NextVersionCalculatorTests.cs'
raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root)
text=raw.decode();sha=hashlib.sha256(raw).hexdigest()
starts=list(re.finditer(r'^    public void (\w+)\(\)',text,re.M))
assert len(starts)==14
methods={};recipes=[]
for number,m in enumerate(starts):
 name=m[1];body=text[m.end():starts[number+1].start() if number+1<len(starts) else len(text)]
 if 'contextBuilder' in body:continue
 builder='TrunkBased/preview1' if 'TrunkBasedConfigurationBuilder.New' in body else 'GitFlow/v1'
 cfg={'workflow':builder};calc={}
 next_version=re.search(r'\.WithNextVersion\("([^"]+)"\)',body)
 if next_version:calc['next-version']=next_version[1]
 if '"custom"' in body:
  # A new C# BranchConfigurationBuilder defaults to enum None, unlike a
  # deserialized new branch (Inherit). Materialize that builder-only default.
  label='alpha.{BranchName}' if 'alpha.{ConfigurationConstants.BranchNamePlaceholder}' in body else '{BranchName}'
  expression=re.search(r'\.WithRegularExpression\(@"([^"]+)"\)',body);assert expression
  calc['branches']={'custom':{'regex':expression[1],'label':label,'increment':'None','source-branches':[]}}
  if '.WithDeploymentMode(DeploymentMode.ManualDeployment)' in body:calc['branches']['custom']['mode']='ManualDeployment'
 elif name=='PreReleaseNumberShouldBeScopeToPreReleaseLabelInManualDeployment':
  calc['branches']={'main':{'label':'beta','mode':'ManualDeployment'},'feature':{'mode':'ManualDeployment'}}
 if calc:cfg['calculation']=calc
 config=json.dumps(cfg,separators=(',',':'))+'\n'
 steps=[];known={'main'};current='main';assertions=[]
 def commit(message=''):
  step={'op':'commit','id':'commit-%03d'%len(steps)}
  if message:step['message']=message
  steps.append(step)
 def checkout(branch):
  global current
  assert branch in known,branch
  steps.append({'op':'checkout','name':branch});current=branch
 for line in body.splitlines():
  line=line.strip()
  if not line.startswith('fixture.'):continue
  q=re.fullmatch(r'fixture\.(MakeACommit|MakeATaggedCommit|BranchTo|Checkout|CreateBranch|ApplyTag|MergeNoFF|MergeTo)\((.*?)\);',line)
  if q:
   op,arg=q.groups();arg='main' if arg=='MainBranch' else json.loads(arg) if arg else ''
   if op=='MakeACommit':commit(arg)
   elif op=='MakeATaggedCommit':commit();steps.append({'op':'tag','name':arg})
   elif op=='BranchTo':
    assert arg not in known;known.add(arg);steps.append({'op':'branch','name':arg});checkout(arg)
   elif op=='CreateBranch':assert arg not in known;known.add(arg);steps.append({'op':'branch','name':arg})
   elif op=='Checkout':checkout(arg)
   elif op=='ApplyTag':steps.append({'op':'tag','name':arg})
   elif op=='MergeNoFF':steps.append({'op':'merge','id':'merge-%03d'%len(steps),'from':arg,'message':"Merge branch '"+arg+"'"})
   else:
    prev=current;checkout(arg);steps.append({'op':'merge','id':'merge-%03d'%len(steps),'from':prev,'message':"Merge branch '"+prev+"'"})
   continue
  q=re.fullmatch(r'fixture\.AssertFullSemver\("([^"]+)", configuration\);',line)
  if q:
   assertions.append(q[1]);steps.append({'op':'checkpoint','id':'assert-%03d'%(len(assertions)-1),'semver':q[1]});continue
  q=re.fullmatch(r'fixture\.Repository\.Merge\("([^"]+)", Generate\.SignatureNow\(\)\);',line)
  if q:steps.append({'op':'fast-forward','from':q[1]});continue
  raise AssertionError((name,line))
 assert assertions and len(assertions)==len(re.findall(r'fixture\.AssertFullSemver\(',body)),name
 recipe_id='csharp-next-version-%02d'%number
 recipe={'id':recipe_id,'description':name,'config':config,'steps':steps}
 recipes.append(recipe);methods[name]=[{'id':'case-000','recipe_id':recipe_id,'assertions':assertions}]
assert len(methods)==11
recipe_path='testdata/csharp-next-version-recipes.json'
(root/recipe_path).write_text(json.dumps(recipes,indent=2)+'\n')
corpus={'revision':revision,'source':source,'source_sha256':sha,'methods':methods,'recipes':recipe_path,'recipes_sha256':hashlib.sha256((root/recipe_path).read_bytes()).hexdigest()}
path='testdata/csharp-next-version.json';data=json.dumps(corpus,indent=2)+'\n';(root/path).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for name,cases in methods.items():
 key='NextVersionCalculatorTests.'+name
 item=dict(source=source,source_sha256=sha,method=name,go_test='TestCSharpNextVersionCalculator',case_file=path,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['Deterministic go-git histories preserve every original branch, tag, parent and assertion checkpoint. Commits modify separate fixture files instead of the original random content.','MergeNoFF/MergeTo retain true merges; default merges in these cases fast-forward to the original descendant.','Builder-only custom branch increment None is explicitly materialized in YAML; workflow names select the pinned embedded defaults. Three mocked-context methods remain pending.'],status='ported-awaiting-platform-verification')
 if key in manifest:
  old=manifest[key];assert all(old[k]==v for k,v in item.items() if k!='status'),key;item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n')
print(len(methods),'methods;',sum(len(c[0]['assertions']) for c in methods.values()),'original version assertions')
