#!/usr/bin/env python3
"""Extract original build-agent test parameters from the pinned C# sources."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

root=Path(__file__).resolve().parent.parent
revision=json.loads((root/'baseline.json').read_text())['revision']
sources=subprocess.check_output(['git','ls-tree','-r','--full-tree','--name-only',revision,'src/GitVersion.BuildAgents.Tests/Agents'],cwd=root,text=True).splitlines()
suites={}
token=re.compile(r'"""(.*?)"""|("(?:[^"\\]|\\.)*")|(null)',re.S)
for source in sources:
    if not source.endswith('.cs'):continue
    raw=subprocess.check_output(['git','show',f'{revision}:{source}'],cwd=root)
    text=raw.decode()
    matches=list(re.finditer(r'((?:^    \[[^\n]+\]\n)+)    public void (\w+)\(',text,re.M))
    methods={}
    for index,m in enumerate(matches):
        attrs,name=m.groups()
        if not re.search(r'\[Test(?:Case\(|\])',attrs):continue
        cases=[]
        for arguments in re.findall(r'^    \[TestCase\((.*)\)\]$',attrs,re.M):
            values=[]
            for item in token.finditer(arguments):
                values.append(item[1] if item[1] is not None else json.loads(item[2]) if item[2] else None)
            cases.append(values)
        body=text[m.end():matches[index+1].start() if index+1<len(matches) else len(text)]
        body=body.split('\n    private ',1)[0]
        methods[name]={'cases':[{'id':f'case-{i:03d}','arguments':args} for i,args in enumerate(cases or [[]])]}
    suites[Path(source).stem]={'source':source,'sha256':hashlib.sha256(raw).hexdigest(),'methods':methods}
assert sum(len(s['methods']) for s in suites.values())==79
target='testdata/csharp-build-agents.json'
data=json.dumps({'revision':revision,'suites':suites},indent=2)+'\n'
(root/target).write_text(data)
path=root/'testdata/csharp-unit-port-mappings.json'
manifest=json.loads(path.read_text())
for suite,details in suites.items():
    for method,item in details['methods'].items():
        key=suite+'.'+method
        entry={
            'source':details['source'],'source_sha256':details['sha256'],'method':method,
            'go_test':'TestCSharpBuildAgents','case_file':target,'case_file_sha256':hashlib.sha256(data.encode()).hexdigest(),
            'case_ids':[case['id'] for case in item['cases']],
            'adaptations':['Go adapters use injected environment lookups and local temporary files. The semantic variable provider supplies the original version and metadata fixtures. AppVeyor wire JSON is tested without a remote service.'],
            'status':'ported-awaiting-platform-verification',
        }
        if key in manifest:
            old=manifest[key]
            assert all(old[k]==value for k,value in entry.items() if k!='status'),key
            entry['status']=old['status']
        manifest[key]=entry
path.write_text(json.dumps(manifest,indent=2)+'\n')
print('79 methods and',sum(len(m['cases']) for s in suites.values() for m in s['methods'].values()),'original cases')
