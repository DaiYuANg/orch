[CmdletBinding()]
param(
    [string]$ServerAddr = "127.0.0.1:17443",
    [string]$Manifest = "examples/local-docker-smoke.yaml",
    [string]$WorkDir = ".orch-smoke",
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

New-Item -ItemType Directory -Force $binDir, $logDir | Out-Null

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

function Wait-SmokeState {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $serverURL, "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $serverURL, "assignments", "--json")

        $workload = $workloads | Where-Object { $_.name -eq $workloadName -and $_.node -eq $nodeID -and $_.status -eq "running" } | Select-Object -First 1
        $assignment = $assignments | Where-Object { $_.key -eq $assignmentKey -and $_.node -eq $nodeID -and $_.status -eq "running" } | Select-Object -First 1

        if ($null -ne $workload -and $null -ne $assignment) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for workload and assignment to become running"
}

function Remove-SmokeContainer {
    $ids = & docker ps -a --filter "name=^/$containerName$" --format "{{.ID}}"
    if ($LASTEXITCODE -ne 0) {
        throw "docker ps failed"
    }
    if (($ids | Out-String).Trim() -ne "") {
        & docker rm -f $containerName | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "docker rm -f $containerName failed"
        }
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
Remove-SmokeContainer

$serverArgs = @(
    "--http-addr", $ServerAddr,
    "--raft-enabled=false",
    "--raft-node-id", $nodeID,
    "--ingress-enabled=false",
    "--dns-enabled=false",
    "--observability-prometheus-enabled=false",
    "--observability-otlp-enabled=false",
    "--log-level", "info"
)

$startArgs = @{
    FilePath               = $serverBin
    ArgumentList           = $serverArgs
    WorkingDirectory       = $repoRoot
    RedirectStandardOutput = $serverStdout
    RedirectStandardError  = $serverStderr
    PassThru               = $true
}
if (Test-IsWindows) {
    $startArgs.WindowStyle = "Hidden"
}

$serverProcess = $null
try {
    $serverProcess = Start-Process @startArgs
    Wait-OrchHealth $serverProcess

    Invoke-Checked $cliBin @("--server", $serverURL, "apply", "--file", $manifestPath)
    Wait-SmokeState

    Write-Host ""
    Write-Host "Smoke deploy is running."
    Write-Host "Server:      $serverURL"
    Write-Host "Container:   $containerName"
    Write-Host "Workloads:   $cliBin --server $serverURL workloads"
    Write-Host "Assignments: $cliBin --server $serverURL assignments"
    Write-Host ""
    Invoke-Checked $cliBin @("--server", $serverURL, "workloads")
    Invoke-Checked $cliBin @("--server", $serverURL, "assignments")
}
finally {
    if (-not $KeepServer -and $null -ne $serverProcess -and -not $serverProcess.HasExited) {
        Stop-Process -Id $serverProcess.Id -Force
        try {
            Wait-Process -Id $serverProcess.Id -Timeout 5
        }
        catch {
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
    Write-Host "Server logs: $serverStdout / $serverStderr"
}
