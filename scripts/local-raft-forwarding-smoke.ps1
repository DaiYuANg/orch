[CmdletBinding()]
param(
    [string]$Manifest = "examples/local-raft-forwarding.yaml",
    [string]$WorkDir = ".orch-raft-smoke",
    [int]$TimeoutSeconds = 120,
    [switch]$KeepServer,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$nodes = @(
    @{ ID = "node-a"; HTTP = "127.0.0.1:17501"; Raft = "127.0.0.1:7451" },
    @{ ID = "node-b"; HTTP = "127.0.0.1:17502"; Raft = "127.0.0.1:7452" },
    @{ ID = "node-c"; HTTP = "127.0.0.1:17503"; Raft = "127.0.0.1:7453" }
)
$appName = "raft-forwarding"
$namespace = "default"
$workRoot = Join-Path $repoRoot $WorkDir
$binDir = Join-Path $workRoot "bin"
$logDir = Join-Path $workRoot "logs"
$dataDir = Join-Path $workRoot "data"

function Assert-UnderRepo {
    param([Parameter(Mandatory = $true)][string]$Path)
    $full = [System.IO.Path]::GetFullPath($Path)
    $repoPrefix = $repoRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if (-not $full.StartsWith($repoPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to use path outside repository: $full"
    }
    return $full
}

$workRoot = Assert-UnderRepo $workRoot
$binDir = Assert-UnderRepo $binDir
$logDir = Assert-UnderRepo $logDir
$dataDir = Assert-UnderRepo $dataDir

if (-not $KeepServer) {
    foreach ($path in @($logDir, $dataDir)) {
        if (Test-Path $path) {
            Remove-Item -LiteralPath $path -Recurse -Force
        }
    }
}
New-Item -ItemType Directory -Force $binDir, $logDir, $dataDir | Out-Null

function Test-IsWindows {
    return [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::Windows
    )
}

$binExt = ""
if (Test-IsWindows) {
    $binExt = ".exe"
}
$serverBin = Join-Path $binDir "orch-server$binExt"
$cliBin = Join-Path $binDir "orch$binExt"

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed ($LASTEXITCODE): $FilePath $($Arguments -join ' ')"
    }
}

function Invoke-CLIJson {
    param(
        [Parameter(Mandatory = $true)][string]$ServerURL,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )
    $cmdArgs = @("--server", $ServerURL) + $Arguments
    $raw = & $cliBin @cmdArgs
    if ($LASTEXITCODE -ne 0) {
        throw "orch CLI failed ($LASTEXITCODE): --server $ServerURL $($Arguments -join ' ')"
    }
    $text = ($raw | Out-String).Trim()
    if ($text -eq "") {
        return @()
    }
    $parsed = $text | ConvertFrom-Json
    if ($null -eq $parsed) {
        return @()
    }
    if ($parsed -is [array]) {
        return $parsed
    }
    return @($parsed)
}

function Wait-OrchHealth {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [Parameter(Mandatory = $true)][string]$URL,
        [Parameter(Mandatory = $true)][string]$Stdout,
        [Parameter(Mandatory = $true)][string]$Stderr
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if ($Process.HasExited) {
            throw "orch-server exited early with code $($Process.ExitCode). See $Stdout and $Stderr"
        }
        & $cliBin --server $URL health *> $null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for orch-server health at $URL"
}

function Get-RaftStatus {
    param([Parameter(Mandatory = $true)][string]$ServerURL)
    return (Invoke-CLIJson -ServerURL $ServerURL -Arguments @("raft", "status", "--json"))[0]
}

function Get-JSONProperty {
    param(
        [Parameter(Mandatory = $true)]$Object,
        [Parameter(Mandatory = $true)][string]$Name,
        $Default = $null
    )
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $Default
    }
    return $prop.Value
}

function Get-RaftLeaderID {
    param([Parameter(Mandatory = $true)]$Status)
    $leaderID = [string](Get-JSONProperty -Object $Status -Name "leaderId" -Default "")
    if ($leaderID -eq "" -and [bool](Get-JSONProperty -Object $Status -Name "isLeader" -Default $false)) {
        $leaderID = [string](Get-JSONProperty -Object $Status -Name "nodeId" -Default "")
    }
    return $leaderID
}

function Wait-RaftLeader {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        foreach ($node in $nodes) {
            $url = "http://$($node.HTTP)"
            try {
                $status = Get-RaftStatus -ServerURL $url
                $leaderID = Get-RaftLeaderID -Status $status
                if ([bool](Get-JSONProperty -Object $status -Name "ready" -Default $false) -and [bool](Get-JSONProperty -Object $status -Name "isLeader" -Default $false) -and $leaderID -ne "") {
                    return $status
                }
            }
            catch {
            }
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for Raft leader"
}

function Wait-RaftMembers {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $allReady = $true
        foreach ($node in $nodes) {
            $status = Get-RaftStatus -ServerURL "http://$($node.HTTP)"
            $members = Get-JSONProperty -Object $status -Name "members" -Default @()
            $leaderID = Get-RaftLeaderID -Status $status
            if (-not [bool](Get-JSONProperty -Object $status -Name "ready" -Default $false) -or $members.Count -ne 3 -or $leaderID -eq "") {
                $allReady = $false
                break
            }
        }
        if ($allReady) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for all Raft members"
}

function Wait-AppPresence {
    param([bool]$Present)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $allMatch = $true
        foreach ($node in $nodes) {
            $items = Invoke-CLIJson -ServerURL "http://$($node.HTTP)" -Arguments @("get", "apps", "--json")
            $found = $items | Where-Object { $_.name -eq $appName -and $_.namespace -eq $namespace } | Select-Object -First 1
            if ($Present -and $null -eq $found) {
                $allMatch = $false
                break
            }
            if (-not $Present -and $null -ne $found) {
                $allMatch = $false
                break
            }
        }
        if ($allMatch) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    if ($Present) {
        throw "Timed out waiting for app replication"
    }
    throw "Timed out waiting for app deletion replication"
}

function Set-SmokeEnvironment {
    param([Parameter(Mandatory = $true)][hashtable]$Vars)
    $previous = @{}
    foreach ($key in $Vars.Keys) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$Vars[$key], "Process")
    }
    return $previous
}

