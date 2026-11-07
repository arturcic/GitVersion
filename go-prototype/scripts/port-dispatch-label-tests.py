#!/usr/bin/env python3
"""Source-bound formatter dispatch and configuration-label case extraction."""
import hashlib,json,re,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1];revision=json.loads((root/'baseline.json').read_text())['revision']
STR=r'"(?:\\.|[^"\\])*"'
def read(source):return subprocess.check_output(['git','show',revision+':'+source],cwd=root.parent)
def methods(text):
 matches=list(re.finditer(r'public void (\w+)\([^\n]*',text));previous=0
 for i,m in enumerate(matches):
  prefix=text[previous:m.start()];end=matches[i+1].start() if i+1<len(matches) else len(text);previous=m.end()
  if not re.search(r'\[Test(?:Case)?(?:\(|\])',prefix):continue
  body=re.split(r'\n\s*\[Test(?:Case)?(?:\(|\])',text[m.start():end])[0]
  attrs=[x[1] for x in re.finditer(r'\[TestCase\(([^\n]*)\)\]',prefix)]
  yield m[1],attrs,body

def save(source,output,target,count):
 assert len(output)==count,(source,len(output))
 for cases in output.values():
  for i,c in enumerate(cases):c['id']='case-%03d'%i
 raw=read(source);doc={'revision':revision,'source':source,'source_sha256':hashlib.sha256(raw).hexdigest(),'methods':output}
 (root/'testdata'/target).write_text(json.dumps(doc,indent=2)+'\n')
 print(Path(source).stem,len(output),'methods',sum(map(len,output.values())),'cases')

for cls,count in [('FormattableFormatterTests',5),('DateFormatterTests',3)]:
 source='src/GitVersion.Core.Tests/Formatting/'+cls+'.cs';output={}
 for name,attributes,body in methods(read(source).decode('utf-8-sig')):
  op='formattable' if cls.startswith('Formattable') else 'date'
  if name.startswith('Priority'):
   value=int(re.search(r'Priority.ShouldBe\((\d+)\)',body)[1]);cases=[dict(operation='priority-'+op,priority=value)]
  elif 'NullValue' in name:cases=[dict(operation=op,kind='null',format='G' if op=='formattable' else 'yyyy-MM-dd',expected='',success=False)]
  else:
   success='result.ShouldBeFalse()' not in body
   cases=[]
   for attr in attributes:
    value,format,expected=json.loads('['+attr+']')
    kind='date' if op=='date' else 'integer' if isinstance(value,int) else 'float'
    cases.append(dict(operation=op,kind=kind,value=value,format=format,expected=expected,success=success))
  output[name]=cases
 save(source,output,'csharp-'+('formattable-formatter' if cls.startswith('Formattable') else 'date-formatter')+'.json',count)

source='src/GitVersion.Core.Tests/Formatting/ValueFormatterTests.cs';text=read(source).decode('utf-8-sig');output={}
for name,attributes,body in methods(text):
 assert not attributes
 if name=='RemoveFormatter_RemovesExistingFormatter':
  assert 'sut.RemoveFormatter<NumericFormatter>()' in body and 'sut.RemoveFormatter<FormattableFormatter>()' in body
  assert 'before.ShouldBe("123.5")' in body and 'afterFormatted.ShouldBe("123.5")' in body and 'afterNotFormatted.ShouldBeEmpty()' in body
  stages=[dict(op='format',kind='float',value=123.45,format='n1',expected='123.5'),dict(op='remove',name='numeric'),dict(op='format',kind='float',value=123.45,format='n1',expected='123.5',success=True),dict(op='remove',name='formattable'),dict(op='format',kind='float',value=123.45,format='n1',expected='',success=False)]
 elif name in ['RegisterFormatter_AddsNewFormatter','Formatters_ExecuteInPriorityOrder']:
  assert 'formatted.ShouldBe("CUSTOM:test")' in body and 'Priority = 0' in body
  stages=[]
  if name.startswith('Formatters_'):
   assert 'Priority = 99' in body and body.index('RegisterFormatter(lowPriorityFormatter)')<body.index('RegisterFormatter(highPriorityFormatter)')
   stages.append(dict(op='register',priority=99,marker='CUSTOM:'))
  stages += [dict(op='register',priority=0,marker='CUSTOM:'),dict(op='format',kind='string',value='test',format='custom',expected='CUSTOM:test',success=True)]
 else:
  expected=re.search(r'formatted.ShouldBe\(('+STR+r')\)',body)
  expected=json.loads(expected[1]) if expected else ''
  success='result.ShouldBeTrue()' in body
  if name=='TryFormat_Date_UsesDateFormatter':kind='date';value='2023-12-25T00:00:00Z';format='yyyy-MM-dd';assert 'new DateTime(2023, 12, 25, 0, 0, 0, DateTimeKind.Utc)' in body
  elif name=='TryFormat_FormattableObject_UsesFormattableFormatter':kind='decimal';value='123.456';format='C';assert 'const decimal value = 123.456m;' in body
  else:
   call=re.search(r'TryFormat\((null|'+STR+r'|[\d.]+), ('+STR+r')',body);assert call,name
   value=json.loads(call[1]);format=json.loads(call[2]);kind='null' if value is None else 'string' if isinstance(value,str) else 'float'
  stages=[dict(op='format',kind=kind,value=value,format=format,expected=expected,success=success)]
 output[name]=[dict(operation='dispatch',stages=stages)]
save(source,output,'csharp-value-formatter.json',9)

