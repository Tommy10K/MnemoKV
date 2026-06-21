param(
    [ValidateSet("json", "binary", "all")]
    [string]$Format = "all"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
$nodeExe = Join-Path $binDir "mnemokv-node.exe"
$apiBase = "http://127.0.0.1:7380"

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

function Invoke-MnemoCommand {
    param([string[]]$CommandArgs)
    $body = @{ args = $CommandArgs } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$apiBase/commands" -ContentType "application/json" -Body $body
}

function Stop-DemoNode {
    param([System.Diagnostics.Process]$Process)
    if ($null -ne $Process -and -not $Process.HasExited) {
        Stop-Process -Id $Process.Id
        $Process.WaitForExit()
    }
}

function Start-DemoNode {
    param([string]$ConfigPath, [string]$DataDir, [string]$RunName)
    $stdout = Join-Path $DataDir "$RunName.stdout.log"
    $stderr = Join-Path $DataDir "$RunName.stderr.log"
    Start-Process -FilePath $nodeExe -ArgumentList @("-config", "`"$ConfigPath`"") -WorkingDirectory $root -RedirectStandardOutput $stdout -RedirectStandardError $stderr -WindowStyle Hidden -PassThru
}

New-Item -ItemType Directory -Force $binDir | Out-Null
Push-Location $root
try {
    & go build -o $nodeExe ./cmd/node
    if ($LASTEXITCODE -ne 0) { throw "failed to build node" }
}
finally {
    Pop-Location
}

$formats = if ($Format -eq "all") { @("json", "binary") } else { @($Format) }
foreach ($currentFormat in $formats) {
    $configPath = Join-Path $root "configs\standalone-persistence-$currentFormat.yaml"
    $dataDir = Join-Path $root "data\standalone-persistence-$currentFormat"
    $allowedDataRoot = [System.IO.Path]::GetFullPath((Join-Path $root "data"))
    $allowedDataPrefix = $allowedDataRoot.TrimEnd([char[]]"\/") + [System.IO.Path]::DirectorySeparatorChar
    $resolvedDataDir = [System.IO.Path]::GetFullPath($dataDir)
    if (-not $resolvedDataDir.StartsWith($allowedDataPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing to clean data directory outside $allowedDataRoot"
    }
    if (Test-Path -LiteralPath $dataDir) {
        Remove-Item -LiteralPath $dataDir -Recurse -Force
    }
    New-Item -ItemType Directory -Force $dataDir | Out-Null

    $node = $null
    try {
        $node = Start-DemoNode $configPath $dataDir "before-restart"
        Wait-ForApi $node
        Invoke-MnemoCommand @("SET", "persist:string", "value-$currentFormat") | Out-Null
        Invoke-MnemoCommand @("RPUSH", "persist:list", "one", "two") | Out-Null
        Invoke-MnemoCommand @("ZADD", "persist:zset", "1", "alice", "2", "bob") | Out-Null
        $snapshot = Invoke-RestMethod -Method Post -Uri "$apiBase/admin/snapshot"
        $snapshotPath = if ([System.IO.Path]::IsPathRooted($snapshot.path)) { $snapshot.path } else { Join-Path $root $snapshot.path }
        if ($snapshot.format -ne $currentFormat -or -not (Test-Path -LiteralPath $snapshotPath)) {
            throw "snapshot was not created correctly: $($snapshot | ConvertTo-Json -Compress)"
        }
        Stop-DemoNode $node
        $node = $null

        $node = Start-DemoNode $configPath $dataDir "after-restart"
        Wait-ForApi $node
        $string = Invoke-MnemoCommand @("GET", "persist:string")
        $listLength = Invoke-MnemoCommand @("LLEN", "persist:list")
        $zset = Invoke-MnemoCommand @("ZRANGE", "persist:zset", "0", "-1", "WITHSCORES")
        $zsetText = (($zset.value | ForEach-Object { [string]$_.value }) -join "|")
        if ($string.value -ne "value-$currentFormat" -or [int]$listLength.value -ne 2 -or $zsetText -ne "alice|1|bob|2") {
            throw "$currentFormat restore did not reproduce the dataset"
        }
        Write-Host "ok: $currentFormat snapshot restored strings, lists, and sorted sets"
    }
    finally {
        Stop-DemoNode $node
    }
}
Write-Host "persistence demo passed"
