#!/usr/bin/env python3
"""Bind all pinned locator declarations, including the two same-named methods."""
from pathlib import Path
import json,re,hashlib,subprocess,collections
root=Path(__file__).resolve().parents[1];rev=json.loads((root/'baseline.json').read_text())['revision'];source='src/GitVersion.Configuration.Tests/Configuration/ConfigurationFileLocatorTests.cs';raw=subprocess.check_output(['git','show',rev+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig')
constants={'DefaultFileName':'GitVersion.yml','DefaultAlternativeFileName':'GitVersion.yaml','DefaultFileNameDotted':'.GitVersion.yml','DefaultAlternativeFileNameDotted':'.GitVersion.yaml'}
methods={};matches=list(re.finditer(r'public void (\w+)\(([^)]*)\)',s));counts=collections.Counter(m[1] for m in matches)
for n,m in enumerate(matches):
 name=m[1];before=s[:m.start()];attribute_lines=[]
 for text in reversed(before.splitlines()):
  text=text.strip()
  if not text:continue
  if text.startswith('[Test') and text.endswith(']'):attribute_lines.insert(0,text)
  else:break
 if not attribute_lines:continue
 attributes='\n'.join(attribute_lines)
 group='named' if m.start()>s.index('public class NamedConfigurationFileLocatorTests') else 'default';line=s[:m.start()].count('\n')+1;identity=name+('@'+str(line) if counts[name]>1 else '')
 body=s[m.end():matches[n+1].start() if n+1<len(matches) else len(s)]
 rows=re.findall(r'\[TestCase\(([^\n]*)\)\]',attributes) or [''];cases=[]
 for i,row in enumerate(rows):
  values=[]
  for v in row.split(',') if row else []:
   v=v.strip();values.append(constants[v.split('.')[-1]] if v.startswith('ConfigurationFileLocator.') else json.loads(v))
  c=dict(id='case-%03d'%i,operation='verify',working='@working',repo='@repo',custom='my-config.yaml' if group=='named' else None,files=[],arguments=values,error=False)
  if name=='DiscoveryIsIndependentOfDirectoryEnumerationOrder':c.update(operation='order',reverse=values[0],expected_path='@repo/GitVersion.yml')
  elif name=='ThrowsExceptionOnAmbiguousConfigFileLocation':
   c['files']=['@repo/'+(values[0] if group=='default' else c['custom']),'@working/'+(values[1] if group=='default' else c['custom'])];c['error']=True;c['expected_message']=re.search(r'var expectedMessage = \$"([^"]+)"',body)[1]
  elif name in ['NoWarnOnGitVersionYmlFile','NoWarnOnLowercasedGitVersionYmlFile']:
   c.update(operation='provide',working='@repo');c['files']=['@repo/'+(values[0].lower() if 'Lowercased' in name else values[0])]
  elif name=='NoWarnOnNoGitVersionYmlFile':c.update(operation='provide',working='@repo')
  elif name=='DoNotThrowWhenWorkingAndRepoPathsAreSame':c.update(working='@repo',files=['@repo/my-config.yaml'])
  elif name=='DoNotThrowWhenWorkingAndRepoPathsAreSame_WithDifferentCasing':c.update(working='@lowerrepo',files=['@lowerrepo/my-config.yaml'])
  elif name=='ReturnConfigurationFilePathIfCustomConfigurationIsSet':c.update(operation='get',working='@repo',custom='@repo/Configuration/CustomConfig.yaml',files=['@repo/Configuration/CustomConfig.yaml'],expected_path='@repo/Configuration/CustomConfig.yaml')
  elif name=='ReturnConfigurationFilePathIfCustomConfigurationIsSet_InvalidConfigurationFilePaths':c.update(operation='get',working='@repo',custom=values[0],expected_path=None)
  elif name=='DoNotThrowWhenConfigFileIsInSubDirectoryOfRepoPath':c.update(working='@repo',custom='./src/my-config.yaml',files=['@repo/src/my-config.yaml'])
  elif name in ['NoWarnOnCustomYmlFile','NoWarnOnCustomYmlFileOutsideRepoPath']:
   c.update(operation='provide',working='@repo',files=[('@unrelated/' if 'Outside' in name else '@process/')+'my-config.yaml'],expected_log=re.search(r'stringLogger.ShouldMatch\("([^"]+)"',body)[1])
  elif name=='ThrowsExceptionOnCustomYmlFileDoesNotExist':c.update(error=True,expected_message=re.search(r'var expectedMessage = \$"([^"]+)"',body)[1])
  else:raise AssertionError(name)
  cases.append(c)
 methods[identity]=cases
assert len(methods)==14;out=dict(revision=rev,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),methods=methods);target='testdata/csharp-file-locator.json';data=json.dumps(out,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for method,cases in methods.items():
 key='ConfigurationFileLocatorTests.'+method;old=manifest.get(key)
 item=dict(source=source,source_sha256=out['source_sha256'],method=method,go_test='TestCSharpConfigurationFileLocator',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=[c['id'] for c in cases],adaptations=['The original filesystem mock maps to an injected Go filesystem with independent directory enumeration. Original empty configuration files remain empty.','Fixture-relative paths replace temp directory values; exact original diagnostic templates resolve to the corresponding fixture paths.','Both nested same-named declarations retain separate pinned-line identities. Original null, filename, lowercase, directory, custom path, log and error assertions are retained.','ProvideForDirectory uses the same production locator/default parser as CLI loading; no warning exceptions map to nil Go errors.'],status='ported-awaiting-platform-verification')
 if old:assert all(old[k]==v for k,v in item.items() if k!='status');item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n');print(len(methods),'methods',sum(map(len,methods.values())),'original cases')
