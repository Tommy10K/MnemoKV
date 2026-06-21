param(
    [string]$ApiBaseUrl = "http://127.0.0.1:7380",
    [string]$Dataset = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if ($Dataset -eq "") {
    $Dataset = Join-Path $root "examples/demo-dataset.json"
}
$resolvedDataset = (Resolve-Path -LiteralPath $Dataset).Path
$definition = Get-Content -Raw -LiteralPath $resolvedDataset | ConvertFrom-Json
$base = $ApiBaseUrl.TrimEnd('/')

$health = Invoke-RestMethod -Uri "$base/health" -TimeoutSec 3
Write-Host "Loading '$($definition.description)' into $($health.nodeId)..."

foreach ($command in $definition.commands) {
    $args = @($command | ForEach-Object { [string]$_ })
    $body = @{ args = $args } | ConvertTo-Json -Compress
    $result = Invoke-RestMethod -Method Post -Uri "$base/commands" -ContentType "application/json" -Body $body
    if ($result.type -eq "error") {
        throw "Command '$($args -join ' ')' failed: $($result.value)"
    }
}

function Invoke-DemoRead([string[]]$CommandArgs) {
    $body = @{ args = $CommandArgs } | ConvertTo-Json -Compress
    return Invoke-RestMethod -Method Post -Uri "$base/commands" -ContentType "application/json" -Body $body
}

$visits = Invoke-DemoRead @("GET", "demo:visits")
$queue = Invoke-DemoRead @("LLEN", "demo:queue")
$leaders = Invoke-DemoRead @("ZCARD", "demo:leaderboard")

if ($visits.value -ne "42" -or $queue.value -ne 3 -or $leaders.value -ne 3) {
    throw "Dataset verification failed"
}

Write-Host "Demo dataset loaded and verified: visits=42, queue=3, leaderboard=3."
