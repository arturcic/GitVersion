#!/usr/bin/env python3
"""Bind all pinned version-filter methods to their complete original assertions."""
from pathlib import Path
import json,subprocess,hashlib,re
root=Path(__file__).resolve().parents[1];rev=json.loads((root/'baseline.json').read_text())['revision'];manifest=json.loads((root/'testdata/csharp-unit-port-mappings.json').read_text());methods={};previous=dict(manifest)
for cls in ['MinDateVersionFilterTests','ShaVersionFilterTests','PathFilterTests']:
 source='src/GitVersion.Core.Tests/VersionCalculation/'+cls+'.cs';raw=subprocess.check_output(['git','show',rev+':'+source],cwd=root.parent);s=raw.decode('utf-8-sig');names=re.findall(r'\[Test\]\s*public void (\w+)\(\)',s);assert len(names)==4
 for name in names:
  body=re.search(r'public void '+name+r'\(\)\s*\{(.*?)\n    }',s,re.S)[1]
  error='Should.Throw<ArgumentNullException>' in body
  excluded='ShouldBeTrue()' in body
  assert error or ('ShouldBeFalse()' in body)!=excluded
  commit=None if 'WithNullCommit' in name or error else dict(sha='0123456789abcdef0123456789abcdef01234567',when='2026-01-01T00:00:00Z',paths=['/path'] if cls=='PathFilterTests' else [])
  # Original UtcNow +/- one year becomes a fixed instant +/- one year.
  minimum='2027-01-01T00:00:00Z' if excluded or commit is None else '2025-01-01T00:00:00Z'
  prefixes=[commit['sha']] if excluded and commit else ['mismatched']
  paths=['/path'] if excluded or commit is None else ['/another_path']
  case=dict(id='case-000',filter=cls.removesuffix('Tests'),commit=commit,minimum=minimum,prefixes=prefixes,paths=paths,null_version=error,error=error,excluded=excluded,reason_nonempty=excluded)
  key=cls+'.'+name;methods[key]=case
  item=dict(source=source,source_sha256=hashlib.sha256(raw).hexdigest(),method=name,go_test='TestCSharpVersionFilters',case_file='testdata/csharp-version-filters.json',case_ids=['case-000'],adaptations=['Original relative UtcNow/commit mock uses a fixed UTC commit instant and fixed past/future years.','Go errors replace null-argument exceptions; all original exclude flags and null/nonblank reason assertions are retained.','Mock commit paths and SHA become an immutable commit value; null version and null commit remain distinct.'],status='ported-awaiting-platform-verification')
  manifest[key]=item

for cls in ['MinDateVersionFilterTests','ShaVersionFilterTests','PathFilterTests']:
 selected={k:v for k,v in methods.items() if k.startswith(cls+'.')}
 source='src/GitVersion.Core.Tests/VersionCalculation/'+cls+'.cs'
 first=manifest[next(iter(selected))]
 target='testdata/csharp-'+re.sub(r'(?<!^)(?=[A-Z])','-',cls.removesuffix('Tests')).lower()+'.json'
 out=dict(revision=rev,source=source,source_sha256=first['source_sha256'],methods={})
 for k,c in selected.items():
  if c['null_version']:
   if cls=='PathFilterTests':c['paths']=[]
   if cls=='ShaVersionFilterTests':c['prefixes']=['0123456789abcdef0123456789abcdef01234567']
   if cls=='MinDateVersionFilterTests':c['minimum']='2026-01-01T00:00:01Z'
  out['methods'][k.split('.')[1]]=[c]
 data=json.dumps(out,indent=2)+'\n';(root/target).write_text(data)
 for k in selected:
  manifest[k]['case_file']=target;manifest[k]['case_file_sha256']=hashlib.sha256(data.encode()).hexdigest()
  if k in previous:
   assert all(previous[k][field]==value for field,value in manifest[k].items() if field!='status'),k
   manifest[k]['status']=previous[k]['status']
(root/'testdata/csharp-unit-port-mappings.json').write_text(json.dumps(manifest,indent=2)+'\n')
print('12 source methods / 12 original cases')