# C# regular/verbatim string expressions and literal concatenation in attributes.
def split(text,separator=','):
 parts=[];start=0;quote=False;verbatim=False;escape=False;i=0
 while i<len(text):
  c=text[i]
  if quote:
   if verbatim and c=='"' and i+1<len(text) and text[i+1]=='"':i+=2;continue
   if escape:escape=False
   elif c=='\\' and not verbatim:escape=True
   elif c=='"':quote=False
  elif c=='"':quote=True;verbatim=i>0 and text[i-1]=='@'
  elif c==separator:parts.append(text[start:i].strip());start=i+1
  i+=1
 parts.append(text[start:].strip());return parts
sha='1234567890abcdef1234567890abcdef12345678'
def literal(text):
 parts=split(text,'+')
 if len(parts)>1:return ''.join(literal(x) for x in parts)
 if text=='CommitSha':return sha
 if text.startswith('@"'):return text[2:-1].replace('""','"')
 return json.loads(text)
source='src/GitVersion.Configuration.Tests/Configuration/ConfigurationExtensionsTests.cs';text=read(source).decode('utf-8-sig');output={}
branch_regex=r'^features?[\/-](?<BranchName>.+)'
for name,attrs,body in methods(text):
 if name in ['EnsureGetEffectiveConfigurationWithoutBranchUsesEmptyBranchConfiguration','EnsureIsReleaseBranchWithReferenceNameWorksAsExpected']:continue
 args=[[literal(x) for x in split(a)] for a in attrs]
 base=dict(operation='label',sha=sha,short_sha='1234567',environment={})
 def case(**values):return dict(base,**values)
 if name=='CommitLabelPlaceholdersComposeWithCapturesAndFallbacks':
  cases=[case(label=a,expected=b,regex=r'^feature/(?<StoryNo>sc-\d+)/(?<BranchName>.+)',branch='feature/sc-42/add-login') for a,b in args]
 elif name=='CommitLabelPlaceholdersDoNotRequireMatchingBranchRegex':
  cases=[case(label='ci.{ShortSha}',expected='ci.1234567',regex=a,branch=b,override=c) for a,b,c in args]
 elif name=='NamedCapturesTakePrecedenceOverCommitPlaceholders':
  cases=[case(label='ci.{'+a+'}',expected=c,regex='^feature/(?<'+a+'>.+)?$',branch='feature/'+b) for a,b,c in args]
 elif name=='BranchOverrideChangesCaptureButKeepsCurrentCommitHash':
  cases=[case(label='{BranchName}.{ShortSha}',regex='^feature/(?<BranchName>.+)',branch='feature/original',override='feature/override',expected='override.1234567')]
 elif name=='EnvironmentValuesComposeWithCommitPlaceholdersAndAreSanitized':
  cases=[case(label='{env:BUILD_LABEL}.{ShortSha}',branch='feature/topic',environment={'BUILD_LABEL':'team/topic'},expected='team-topic.1234567')]
 elif name=='CommitPlaceholdersDoNotSuppressUnknownPlaceholderErrors':
  cases=[case(label=a[0],branch='feature/topic',error=True) for a in args]
 elif name=='CommitPlaceholdersPreserveMalformedFallbackErrors':
  cases=[case(label='{env:MISSING_VAR ??? "fallback"}.{ShortSha}',branch='feature/topic',error=True)]
 elif name=='LabelResolutionRequiresExplicitCurrentCommit':
  cases=[case(label=a[0],branch='feature/topic',missing_commit=True,error=True) for a in args]
 elif name=='EnsureGetBranchSpecificLabelWorksAsExpected':
  cases=[case(branch=a,regex=b,label=c,expected=d) for a,b,c,d in args]
 elif name=='EnsureGetBranchSpecificLabelProcessesEnvironmentVariables':
  cases=[case(branch='pull-request',regex='^pull[/-]',label='pr-{env:GITHUB_HEAD_REF}',environment={'GITHUB_HEAD_REF':'feature-branch'},expected='pr-feature-branch')]
 elif name=='EnsureGetBranchSpecificLabelProcessesEnvironmentVariablesWithFallback':
  cases=[case(branch='pull-request',regex='^pull[/-]',label='pr-{env:GITHUB_HEAD_REF ?? "unknown"}',expected='pr-unknown')]
 elif name=='EnsureGetBranchSpecificLabelProcessesEnvironmentVariablesAndRegexPlaceholders':
  cases=[case(branch='feature/test-branch',regex=branch_regex,label='{BranchName}-{env:GITHUB_HEAD_REF}',environment={'GITHUB_HEAD_REF':'feature-branch'},expected='test-branch-feature-branch')]
 elif name=='EnsureGetBranchSpecificLabelWorksWithoutEnvironmentWhenNoEnvPlaceholders':
  cases=[case(branch='feature/test',regex=branch_regex,label='{BranchName}',expected='test')]
 elif name=='EnsureGetBranchSpecificLabelThrowsWhenEnvVarMissing':
  cases=[case(branch='pull-request',regex='^pull[/-]',label='pr-{env:MISSING_VAR}',error=True)]
 elif name=='EnsureGetBranchSpecificLabelThrowsWhenBranchNamePropertyMissing':
  cases=[case(branch='pull-request',regex=r'^features?[\/-]',label='{BranchName}',error=True)]
 elif name=='EnsureGetBranchSpecificLabelReturnsValidLabelForEnvironmentVariables':
  cases=[case(branch='feature/test-feature',regex=branch_regex,label='{env:GITHUB_HEAD_REF}',environment={'GITHUB_HEAD_REF':a},expected=b) for a,b in args]
 else:raise AssertionError(name)
 assert cases
 # Retain original assertions: positive cases use actual.ShouldBe and failures
 # throw at the configured call boundary. These bodies are manually reviewed.
 assert 'ShouldBe(' in body or 'Should.Throw<' in body,name
 output[name]=cases
save(source,output,'csharp-configuration-labels.json',16)
