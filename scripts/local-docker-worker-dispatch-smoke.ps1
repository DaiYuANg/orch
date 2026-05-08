[CmdletBinding()]
param(
    [string]$SchedulerAddr = "127.0.0.1:17445",
    [string]$WorkerAddr = "127.0.0.1:17446",
    [string]$Manifest = "examples/local-docker-worker-dispatch.yaml",
    [string]$WorkDir = ".orch-worker-dispatch-smoke",
    [int]$TimeoutSeconds = 120,
    [switch]$KeepServer,
    [switch]$KeepContainer,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$schedulerURL = "http://$SchedulerAddr"
$workerURL = "http://$WorkerAddr"
$schedulerNodeID = "scheduler-node"
$workerNodeID = "worker-node"
$appName = "dispatch-smoke"
$workloadName = "remote-worker"
$assignmentKey = "default/$appName/$workloadName"
$containerName = "orch-default-$workloadName"
$binDir = Join-Path $repoRoot (Join-Path $WorkDir "bin")
$logDir = Join-Path $repoRoot (Join-Path $WorkDir "logs")
$dataDir = Join-Path $repoRoot (Join-Path $WorkDir "data")
$schedulerStdout = Join-Path $logDir "scheduler.out.log"
$schedulerStderr = Join-Path $logDir "scheduler.err.log"
$workerStdout = Join-Path $logDir "worker.out.log"
$workerStderr = Join-Path $logDir "worker.err.log"

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
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $raw = & $cliBin @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "orch CLI failed ($LASTEXITCODE): $($Arguments -join ' ')"
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

function Wait-DispatchRunning {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $schedulerURL, "get", "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $schedulerURL, "get", "assignments", "--json")
        $apps = Invoke-CLIJson @("--server", $schedulerURL, "get", "apps", "--json")

        $workload = $workloads | Where-Object { $_.name -eq $workloadName -and $_.node -eq $workerNodeID -and $_.status -eq "running" } | Select-Object -First 1
        $assignment = $assignments | Where-Object { $_.key -eq $assignmentKey -and $_.node -eq $workerNodeID -and $_.status -eq "running" } | Select-Object -First 1
        $app = $apps | Where-Object { $_.name -eq $appName -and $_.namespace -eq "default" -and $_.status -eq "running" -and $_.observedGeneration } | Select-Object -First 1

        if ($null -ne $workload -and $null -ne $assignment -and $null -ne $app) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for remote worker dispatch to become running"
}

function Wait-DispatchStopped {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $schedulerURL, "get", "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $schedulerURL, "get", "assignments", "--json")

        $workload = $workloads | Where-Object { $_.name -eq $workloadName -and $_.node -eq $workerNodeID } | Select-Object -First 1
        $assignment = $assignments | Where-Object { $_.key -eq $assignmentKey -and $_.node -eq $workerNodeID -and $_.status -eq "stopped" } | Select-Object -First 1

        if ($null -eq $workload -and $null -ne $assignment) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for remote worker dispatch to stop"
}

function Wait-AppDeleted {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $apps = Invoke-CLIJson @("--server", $schedulerURL, "get", "apps", "--json")
        $app = $apps | Where-Object { $_.name -eq $appName -and $_.namespace -eq "default" } | Select-Object -First 1
        if ($null -eq $app) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for app deletion"
}

function Test-SmokeContainerExists {
    $ids = & docker ps -a --filter "name=^/$containerName$" --format "{{.ID}}"
    if ($LASTEXITCODE -ne 0) {
        throw "docker ps failed"
    }
    return (($ids | Out-String).Trim() -ne "")
}

function Wait-SmokeContainerRemoved {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (-not (Test-SmokeContainerExists)) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for Docker container $containerName to be removed"
}

function Remove-SmokeContainer {
    if (Test-SmokeContainerExists) {
        & docker rm -f $containerName | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "docker rm -f $containerName failed"
        }
    }
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

function Start-OrchServer {
    param(
        [Parameter(Mandatory = $true)][string]$NodeID,
        [Parameter(Mandatory = $true)][string]$Addr,
        [Parameter(Mandatory = $true)][string]$NodeDataDir,
        [Parameter(Mandatory = $true)][string]$Stdout,
        [Parameter(Mandatory = $true)][string]$Stderr,
        [string[]]$ExtraArgs = @()
    )

    $previous = Set-SmokeEnvironment @{
        ORCH_DATA_DIR                         = $NodeDataDir
        ORCH_HTTP_ADDR                        = $Addr
        ORCH_RAFT_ENABLED                     = "false"
        ORCH_RAFT_NODE_ID                     = $NodeID
        ORCH_INGRESS_ENABLED                  = "false"
        ORCH_DNS_ENABLED                      = "false"
        ORCH_OBSERVABILITY_PROMETHEUS_ENABLED = "false"
        ORCH_OBSERVABILITY_OTLP_ENABLED       = "false"
        ORCH_LOG_LEVEL                        = "info"
    }
    try {
        $args = @("--http-addr", $Addr, "--raft-enabled=false", "--raft-node-id", $NodeID, "--ingress-enabled=false", "--dns-enabled=false", "--observability-prometheus-enabled=false", "--observability-otlp-enabled=false", "--log-level", "info")
        $args += $ExtraArgs
        $startArgs = @{
            FilePath               = $serverBin
            WorkingDirectory       = $repoRoot
            ArgumentList           = $args
            RedirectStandardOutput = $Stdout
            RedirectStandardError  = $Stderr
            PassThru               = $true
        }
        if (Test-IsWindows) {
            $startArgs.WindowStyle = "Hidden"
        }
        return Start-Process @startArgs
    }
    finally {
        Restore-SmokeEnvironment $previous
    }
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker CLI was not found on PATH"
}
Invoke-Checked docker @("version")

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

$manifestPath = (Resolve-Path (Join-Path $repoRoot $Manifest)).Path
$schedulerConfigPath = Join-Path $dataDir "scheduler.config.yaml"
@"
cluster:
  nodes:
    ${workerNodeID}: ${workerURL}
"@ | Set-Content -NoNewline -Encoding utf8 $schedulerConfigPath
Remove-SmokeContainer

$schedulerProcess = $null
$workerProcess = $null
try {
    $workerProcess = Start-OrchServer `
        -NodeID $workerNodeID `
        -Addr $WorkerAddr `
        -NodeDataDir (Join-Path $dataDir "worker") `
        -Stdout $workerStdout `
        -Stderr $workerStderr
    Wait-OrchHealth -Process $workerProcess -URL $workerURL -Stdout $workerStdout -Stderr $workerStderr

    $schedulerProcess = Start-OrchServer `
        -NodeID $schedulerNodeID `
        -Addr $SchedulerAddr `
        -NodeDataDir (Join-Path $dataDir "scheduler") `
        -Stdout $schedulerStdout `
        -Stderr $schedulerStderr `
        -ExtraArgs @("--config", $schedulerConfigPath)
    Wait-OrchHealth -Process $schedulerProcess -URL $schedulerURL -Stdout $schedulerStdout -Stderr $schedulerStderr

    Invoke-Checked $cliBin @("--server", $schedulerURL, "apply", "--file", $manifestPath, "--watch", "--timeout", "$($TimeoutSeconds)s")
    Wait-DispatchRunning

    Write-Host ""
    Write-Host "Worker dispatch smoke is running."
    Write-Host "Scheduler: $schedulerURL"
    Write-Host "Worker:    $workerURL"
    Write-Host "Container: $containerName"
    Write-Host ""
    Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "apps")
    Invoke-Checked $cliBin @("--server", $schedulerURL, "describe", "app", $appName, "-n", "default")
    Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "workloads")
    Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "assignments")

    if (-not $KeepContainer) {
        Write-Host ""
        Write-Host "Stopping dispatched app..."
        Invoke-Checked $cliBin @("--server", $schedulerURL, "stop", "app", $appName, "-n", "default")
        Wait-DispatchStopped
        Wait-SmokeContainerRemoved
        Write-Host "Worker dispatch stop completed."
        Invoke-Checked $cliBin @("--server", $schedulerURL, "describe", "app", $appName, "-n", "default")

        Write-Host ""
        Write-Host "Starting dispatched app after stop..."
        Invoke-Checked $cliBin @("--server", $schedulerURL, "start", "app", $appName, "-n", "default")
        Wait-DispatchRunning
        Write-Host "Worker dispatch start completed."
        Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "apps")
        Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "assignments")

        Write-Host ""
        Write-Host "Deleting dispatched app..."
        Invoke-Checked $cliBin @("--server", $schedulerURL, "delete", "app", $appName, "-n", "default")
        Wait-DispatchStopped
        Wait-AppDeleted
        Wait-SmokeContainerRemoved
        Write-Host "Worker dispatch delete completed."
        Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "apps")
        Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $schedulerURL, "get", "assignments")
    }
}
finally {
    if (-not $KeepServer -and $null -ne $schedulerProcess -and -not $schedulerProcess.HasExited) {
        Stop-Process -Id $schedulerProcess.Id -Force
        if (-not $schedulerProcess.WaitForExit(5000)) {
            Write-Warning "Timed out waiting for scheduler orch-server process to stop"
        }
    }
    if (-not $KeepServer -and $null -ne $workerProcess -and -not $workerProcess.HasExited) {
        Stop-Process -Id $workerProcess.Id -Force
        if (-not $workerProcess.WaitForExit(5000)) {
            Write-Warning "Timed out waiting for worker orch-server process to stop"
        }
    }
    if (-not $KeepContainer) {
        try {
            Remove-SmokeContainer
        }
        catch {
            Write-Warning $_
        }
    }
    Write-Host "Scheduler logs: $schedulerStdout / $schedulerStderr"
    Write-Host "Worker logs:    $workerStdout / $workerStderr"
}
