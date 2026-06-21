param(
    [switch]$KeepRunning
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
$binary = Join-Path $binDir "mnemokv-node.exe"
$runRoot = Join-Path ([IO.Path]::GetTempPath()) ("mnemokv-cluster-" + [Guid]::NewGuid().ToString("N"))
$processes = @{}
$apiByNode = @{ "node-1" = 7381; "node-2" = 7382; "node-3" = 7383 }

function Invoke-NodeCommand([int]$ApiPort, [string[]]$CommandArgs) {
    $body = @{ args = $CommandArgs } | ConvertTo-Json -Compress
    return Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:$ApiPort/commands" -ContentType "application/json" -Body $body
}

function Get-FnvSlot([string]$Key, [int]$SlotCount) {
    [uint32]$hash = 2166136261
    foreach ($byte in [Text.Encoding]::UTF8.GetBytes($Key)) {
        $hash = [uint32](([uint64]($hash -bxor $byte) * 16777619) % [uint64]4294967296)
    }
    return [int]($hash % $SlotCount)
}

function Wait-ForNode([int]$ApiPort) {
    $deadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $deadline) {
        try {
            $null = Invoke-RestMethod -Uri "http://127.0.0.1:$ApiPort/health" -TimeoutSec 1
            return
        } catch {
            Start-Sleep -Milliseconds 100
        }
    }
    throw "node API on port $ApiPort did not become ready"
}

try {
    New-Item -ItemType Directory -Force -Path $binDir, $runRoot | Out-Null
    & go build -o $binary (Join-Path $root "cmd/node")
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }

    foreach ($index in 1..3) {
        $source = Join-Path $root "configs/cluster-node-$index.yaml"
        $config = Join-Path $runRoot "node-$index.yaml"
        $data = (Join-Path $runRoot "data/node-$index").Replace("\", "/")
        (Get-Content -Raw $source).Replace("./data/node-$index", $data) | Set-Content -NoNewline $config
        $processes["node-$index"] = Start-Process -FilePath $binary -ArgumentList @("-config", $config) -PassThru -WindowStyle Hidden
    }

    foreach ($port in 7381..7383) { Wait-ForNode $port }

    $states = 7381..7383 | ForEach-Object { Invoke-RestMethod -Uri "http://127.0.0.1:$_/cluster/state" }
    $maps = $states | ForEach-Object {
        ($_.slots | ForEach-Object { "$($_.number):$($_.leaderId):$($_.replicaId):$($_.term)" }) -join "|"
    }
    if (($maps | Select-Object -Unique).Count -ne 1) { throw "nodes disagree on the slot map" }

    $key = "demo:cluster:key"
    $set = Invoke-NodeCommand 7383 @("SET", $key, "routed-value")
    if ($set.type -ne "string" -or $set.value -ne "OK") { throw "routed SET failed: $($set | ConvertTo-Json -Compress)" }
    foreach ($port in 7381..7383) {
        $get = Invoke-NodeCommand $port @("GET", $key)
        if ($get.value -ne "routed-value") { throw "routed GET failed through API port $port" }
    }

    if ($states[0].slotCount -lt 1 -or $states[0].slots.Count -ne $states[0].slotCount) {
        throw "cluster API did not return the authoritative slot table"
    }
    $slot = Get-FnvSlot $key $states[0].slotCount
    $assignment = $states[0].slots[$slot]
    $replicaProcess = $processes[$assignment.replicaId]
    Stop-Process -Id $replicaProcess.Id -Force
    $replicaProcess.WaitForExit()
    $liveGatewayPort = $apiByNode[$assignment.leaderId]
    $rejected = Invoke-NodeCommand $liveGatewayPort @("SET", $key, "must-not-commit")
    if ($rejected.type -ne "error") { throw "write unexpectedly succeeded while replica was down" }

    $other = "demo:other:0"
    while ((Get-FnvSlot $other $states[0].slotCount) -eq $slot) {
        $other = "demo:other:" + ([int]($other.Split(":")[-1]) + 1)
    }
    $crossSlot = Invoke-NodeCommand $liveGatewayPort @("DEL", $key, $other)
    if ($crossSlot.type -ne "error" -or $crossSlot.value -notmatch "CROSSSLOT") { throw "cross-slot DEL was not rejected" }

    Write-Host "Cluster demo passed: identical metadata, any-node routing, strict replica acknowledgement, and CROSSSLOT rejection."
    if ($KeepRunning) {
        Write-Host "The remaining nodes are running. Press Enter to stop them."
        [void](Read-Host)
    }
} finally {
    foreach ($process in $processes.Values) {
        if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
    }
    if (Test-Path -LiteralPath $runRoot) { Remove-Item -LiteralPath $runRoot -Recurse -Force }
}
