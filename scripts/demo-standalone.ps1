param(
    [string]$ServerHost = "127.0.0.1",
    [int]$RespPort = 6380,
    [int]$ApiPort = 7380,
    [int]$WorkloadSeconds = 2
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$apiBase = "http://${ServerHost}:$ApiPort"

function Invoke-MnemoCommand {
    param([string[]]$CommandArgs)
    $body = @{ args = $CommandArgs } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$apiBase/commands" -ContentType "application/json" -Body $body
}

function Assert-Result {
    param($Result, [string]$Type, $Value, [string]$Label)
    if ($Result.type -ne $Type) {
        throw "$Label returned type '$($Result.type)', expected '$Type'"
    }
    if ($null -ne $Value -and [string]$Result.value -ne [string]$Value) {
        throw "$Label returned '$($Result.value)', expected '$Value'"
    }
    Write-Host "ok: $Label"
}

function Send-MalformedResp {
    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $client.ReceiveTimeout = 3000
        $client.SendTimeout = 3000
        $client.Connect($ServerHost, $RespPort)
        $stream = $client.GetStream()
        $payload = [System.Text.Encoding]::ASCII.GetBytes("*x`r`n")
        $stream.Write($payload, 0, $payload.Length)
        $bytes = [System.Collections.Generic.List[byte]]::new()
        while ($true) {
            $next = $stream.ReadByte()
            if ($next -lt 0) { break }
            $bytes.Add([byte]$next)
            if ($next -eq 10) { break }
        }
        return [System.Text.Encoding]::ASCII.GetString($bytes.ToArray()).TrimEnd([char[]]"`r`n")
    }
    finally {
        $client.Dispose()
    }
}

Invoke-RestMethod -Method Get -Uri "$apiBase/health" | Out-Null
Assert-Result (Invoke-MnemoCommand @("FLUSHDB")) "string" "OK" "FLUSHDB"

Assert-Result (Invoke-MnemoCommand @("SET", "demo:string", "hello")) "string" "OK" "SET string"
Assert-Result (Invoke-MnemoCommand @("GET", "demo:string")) "bulk" "hello" "GET string"
Assert-Result (Invoke-MnemoCommand @("INCR", "demo:counter")) "integer" 1 "INCR counter"

Assert-Result (Invoke-MnemoCommand @("SET", "demo:ttl", "alive", "EX", "30")) "string" "OK" "SET with TTL"
$ttl = Invoke-MnemoCommand @("TTL", "demo:ttl")
if ($ttl.type -ne "integer" -or [int]$ttl.value -le 0 -or [int]$ttl.value -gt 30) {
    throw "TTL returned '$($ttl.value)', expected 1..30"
}
Write-Host "ok: TTL is $($ttl.value)s"

Assert-Result (Invoke-MnemoCommand @("RPUSH", "demo:list", "first", "second")) "integer" 2 "RPUSH list"
Assert-Result (Invoke-MnemoCommand @("LPOP", "demo:list")) "bulk" "first" "LPOP list"
Assert-Result (Invoke-MnemoCommand @("LLEN", "demo:list")) "integer" 1 "LLEN list"

Assert-Result (Invoke-MnemoCommand @("ZADD", "demo:scores", "10", "alice", "20", "bob")) "integer" 2 "ZADD sorted set"
$range = Invoke-MnemoCommand @("ZRANGE", "demo:scores", "0", "-1", "WITHSCORES")
$rangeText = (($range.value | ForEach-Object { [string]$_.value }) -join "|")
if ($range.type -ne "array" -or $rangeText -ne "alice|10|bob|20") {
    throw "ZRANGE returned '$rangeText'"
}
Write-Host "ok: ZRANGE sorted set"

$protocolReply = Send-MalformedResp
if ($protocolReply -ne "-ERR Protocol error") {
    throw "malformed RESP returned '$protocolReply'"
}
Write-Host "ok: malformed RESP rejected"

Push-Location $root
try {
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = "go"
    $startInfo.Arguments = "run ./cmd/workload -addr ${ServerHost}:$RespPort -profile mixed -concurrency 2 -duration ${WorkloadSeconds}s -keyspan 50 -seed 42"
    $startInfo.WorkingDirectory = $root
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $workload = [System.Diagnostics.Process]::new()
    $workload.StartInfo = $startInfo
    try {
        if (-not $workload.Start()) { throw "failed to start workload" }
        $workloadOutput = $workload.StandardOutput.ReadToEnd()
        $workloadErrors = $workload.StandardError.ReadToEnd()
        $workload.WaitForExit()
        $workloadExitCode = $workload.ExitCode
        $workloadOutput = ($workloadErrors + $workloadOutput).Trim()
    }
    finally {
        $workload.Dispose()
    }
    if ($workloadExitCode -ne 0 -or $workloadOutput -notmatch "errors=0") {
        throw "workload failed: $workloadOutput"
    }
    Write-Host "ok: workload ($workloadOutput)"
}
finally {
    Pop-Location
}

$metrics = Invoke-RestMethod -Method Get -Uri "$apiBase/metrics/summary"
if ([uint64]$metrics.counters.'cmd.total' -le 0) {
    throw "metrics did not report cmd.total"
}
Write-Host "ok: metrics cmd.total=$($metrics.counters.'cmd.total')"
Write-Host "standalone demo passed"
