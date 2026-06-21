param(
    [string]$ServerHost = "127.0.0.1",
    [int]$ApiPort = 7380
)

$ErrorActionPreference = "Stop"
$apiBase = "http://${ServerHost}:$ApiPort"

function Invoke-MnemoCommand {
    param([string[]]$CommandArgs)
    $body = @{ args = $CommandArgs } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$apiBase/commands" -ContentType "application/json" -Body $body
}

Invoke-MnemoCommand @("FLUSHDB") | Out-Null
$value = "x" * 180

foreach ($key in @("alpha", "beta")) {
    $result = Invoke-MnemoCommand @("SET", $key, $value)
    if ($result.type -ne "string" -or $result.value -ne "OK") {
        throw "SET $key failed: $($result | ConvertTo-Json -Compress)"
    }
}

$rejected = Invoke-MnemoCommand @("SET", "gamma", $value)
if ($rejected.type -ne "error" -or [string]$rejected.value -notlike "OOM *") {
    throw "noeviction did not reject the over-limit write: $($rejected | ConvertTo-Json -Compress)"
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
if ($state.evictionPolicy -ne "lru" -or [uint64]$state.usedBytes -gt [uint64]$state.memoryLimit -or [uint64]$state.rejectedWrites -lt 1) {
    throw "unexpected engine state: $($state | ConvertTo-Json -Compress)"
}
Write-Host "ok: LRU evicted before admitting growth; used=$($state.usedBytes) limit=$($state.memoryLimit)"
Write-Host "low-memory demo passed"
