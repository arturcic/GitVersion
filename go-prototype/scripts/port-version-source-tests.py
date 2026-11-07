#!/usr/bin/env python3
"""Port all original version-source cases and assertions from immutable C# source."""
import json,re,subprocess,hashlib,copy
from pathlib import Path
root=Path(__file__).resolve().parents[1];rev=json.loads((root/'baseline.json').read_text())['revision'];source='src/GitVersion.Core.Tests/VersionCalculation/VersionSourceTests.cs';raw=subprocess.check_output(['git','show',rev+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig')
source_methods=list(re.finditer(r'public void (\w+)\(([^)]*)\)\s*\{(.*?)\n    }',s,re.S));assert len(source_methods)==19
methods={}
def lit(v,args):
 v=v.strip()
 if v in args:return args[v]
 if v=='null':return None
 if v in ['true','false']:return v=='true'
 if v.startswith(('VersionField.','DeploymentMode.')):return v.split('.')[1]
 if v.startswith('$"'):return re.sub(r'\{(\w+)\}',lambda m:str(args[m[1]]),json.loads(v[1:]))
 if v.startswith('"'):return json.loads(v)
 if re.fullmatch(r'\d+',v):return int(v)
 if '?' in v:
  cond,a,b=re.fullmatch(r'(.+?) \? (.+) : (.+)',v).groups();ok=args[cond] if cond in args else args[cond.split(' == ')[0]]==int(cond.split(' == ')[1]);return lit(a if ok else b,args)
 return '@'+v
class History:
 def __init__(self):self.steps=[];self.n=0
 def commit(self,alias=None,tag=None):
  self.n+=1;self.steps.append(dict(op='commit',id='commit-'+str(self.n)))
  if tag:self.steps.append(dict(op='tag',name=tag))
  if alias:self.steps.append(dict(op='checkpoint',id=alias.lower()))
 def branch(self,name):self.steps.extend([dict(op='branch',name=name),dict(op='checkout',name=name)])
 def checkout(self,name):self.steps.append(dict(op='checkout',name=name))
 def merge(self,name,alias=None):
  self.n+=1;self.steps.append(dict(op='merge',id='merge-'+str(self.n),**{'from':name},message="Merge branch '"+name+"'"))
  if alias:self.steps.append(dict(op='checkpoint',id=alias.lower()))
for m in source_methods:
 name,params,body=m.groups();start=m.start();prefix=s[s.rfind('\n    [',0,start):start]
 attrs=re.findall(r'\[TestCase\((.*?)\)\]',s[max(0,s.rfind('\n    }',0,start)):start])
 params=[x.strip().split()[-1] for x in params.split(',') if x.strip()]
 attrs=attrs or [''];cases=[]
 for n,attr in enumerate(attrs):
  vals=[lit(v,{}) for v in attr.split(',')] if attr else [];args=dict(zip(params,vals));assert len(args)==len(params)
  h=History();cfg=dict(workflow='GitHubFlow/v1',calculation={});op='repository';special={}
  if name=='ConfiguredVersionKeepsExternalSemanticSource':
   h.commit('tagSha','1.0.0')
   for i in range(args['commitsAfterTag']):h.commit()
   cfg['calculation']['next-version']='5.0.0'
  elif name=='BranchNameKeepsExternalSemanticSource':h.commit('tagSha','1.0.0');h.branch('release/5.0.0');h.commit();cfg['workflow']='GitFlow/v1'
  elif name=='TrackedReleaseKeepsSemanticOriginSeparateFromMergeBase':
   h.commit(tag='1.2.1');h.branch('develop');h.commit('mergeBaseSha');h.branch(args['releaseBranch']);h.commit('tagSha','1.3.0-beta.1');h.checkout('develop');h.commit();cfg['workflow']='GitFlow/v1'
  elif name=='ExternalOnlySourceCountsReachableHistory':h.commit();h.commit();cfg['calculation']['next-version']='5.0.0'
  elif name=='OrdinaryTagRetainsSelectedIncrement':h.commit('tagSha','1.0.0');h.commit()
  elif name=='TaggedHeadUsesTagForBothSources':
   h.commit()
   if args['inheritedIncrement']:h.branch('feature/inherited')
   h.commit('tagSha','2.3.4');cfg['calculation']['branches']={'feature':{'increment':'Inherit','prevent-increment':{'when-current-commit-tagged':True}}}
  elif name=='DeploymentModesPreserveSourceMetadata':h.commit('tagSha','1.0.0');h.commit();h.commit();cfg['calculation']['branches']={'main':{'label':'beta','mode':args['mode']}}
  elif name=='IgnoredCommitInMergeGraphIsExcludedFromDistance':
   h.commit('tagSha','1.0.0');h.branch('feature/counting');h.commit('ignoredSha');h.commit();h.checkout('main');h.commit();h.merge('feature/counting');cfg['calculation'].update({'next-version':'5.0.0','ignore':{'sha':['@ignoredSha']}})
  elif name=='ReleaseBranchPresenceDoesNotChangeLatestSource':
   cfg['workflow']='GitFlow/v1';h.commit(tag='0.1.0');h.branch('develop');h.commit();h.branch('release/0.2.0');h.commit();h.checkout('main');h.merge('release/0.2.0');h.steps.extend([dict(op='tag',name='0.2.0'),dict(op='checkpoint',id='releasetagsha')])
   if args['deleteReleaseBranch']:h.steps.append(dict(op='delete-branch',name='release/0.2.0'))
   h.checkout('develop');h.merge('main');cfg['calculation']['branches']={'develop':{'mode':'ManualDeployment','track-merge-message':False}}
  elif name=='AlternativeTagFloorReplacesSemanticSourceWithoutResettingCount':h.commit('countSha','1.0.0');h.branch('feature/work');h.commit('floorSha','5.0.0-other.1')
  elif name in ['MainlineRepeatedIncrementsKeepTagOrigin','MainlineMergeKeepsChildSemanticOrigin']:
   cfg['calculation'].update({'strategies':['ConfiguredNextVersion','Mainline'],'branches':{'main':{'mode':'ContinuousDeployment','prevent-increment':{'of-merged-branch':True}},'feature':{'increment':'Minor','mode':'ContinuousDelivery','prevent-increment':{'when-current-commit-tagged':False}}}})
   h.commit('tagSha','1.0.0')
   if name=='MainlineRepeatedIncrementsKeepTagOrigin':h.commit('previousSha');h.commit()
   else:h.branch('feature/work');h.commit();h.commit();h.checkout('main');h.merge('feature/work')
  elif name=='MergeMessageKeepsSupplyingCommitSeparateFromOlderTag':
   cfg['workflow']='GitFlow/v1';h.commit('tagSha','1.0.0');h.branch('release/'+args['releaseVersion']);h.commit();h.checkout('main');h.merge('release/'+args['releaseVersion'],'mergeSha');h.commit()
  elif name=='StableWinnerExcludesPreReleaseCandidateFromCountingSelection':
   h.commit('stableSha','1.0.0');h.commit('preReleaseSha','4.0.0-alpha.1');h.commit();cfg['calculation']['branches']={'main':{'mode':'ContinuousDeployment'}};op='injected-strategy'
  elif name in ['VersionSourceSha','VersionSourceShaUsingTag','VersionSourceShaOneCommit']:
   cfg['workflow']='GitFlow/v1';h.commit()
   if name!='VersionSourceShaOneCommit':
    h.branch('develop');h.commit('secondCommitSha','1.0.0' if name=='VersionSourceShaUsingTag' else None);h.branch('feature/foo');h.commit()
  elif name in ['MainlineFoldKeepsExternalOriginWhenOperatorChangesCountAnchor','OperatorAlternativeFloorUsesSupplyingSourceOnlyWhenItWins']:
   op='operator';special=args
  else:raise AssertionError(name)
  expected={}
  if op!='operator':
   for field,expr in re.findall(r'(?:version|semanticVersion)\.BuildMetaData\.(\w+)(?:\.ShouldNotBeNull\(\)\.ToString\(\))?\.ShouldBe\((.*?)\);',body):expected[field]=lit(expr,args)
   for field in re.findall(r'(?:version|semanticVersion)\.BuildMetaData\.(\w+)\.ShouldBeNull\(\);',body):expected[field]=None
   full=re.search(r'version\.ToString\("f"\)\.ShouldBe\((.*?)\);',body)
   if full:expected['FullSemVer']=lit(full[1],args)
   assert expected,name
  else:
   if name.startswith('MainlineFold'):expected={'version':'5.1.0-1','anchor':'nextCountCommit','source_sha':None,'source_version':'5.0.1-1','increment':'Minor'}
   else:expected={'version':lit('alternativeMajor == 5 ? "5.0.0-1" : "1.0.1-1"',args),'anchor':'originalCommit','source_sha':'alternativeCommit' if args['alternativeMajor']==5 else 'originalCommit','source_version':args['sourceVersion'],'increment':args['increment']}
  h.steps.append(dict(op='checkpoint',id='head'))
  recipe=dict(id='version-source-'+str(len(methods))+'-'+str(n),description=name,config=json.dumps({'workflow':cfg['workflow']}),steps=h.steps)
  cases.append(dict(id='case-%03d'%n,operation=op,arguments=vals,recipe=recipe,config=json.dumps(cfg),expected=expected,special=special))
 methods[name]=cases
assert len(methods)==19
out=dict(revision=rev,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),methods=methods);target='testdata/csharp-version-sources.json';data=json.dumps(out,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for name,cases in methods.items():
 key='VersionSourceTests.'+name;old=manifest.get(key)
 item=dict(source=source,source_sha256=out['source_sha256'],method=name,go_test='TestCSharpVersionSources',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['Deterministic go-git history replaces repository fixture identities/timestamps/random files; parent order, tags and branch operations are retained.','Original assertions retain every source/counting identity and distance. Named SHA variables resolve to original setup checkpoint identities. Model-null legacy SHA maps to the empty Go trace string, independently of the legacy JSON formatter empty-string contract.','TrunkBased builder is projected to the same explicit Mainline strategy, modes, increments and prevent-increment settings used by these scenarios.','Original injected mock strategy uses the shared production strategy callback; operator cases use the production Mainline base/operator model.'],status='ported-awaiting-platform-verification')
 if old:
  assert old['status']=='ported-awaiting-platform-verification' or all(old[k]==v for k,v in item.items() if k!='status');item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n');print(len(methods),'methods',sum(map(len,methods.values())),'original cases')