function Restore-SmokeEnvironment {
    param([hashtable]$Previous)
    foreach ($key in $Previous.Keys) {
        [Environment]::SetEnvironmentVariable($key, $Previous[$key], "Process")
    }
}

function Start-RaftNode {
    param([Parameter(Mandatory = $true)][hashtable]$Node)

    $nodeData = Join-Path $dataDir $Node.ID
    New-Item -ItemType Directory -Force $nodeData | Out-Null
    $stdout = Join-Path $logDir "$($Node.ID).out.log"
    $stderr = Join-Path $logDir "$($Node.ID).err.log"
    $previous = Set-SmokeEnvironment @{
        ORCH_DATA_DIR = $nodeData
    }
    try {
        $serverArgs = @(
            "--http-addr", $Node.HTTP,
            "--raft-node-id", $Node.ID,
            "--raft-bind", $Node.Raft,
            "--raft-advertise", $Node.Raft,
            "--raft-peers", $raftPeers,
            "--raft-bootstrap=true",
            "--raft-data-dir", (Join-Path $nodeData "dragonboat"),
            "--cluster-nodes", $clusterNodes,
            "--ingress-enabled=false",
            "--dns-enabled=false",
            "--observability-prometheus-enabled=false",
            "--observability-otlp-enabled=false",
            "--log-level", "info"
        )
        $startArgs = @{
            FilePath               = $serverBin
            WorkingDirectory       = $repoRoot
            ArgumentList           = $serverArgs
            RedirectStandardOutput = $stdout
            RedirectStandardError  = $stderr
            PassThru               = $true
        }
        if (Test-IsWindows) {
            $startArgs.WindowStyle = "Hidden"
        }
        $proc = Start-Process @startArgs
        return @{
            Process = $proc
            Stdout  = $stdout
            Stderr  = $stderr
            URL     = "http://$($Node.HTTP)"
            ID      = $Node.ID
        }
    }
    finally {
        Restore-SmokeEnvironment $previous
    }
}

if (-not $SkipBuild) {
    Invoke-Checked go @("build", "-o", $serverBin, "./cmd/orch-server")
    Invoke-Checked go @("build", "-o", $cliBin, "./cmd/orch-cli")
}

if (-not (Test-Path $serverBin)) {
    throw "Server binary not found: $serverBin"
}
if (-not (Test-Path $cliBin)) {
    throw "CLI binary not found: $cliBin"
}

$clusterNodes = ($nodes | ForEach-Object { "$($_.ID)=http://$($_.HTTP)" }) -join ","
$raftPeers = ($nodes | ForEach-Object { "$($_.ID)=$($_.Raft)" }) -join ","
$manifestPath = (Resolve-Path (Join-Path $repoRoot $Manifest)).Path

$processes = @()
try {
    foreach ($node in $nodes) {
        $started = Start-RaftNode -Node $node
        $processes += $started
    }
    foreach ($started in $processes) {
        Wait-OrchHealth -Process $started.Process -URL $started.URL -Stdout $started.Stdout -Stderr $started.Stderr
    }
    Wait-RaftMembers
    $leader = Wait-RaftLeader
    $leaderID = Get-RaftLeaderID -Status $leader
    $follower = $nodes | Where-Object { $_.ID -ne $leaderID } | Select-Object -First 1
    if ($null -eq $follower) {
        throw "Could not find follower; leader=$leaderID"
    }
    $followerURL = "http://$($follower.HTTP)"

    Write-Host "Raft leader:   $leaderID"
    Write-Host "Follower used: $($follower.ID)"
    Write-Host ""
    Invoke-Checked $cliBin @("--server", $followerURL, "apply", "--file", $manifestPath)
    Wait-AppPresence -Present $true

    foreach ($node in $nodes) {
        Invoke-Checked $cliBin @("--server", "http://$($node.HTTP)", "raft", "status")
    }
    Invoke-Checked $cliBin @("--server", $followerURL, "get", "apps")

    Write-Host ""
    Write-Host "Deleting app through follower..."
    Invoke-Checked $cliBin @("--server", $followerURL, "delete", "app", $appName, "-n", $namespace)
    Wait-AppPresence -Present $false
    Write-Host "Raft forwarding smoke completed."
}
finally {
    if (-not $KeepServer) {
        foreach ($started in $processes) {
            if ($null -ne $started.Process -and -not $started.Process.HasExited) {
                Stop-Process -Id $started.Process.Id -Force
                if (-not $started.Process.WaitForExit(5000)) {
                    Write-Warning "Timed out waiting for orch-server process $($started.ID) to stop"
                }
            }
        }
    }
    Write-Host "Logs: $logDir"
}
