param(
    [switch]$ReturnNode,
    [switch]$KeepRunning,
    [int]$PreFailureDelaySeconds = 10
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
$binary = Join-Path $binDir "mnemokv-node.exe"
$runRoot = Join-Path ([IO.Path]::GetTempPath()) ("mnemokv-auto-recovery-" + [Guid]::NewGuid().ToString("N"))
$processes = @{}
$apiPorts = 7381..7385
$failedNode = "node-1"
$failedPort = 7381

function Wait-ForNode([int]$ApiPort, [int]$TimeoutSeconds = 20) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $health = Invoke-RestMethod -Uri "http://127.0.0.1:$ApiPort/health" -TimeoutSec 1
            if ($health.status -eq "ok") { return }
        } catch {}
        Start-Sleep -Milliseconds 200
    }
    throw "node API on port $ApiPort did not become ready"
}

function Get-LiveClusterState {
    foreach ($port in $apiPorts) {
        if ($port -eq $failedPort -and -not $ReturnNode) { continue }
        try {
            return Invoke-RestMethod -Uri "http://127.0.0.1:$port/cluster/state" -TimeoutSec 2
        } catch {}
    }
    throw "no live cluster API is reachable"
}

function Wait-ForControllerLeader([int]$TimeoutSeconds = 30) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        foreach ($port in $apiPorts) {
            try {
                $state = Invoke-RestMethod -Uri "http://127.0.0.1:$port/controller/state" -TimeoutSec 1
                if ($state.isLeader) { return $state }
            } catch {}
        }
        Start-Sleep -Milliseconds 250
    }
    throw "automatic controller did not elect a leader"
}

function Wait-ForHealthyTopology([string[]]$ExcludedNodes, [int]$ExpectedActiveNodes, [int]$TimeoutSeconds = 180) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $seen = [Collections.Generic.HashSet[string]]::new()
    while ((Get-Date) -lt $deadline) {
        try {
            $state = Get-LiveClusterState
            $status = [string]$state.recovery.state
            if ($status) {
                [void]$seen.Add($status)
                $plan = $state.recovery.activePlan
                if ($null -ne $plan) {
                    Write-Host ("controller: {0} {1}/{2}" -f $status, $plan.completedSteps, $plan.totalSteps)
                }
            }
            if ($status -eq "healthy" -and $null -eq $state.recovery.activePlan) {
                $leaders = @{}
                $replicas = @{}
				$validTopology = $true
                foreach ($slot in $state.slots) {
                    if (-not $slot.replicaReady -or $slot.leaderId -eq $slot.replicaId) {
						$validTopology = $false
						break
                    }
                    if ($ExcludedNodes -contains $slot.leaderId -or $ExcludedNodes -contains $slot.replicaId) {
						$validTopology = $false
						break
                    }
                    if (-not $leaders.ContainsKey($slot.leaderId)) { $leaders[$slot.leaderId] = 0 }
                    if (-not $replicas.ContainsKey($slot.replicaId)) { $replicas[$slot.replicaId] = 0 }
                    $leaders[$slot.leaderId]++
                    $replicas[$slot.replicaId]++
                }
				if ($validTopology -and $leaders.Count -eq $ExpectedActiveNodes -and $replicas.Count -eq $ExpectedActiveNodes) {
					return @{ State = $state; Seen = $seen; Leaders = $leaders; Replicas = $replicas }
                }
            }
		} catch {}
        Start-Sleep -Milliseconds 500
    }
    throw "cluster did not return to a healthy, fully replicated topology; observed states: $($seen -join ', ')"
}

function Start-DemoNode([int]$Index) {
    $config = Join-Path $runRoot "node-$Index.yaml"
    return Start-Process -FilePath $binary -ArgumentList @("-config", $config) -PassThru -WindowStyle Hidden
}

try {
    New-Item -ItemType Directory -Force -Path $binDir, $runRoot | Out-Null
    & go build -o $binary (Join-Path $root "cmd/node")
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }

    foreach ($index in 1..5) {
        $source = Join-Path $root "configs/cluster-node-$index-auto.yaml"
        $config = Join-Path $runRoot "node-$index.yaml"
        $data = (Join-Path $runRoot "data/node-$index").Replace("\", "/")
        $content = (Get-Content -Raw $source).Replace("./data/auto/node-$index", $data)
        $content = $content.Replace("slotCount: 1024", "slotCount: 512")
        $content = $content.Replace("failureTimeoutMs: 10000", "failureTimeoutMs: 3000")
        $content = $content.Replace("migrationRateLimit: 10", "migrationRateLimit: 500")
        Set-Content -NoNewline -Path $config -Value $content
        $processes["node-$index"] = Start-DemoNode $index
    }

    foreach ($port in $apiPorts) { Wait-ForNode $port }
    $leader = Wait-ForControllerLeader
    $initial = Wait-ForHealthyTopology @() 5 60
    Write-Host "Automatic cluster is healthy; controller leader is $($leader.nodeId), term $($leader.raftTerm)."
    if ($PreFailureDelaySeconds -gt 0) {
        Write-Host "Waiting $PreFailureDelaySeconds seconds before stopping $failedNode."
        Start-Sleep -Seconds $PreFailureDelaySeconds
    }

    $failedProcess = $processes[$failedNode]
    Stop-Process -Id $failedProcess.Id -Force
    $failedProcess.WaitForExit()
    Write-Host "$failedNode stopped; observing promotion, repair, and rebalance."

    $recovered = Wait-ForHealthyTopology @($failedNode) 4 180
    Write-Host "Recovered 4-node placement. Leaders: $($recovered.Leaders | ConvertTo-Json -Compress)"
    Write-Host "Observed recovery states: $($recovered.Seen -join ' -> ')"

    if ($ReturnNode) {
        $processes[$failedNode] = Start-DemoNode 1
        Wait-ForNode $failedPort
        Write-Host "$failedNode returned behind the fresh-data admission gate; waiting for 4-to-5 rebalance."
        $scaled = Wait-ForHealthyTopology @() 5 180
        Write-Host "Returned node admitted and balanced. Leaders: $($scaled.Leaders | ConvertTo-Json -Compress)"
    }

    Write-Host "Automatic recovery demo passed: quorum promotion, full replica repair, balanced ownership, and no empty slot recreation."
    if ($KeepRunning) {
        Write-Host "Nodes are running. Press Enter to stop them."
        [void](Read-Host)
    }
} finally {
    foreach ($process in $processes.Values) {
        if ($null -ne $process -and -not $process.HasExited) { Stop-Process -Id $process.Id -Force }
    }
    $resolvedRunRoot = [IO.Path]::GetFullPath($runRoot)
    $resolvedTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolvedRunRoot.StartsWith($resolvedTemp) -and (Test-Path -LiteralPath $resolvedRunRoot)) {
        Remove-Item -LiteralPath $resolvedRunRoot -Recurse -Force
    }
}
