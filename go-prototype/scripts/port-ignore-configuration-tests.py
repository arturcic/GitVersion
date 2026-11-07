#!/usr/bin/env python3
"""Preserve all original v6 ignore configuration methods and assertions."""
from pathlib import Path
import subprocess,json,hashlib,re,textwrap
root=Path(__file__).resolve().parents[1];rev=json.loads((root/'baseline.json').read_text())['revision'];source='src/GitVersion.Configuration.Tests/Configuration/IgnoreConfigurationTests.cs';raw=subprocess.check_output(['git','show',rev+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig');methods={}
# Original tests explicitly select v6; never rewrite their YAML into v7 for the test.
assert 'EnvironmentVariableName, "v6"' in s
for m in re.finditer(r'\[Test\]\s*public void (\w+)\(\)\s*\{(.*?)\n    }',s,re.S):
 name,body=m.groups();case=dict(id='case-000',operation='decode',expected={})
 literal=re.search(r'const string yaml =\s*(""".*?"""|"(?:\\.|[^"\\])*");',body,re.S)
 if literal:
  value=literal[1];case['yaml']=textwrap.dedent(value[3:-3]).strip('\n') if value.startswith('"""') else json.loads(value)
  case['error']='Should.Throw<YamlException>' in body
  for field,values in re.findall(r'configuration.Ignore\.(\w+)\.ShouldBe\(\[(.*?)\]\);',body):case['expected'][field]=json.loads('['+values+']')
  for field in re.findall(r'configuration.Ignore\.(\w+)\.ShouldBeEmpty\(\)',body):case['expected'][field]=[]
  if 'Before.ShouldBe(null)' in body:case['expected']['Before']=None
  before=re.search(r'Before.ShouldBe\(DateTimeOffset.Parse\("([^"]+)"',body)
  if before:case['expected']['Before']=before[1]
  if 'VersionStrategy.ShouldBe' in body:case['expected']['Strategies']=re.findall(r'VersionStrategies\.(\w+)',body)
 elif 'InvalidIgnore' in name:
  case['operation']='validate';case['field']='Branches' if 'Branch' in name else 'Tags';case['values']=['[invalid'];case['error']=True
  case['contains']=json.loads(re.search(r'exception.Message.ShouldContain\(("[^"]+")\)',body)[1]);case['not_contains']='[invalid'
 else:
  field='Branches' if 'Branch' in name else 'Tags';values=re.search(r'(?:Branches|Tags) = \[(.*?)\]',body)
  builder=re.search(r'With(?:Branches|Tags)\((.*?)\)',body)
  case['field']=field;case['values']=json.loads('['+(values[1] if values else builder[1] if builder else '')+']')
  if 'Serialize_' in name:case['operation']='serialize';case['contains_all']=[json.loads(v) for v in re.findall(r'yaml.ShouldContain\(("[^"]+")\)',body)]
  elif 'Builder_' in name:case['operation']='builder';expected=re.search(r'ignoreConfig.\w+.ShouldBe\(\[(.*?)\]\)',body);case['expected'][field]=json.loads('['+expected[1]+']')
  else:case['operation']='empty';case['expected']['IsEmpty']='IsEmpty.ShouldBeTrue' in body
 methods[name]=[case]
assert len(methods)==18;out=dict(revision=rev,source=source,source_sha256=hashlib.sha256(raw).hexdigest(),methods=methods);target='testdata/csharp-ignore-configuration.json';data=json.dumps(out,indent=2)+'\n';(root/target).write_text(data)
p=root/'testdata/csharp-unit-port-mappings.json';manifest=json.loads(p.read_text())
for method,cases in methods.items():
 key='IgnoreConfigurationTests.'+method;old=manifest.get(key)
 item=dict(source=source,source_sha256=out['source_sha256'],method=method,go_test='TestCSharpIgnoreConfiguration',case_file=target,case_file_sha256=hashlib.sha256(data.encode()).hexdigest(),case_ids=['case-000'],adaptations=['Original v6 document strings are decoded through the legacy document loader, separately from effective calculation validation.','Go value records and empty slices represent the original nonnull configuration/default empty collections. Date assertions compare the original local wall-clock instant using the host local zone, as DateTimeOffset.Parse does.','Ordered distinct slices replace HashSet/builder collections; Go errors replace YamlException/ConfigurationException, retaining all explicitly asserted diagnostic containment/privacy.','Strategy flag combinations map to the same set of enabled strategy names. Serialization preserves all original substring assertions.'],status='ported-awaiting-platform-verification')
 if old:assert all(old[k]==v for k,v in item.items() if k!='status');item['status']=old['status']
 manifest[key]=item
p.write_text(json.dumps(manifest,indent=2)+'\n');print('18 methods / 18 original cases')
