param([switch]$Live)
$ErrorActionPreference = 'Stop'
if(-not $Live){throw 'Add -Live to explicitly run one billable DeepSeek draft request.'}
$settings=@{}
Get-Content -LiteralPath (Join-Path $PSScriptRoot '.env') | ForEach-Object {$pair=$_ -split '=',2;if($pair.Length -eq 2){$settings[$pair[0]]=$pair[1]}}
$headers=@{Authorization=('Bearer '+$settings['DEMO_API_TOKEN'])}
$base='http://127.0.0.1:8080'
$metadata=Invoke-RestMethod ($base+'/api/integrations') -Headers $headers -TimeoutSec 10
if(-not $metadata.ai.configured){throw 'DeepSeek is not configured in the running backend. Check .env and recreate the api service.'}
$result=Invoke-RestMethod ($base+'/api/ai/draft') -Method Post -Headers $headers -ContentType 'application/json' -Body '{"leadId":"L-1001"}' -TimeoutSec 35
if($result.mode -ne 'deepseek' -or [string]::IsNullOrWhiteSpace($result.draft)){throw 'Expected a real DeepSeek response, not a local template.'}
[pscustomobject]@{Result='PASS';Provider='DeepSeek';Mode=$result.mode;Model=$result.model;Draft=$result.draft;ExternalMessagesSent=0}
