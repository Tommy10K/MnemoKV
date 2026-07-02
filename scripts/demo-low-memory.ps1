param(
    [int]$MemoryLimitBytes = 250,
    [switch]$KeepRunning
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
$nodeExe = Join-Path $binDir "mnemokv-node.exe"
$runRoot = Join-Path ([IO.Path]::GetTempPath()) ("mnemokv-low-memory-" + [Guid]::NewGuid().ToString("N"))
$configPath = Join-Path $runRoot "standalone-low-memory.yaml"
$dataDir = Join-Path $runRoot "data"
$apiBase = "http://127.0.0.1:7380"

function Invoke-MnemoCommand {
    param([string[]]$CommandArgs)
    $body = @{ args = $CommandArgs } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$apiBase/commands" -ContentType "application/json" -Body $body
}

function Wait-ForApi {
    param([System.Diagnostics.Process]$Process)
    $deadline = [DateTime]::UtcNow.AddSeconds(8)
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($Process.HasExited) {
            throw "node exited with code $($Process.ExitCode)"
        }
        try {
            Invoke-RestMethod -Method Get -Uri "$apiBase/health" -TimeoutSec 1 | Out-Null
            return
        }
        catch {
            Start-Sleep -Milliseconds 100
        }
    }
    throw "node API did not become ready"
}

function Stop-DemoNode {
    param([System.Diagnostics.Process]$Process)
    if ($null -ne $Process -and -not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force
        $Process.WaitForExit()
    }
}

$node = $null
try {
    New-Item -ItemType Directory -Force -Path $binDir, $runRoot, $dataDir | Out-Null
    Push-Location $root
    try {
        & go build -o $nodeExe ./cmd/node
        if ($LASTEXITCODE -ne 0) { throw "failed to build node" }
    }
    finally {
        Pop-Location
    }

    $data = $dataDir.Replace("\", "/")
    $config = Get-Content -Raw -Path (Join-Path $root "configs\standalone-low-memory.yaml")
    $config = $config.Replace("memoryLimitBytes: 512", "memoryLimitBytes: $MemoryLimitBytes")
    $config = $config.Replace("./data/standalone-low-memory", $data)
    Set-Content -NoNewline -Path $configPath -Value $config

    $stdout = Join-Path $runRoot "node.stdout.log"
    $stderr = Join-Path $runRoot "node.stderr.log"
    $node = Start-Process -FilePath $nodeExe -ArgumentList @("-config", "`"$configPath`"") -WorkingDirectory $root -RedirectStandardOutput $stdout -RedirectStandardError $stderr -WindowStyle Hidden -PassThru
    Wait-ForApi $node

    Invoke-MnemoCommand @("FLUSHDB") | Out-Null
    $value = "x" * 55

    foreach ($key in @("alpha", "beta")) {
        $result = Invoke-MnemoCommand @("SET", $key, $value)
        if ($result.type -ne "string" -or $result.value -ne "OK") {
            throw "SET $key failed: $($result | ConvertTo-Json -Compress)"
        }
    }

    $rejected = Invoke-MnemoCommand @("SET", "gamma", $value)
    if ($rejected.type -ne "error" -or [string]$rejected.value -notlike "OOM *") {
        $state = Invoke-RestMethod -Method Get -Uri "$apiBase/engine/state"
        throw "noeviction did not reject the over-limit write: $($rejected | ConvertTo-Json -Compress); state=$($state | ConvertTo-Json -Compress)"
    }
    $before = Invoke-MnemoCommand @("EXISTS", "alpha", "beta", "gamma")
    if ([int]$before.value -ne 2) {
        throw "noeviction removed existing keys"
    }
    Write-Host "ok: noeviction rejected growth and preserved existing keys"

    $policy = Invoke-RestMethod -Method Post -Uri "$apiBase/engine/eviction-policy" -ContentType "application/json" -Body '{"policy":"lru"}'
    if ($policy.policy -ne "lru") {
        throw "failed to switch to LRU"
    }
    $accepted = Invoke-MnemoCommand @("SET", "gamma", $value)
    if ($accepted.type -ne "string" -or $accepted.value -ne "OK") {
        throw "LRU did not admit the write: $($accepted | ConvertTo-Json -Compress)"
    }
    $after = Invoke-MnemoCommand @("EXISTS", "alpha", "beta", "gamma")
    if ([int]$after.value -ne 2) {
        throw "LRU did not evict exactly one key"
    }
    $state = Invoke-RestMethod -Method Get -Uri "$apiBase/engine/state"
    if ($state.evictionPolicy -ne "lru" -or [uint64]$state.usedBytes -gt [uint64]$state.memoryLimit -or [uint64]$state.memoryLimit -ne [uint64]$MemoryLimitBytes -or [uint64]$state.rejectedWrites -lt 1) {
        throw "unexpected engine state: $($state | ConvertTo-Json -Compress)"
    }
    Write-Host "ok: LRU evicted before admitting growth; used=$($state.usedBytes) limit=$($state.memoryLimit)"
    Write-Host "low-memory demo passed"
    if ($KeepRunning) {
        Write-Host "Low-memory node is still running at RESP 127.0.0.1:6380 and API http://127.0.0.1:7380. Press Enter to stop it."
        [void](Read-Host)
    }
}
finally {
    Stop-DemoNode $node
    $resolvedRunRoot = [IO.Path]::GetFullPath($runRoot)
    $resolvedTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolvedRunRoot.StartsWith($resolvedTemp) -and (Test-Path -LiteralPath $resolvedRunRoot)) {
        Remove-Item -LiteralPath $resolvedRunRoot -Recurse -Force
    }
}
