[CmdletBinding()]
param(
    [string]$ServerAddr = "127.0.0.1:17443",
    [string]$Manifest = "examples/local-docker-smoke.yaml",
    [string]$WorkDir = ".orch-smoke",
    [string]$ContainerRuntime = "docker",
    [int]$TimeoutSeconds = 120,
    [switch]$KeepServer,
    [switch]$KeepContainer,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$serverURL = "http://$ServerAddr"
$nodeID = "smoke-node"
$workloadName = "smoke"
$assignmentKey = "default/smoke/smoke"
$containerName = "orch-default-smoke"
$binDir = Join-Path $repoRoot (Join-Path $WorkDir "bin")
$logDir = Join-Path $repoRoot (Join-Path $WorkDir "logs")
$serverStdout = Join-Path $logDir "orch-server.out.log"
$serverStderr = Join-Path $logDir "orch-server.err.log"
$dataDir = Join-Path $repoRoot (Join-Path $WorkDir "data")

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
$containerRuntime = $ContainerRuntime.ToLowerInvariant()
$allowedContainerRuntimes = @("docker", "podman")
if ($allowedContainerRuntimes -notcontains $containerRuntime) {
    throw "Unsupported container runtime '$ContainerRuntime'; expected one of $($allowedContainerRuntimes -join ', ')"
}

$containerRuntimeLabel = if ($containerRuntime -eq "podman") {
    "Podman"
} else {
    "Docker"
}

function Invoke-ContainerRuntime {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )
    & $containerRuntime @Arguments
}

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

function Wait-OrchHealth {
    param([System.Diagnostics.Process]$Process)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if ($Process.HasExited) {
            throw "orch-server exited early with code $($Process.ExitCode). See $serverStdout and $serverStderr"
        }
        & $cliBin --server $serverURL health *> $null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for orch-server health at $serverURL"
}

function Wait-OrchReady {
    param([System.Diagnostics.Process]$Process)
    if ($Process.HasExited) {
        throw "orch-server exited early with code $($Process.ExitCode). See $serverStdout and $serverStderr"
    }
    Invoke-Checked $cliBin @("--server", $serverURL, "ready", "--wait", "--timeout", "$($TimeoutSeconds)s")
}

function Wait-SmokeState {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $serverURL, "get", "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $serverURL, "get", "assignments", "--json")

        $workload = $workloads | Where-Object { $_.name -eq $workloadName -and $_.node -eq $nodeID -and $_.status -eq "running" } | Select-Object -First 1
        $assignment = $assignments | Where-Object { $_.key -eq $assignmentKey -and $_.node -eq $nodeID -and $_.status -eq "running" } | Select-Object -First 1

        if ($null -ne $workload -and $null -ne $assignment) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for workload and assignment to become running"
}

function Wait-SmokeStopped {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $serverURL, "get", "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $serverURL, "get", "assignments", "--json")

        $workload = $workloads | Where-Object { $_.name -eq $workloadName -and $_.node -eq $nodeID } | Select-Object -First 1
        $assignment = $assignments | Where-Object { $_.key -eq $assignmentKey -and $_.node -eq $nodeID -and $_.status -eq "stopped" } | Select-Object -First 1

        if ($null -eq $workload -and $null -ne $assignment) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for workload to be removed and assignment to become stopped"
}

function Test-SmokeContainerExists {
    $ids = Invoke-ContainerRuntime @("ps", "-a", "--filter", "name=^/$containerName$", "--format", "{{.ID}}")
    if ($LASTEXITCODE -ne 0) {
        throw "$containerRuntimeLabel container list check failed"
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
    throw "Timed out waiting for $containerRuntimeLabel container $containerName to be removed"
}

function Remove-SmokeContainer {
    if (Test-SmokeContainerExists) {
        Invoke-ContainerRuntime @("rm", "-f", $containerName) | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "$containerRuntimeLabel remove $containerName failed"
        }
    }
}

function Set-SmokeEnvironment {
    $vars = [ordered]@{
        ORCH_DATA_DIR                         = $dataDir
        ORCH_HTTP_ADDR                        = $ServerAddr
        ORCH_RAFT_NODE_ID                     = $nodeID
        ORCH_INGRESS_ENABLED                  = "false"
        ORCH_DNS_ENABLED                      = "false"
        ORCH_OBSERVABILITY_PROMETHEUS_ENABLED = "false"
        ORCH_OBSERVABILITY_OTLP_ENABLED       = "false"
        ORCH_LOG_LEVEL                        = "info"
    }
    $previous = @{}
    foreach ($key in $vars.Keys) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$vars[$key], "Process")
    }
    return $previous
}

function Restore-SmokeEnvironment {
    param([hashtable]$Previous)
    foreach ($key in $Previous.Keys) {
        [Environment]::SetEnvironmentVariable($key, $Previous[$key], "Process")
    }
}

