#!/usr/bin/env python3
"""Extract the full pinned template, string/numeric formatter and edge-case suites."""
import hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1];revision=json.loads((root/'baseline.json').read_text())['revision']
string=r'"(?:\\.|[^"\\])*"'
def lit(v):
 v=re.sub(r'\((?:object|int)\?\)null','null',v.strip())
 return json.loads(v)
def attrs(prefix):
 return [json.loads('['+m[1]+']') for m in re.finditer(r'\[TestCase\(([^\n]*)\)\]',prefix)]
classes={'StringFormatWithExtensionTests':('Extensions',34),'StringFormatterTests':('Formatting',6),'NumericFormatterTests':('Formatting',4),'EdgeCaseTests':('Formatting',6)}
for cls,(folder,total) in classes.items():
 source='src/GitVersion.Core.Tests/'+folder+'/'+cls+'.cs';raw=subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent);text=raw.decode('utf-8-sig')
 matches=list(re.finditer(r'public void (\w+)\([^\n]*',text));previous=0;methods={}
 for i,m in enumerate(matches):
  name=m[1];prefix=text[previous:m.start()];end=matches[i+1].start() if i+1<len(matches) else len(text)
  # Include expression-bodied test text on the signature line.
  body=re.split(r'\n\s*\[Test(?:Case)?(?:\(|\])',text[m.start():end])[0]
  parameters=attrs(prefix);previous=m.end();cases=[]
  if not re.search(r'\[Test(?:Case)?(?:\(|\])',prefix):continue
  if cls=='StringFormatWithExtensionTests':
   if parameters:
    if name=='FormatWith_EnvVarAndPropertyAndFallback_DoesNotThrow':
     assert len(parameters)==7
     for template,env,prop,expected in parameters:cases.append(dict(template=template,environment={} if env is None else {'VARIABLE':env},properties={} if prop is None else {'Property':prop},expected=expected))
    elif name=='FormatWith_MissingEnvVarOrPropertyAndNoFallback_Throws':
     assert len(parameters)==4
     cases=[dict(template=a[0],properties={},environment={},error=True) for a in parameters]
    else:raise AssertionError(name)
   else:
    constants={a:lit(b) for a,b in re.findall(r'const string (\w+) = ('+string+r');',body)}
    template=constants.get('target',constants.get('expected'));assert template is not None,name
    expected=constants.get('expected')
    if expected is None:
     e=re.search(r'Is.EqualTo\(('+string+r')\)',body)
     if e:expected=lit(e[1])
    error=bool(re.search(r'Assert.Throws<\w+>',body))
    assert error or expected is not None,name
    env={lit(a):lit(b) for a,b in re.findall(r'SetEnvironmentVariable\(('+string+r'), ('+string+r'|null)\)',body)}
    props={};kind='dictionary'
    anonymous=re.search(r'var propertyObject = new \{(.*?)\};',body,re.S)
    if anonymous:
     for field,value in re.findall(r'(\w+) = ('+string+r'|\((?:int|object)\?\)null|\d+)',anonymous[1]):props[field]=lit(value)
    else:
     assert name=='FormatAssemblyInformationalVersionWithSemanticVersionCustomFormattedVersionSourceDistance',name
     kind='semantic-object'
     for field in ['Major','Minor','Patch','VersionSourceDistance']:
      value=re.search(r'\b'+field+r' = (\d+)',body);assert value,field;props[field]=int(value[1])
     # Preserve the remaining original object setup as fixture data as well.
     props.update({field:lit(value) for field,value in re.findall(r'\b(Branch|VersionSourceSha|Sha|ShortSha) = ('+string+r')',body)})
     props['CommitDate']='2014-03-06 23:59:59Z';props['PreReleaseName']='';props['PreReleaseNumber']=9
     assert 'SemanticVersionPreReleaseTag(string.Empty, 9, true)' in body and '2014-03-06 23:59:59Z' in body
    cases=[dict(template=template,properties=props,environment=env,source_kind=kind,error=error,expected=expected)]
   for c in cases:c['operation']='template'
  elif cls in ['StringFormatterTests','NumericFormatterTests']:
   operation='string' if cls=='StringFormatterTests' else 'numeric'
   if name.startswith('Priority_'):
    priority=re.search(r'Priority.ShouldBe\((\d+)\)',body);assert priority
    cases=[dict(operation='priority-'+operation,priority=int(priority[1]))]
   elif parameters:
    success='result.ShouldBeTrue()' in body
    assert success or 'result.ShouldBeFalse()' in body,name
    for values in parameters:
     if len(values)==3:value,format,expected=values
     elif len(values)==2:value,format=values;expected=''
     elif len(values)==1:assert 'NullValue' in name;value=None;format=values[0];expected=''
     else:raise AssertionError(name)
     cases.append(dict(operation=operation,value=value,format=format,expected=expected,success=success))
   else:
    call=re.search(r'TryFormat\((null|[\d.]+), ('+string+r'), out',body);assert call,name
    assert 'result.ShouldBeFalse()' in body and 'formatted.ShouldBeEmpty()' in body
    cases=[dict(operation=operation,value=lit(call[1]),format=lit(call[2]),success=False,expected='')]
  else:
   operation=re.search(r'public void (Sanitize[A-Za-z]+)_',body)[1]
   if parameters:
    repeat=re.search(r"new string\('(.)', length\)",body);assert repeat
    cases=[dict(operation=operation,input=repeat[1]*a[0],expected=repeat[1]*a[0]) for a in parameters]
   else:
    constant=re.search(r'const string (\w+) = ('+string+r');',body);assert constant
    value=lit(constant[2]);error='Throws' in name;expected=value
    if error:
     msg=re.search(r'ShouldThrowWithMessage<\w+>\(\s*\$('+string+r')\)',body);assert msg,name
     expected=lit(msg[1]).replace('{'+constant[1]+'}',value)
    cases=[dict(operation=operation,input=value,expected=expected,error=error)]
  assert cases,name
  for n,c in enumerate(cases):c['id']='case-%03d'%n
  methods[name]=cases
 assert len(methods)==total,(cls,len(methods))
 output={'revision':revision,'source':source,'source_sha256':hashlib.sha256(raw).hexdigest(),'methods':methods}
 target='testdata/csharp-'+re.sub(r'(?<!^)(?=[A-Z])','-',cls.removesuffix('Tests')).lower()+'.json'
 (root/target).write_text(json.dumps(output,indent=2)+'\n')
 print(cls,len(methods),'methods',sum(map(len,methods.values())),'cases')