if (-not (Get-Command $containerRuntime -ErrorAction SilentlyContinue)) {
    throw "$containerRuntimeLabel CLI was not found on PATH"
}
Invoke-Checked $containerRuntime @("version")

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
Remove-SmokeContainer

$startArgs = @{
    FilePath               = $serverBin
    WorkingDirectory       = $repoRoot
    RedirectStandardOutput = $serverStdout
    RedirectStandardError  = $serverStderr
    PassThru               = $true
}
if (Test-IsWindows) {
    $startArgs.WindowStyle = "Hidden"
}

$serverProcess = $null
$previousEnv = $null
try {
    $previousEnv = Set-SmokeEnvironment
    $serverProcess = Start-Process @startArgs
    Wait-OrchHealth $serverProcess
    Wait-OrchReady $serverProcess

    Invoke-Checked $cliBin @("--server", $serverURL, "apply", "--file", $manifestPath, "--watch", "--timeout", "$($TimeoutSeconds)s")
    Invoke-Checked $cliBin @("--server", $serverURL, "wait", "app", $workloadName, "-n", "default", "--for", "running", "--timeout", "$($TimeoutSeconds)s")
    Wait-SmokeState

    Write-Host ""
    Write-Host "Smoke deploy is running."
    Write-Host "Server:      $serverURL"
    Write-Host "Container:   $containerName"
    Write-Host "Workloads:   $cliBin --server $serverURL get workloads"
    Write-Host "Assignments: $cliBin --server $serverURL get assignments"
    Write-Host ""
    Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
    Invoke-Checked $cliBin @("--server", $serverURL, "describe", "app", $workloadName, "-n", "default")
    Invoke-Checked $cliBin @("--server", $serverURL, "describe", "node", $nodeID)
    Invoke-Checked $cliBin @("--server", $serverURL, "describe", "workload", $workloadName, "--app", $workloadName, "-n", "default")
    Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
    Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")
    Invoke-Checked $cliBin @("--server", $serverURL, "events")
    Invoke-Checked $cliBin @("--server", $serverURL, "logs", $workloadName, "--app", $workloadName, "-n", "default", "--tail", "20")

    if (-not $KeepContainer) {
        Write-Host ""
        Write-Host "Starting already-running smoke app..."
        Invoke-Checked $cliBin @("--server", $serverURL, "start", "app", $workloadName, "-n", "default")
        Invoke-Checked $cliBin @("--server", $serverURL, "wait", "app", $workloadName, "-n", "default", "--for", "running", "--timeout", "$($TimeoutSeconds)s")
        Wait-SmokeState
        Write-Host "Repeated smoke start completed."
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")

        Write-Host ""
        Write-Host "Stopping smoke app..."
        Invoke-Checked $cliBin @("--server", $serverURL, "stop", "app", $workloadName, "-n", "default")
        Invoke-Checked $cliBin @("--server", $serverURL, "wait", "app", $workloadName, "-n", "default", "--for", "stopped", "--timeout", "$($TimeoutSeconds)s")
        Wait-SmokeStopped
        Wait-SmokeContainerRemoved
        Write-Host "Smoke stop completed."
        Invoke-Checked $cliBin @("--server", $serverURL, "describe", "app", $workloadName, "-n", "default")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")

        Write-Host ""
        Write-Host "Starting smoke app after stop..."
        Invoke-Checked $cliBin @("--server", $serverURL, "start", "app", $workloadName, "-n", "default")
        Invoke-Checked $cliBin @("--server", $serverURL, "wait", "app", $workloadName, "-n", "default", "--for", "running", "--timeout", "$($TimeoutSeconds)s")
        Wait-SmokeState
        Write-Host "Smoke start completed."
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")

        Write-Host ""
        Write-Host "Restarting smoke app..."
        Invoke-Checked $cliBin @("--server", $serverURL, "restart", "app", $workloadName, "-n", "default")
        Invoke-Checked $cliBin @("--server", $serverURL, "wait", "app", $workloadName, "-n", "default", "--for", "running", "--timeout", "$($TimeoutSeconds)s")
        Wait-SmokeState
        Write-Host "Smoke restart completed."
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")

        Write-Host ""
        Write-Host "Deleting smoke app..."
        Invoke-Checked $cliBin @("--server", $serverURL, "delete", "app", $workloadName, "-n", "default")
        Wait-SmokeStopped
        Wait-SmokeContainerRemoved
        Write-Host "Smoke delete completed."
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")
    }
}
finally {
    if (-not $KeepServer -and $null -ne $serverProcess -and -not $serverProcess.HasExited) {
        Stop-Process -Id $serverProcess.Id -Force
        if (-not $serverProcess.WaitForExit(5000)) {
            Write-Warning "Timed out waiting for orch-server process to stop"
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
    if ($null -ne $previousEnv) {
        Restore-SmokeEnvironment $previousEnv
    }
    Write-Host "Server logs: $serverStdout / $serverStderr"
}
