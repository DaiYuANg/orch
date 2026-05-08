[CmdletBinding()]
param(
    [string]$ServerAddr = "127.0.0.1:17444",
    [string]$Manifest = "examples/local-docker-dns-smoke.yaml",
    [string]$WorkDir = ".orch-dns-smoke",
    [string]$DNSListen = "0.0.0.0:53",
    [string]$WorkloadNameserver = "",
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
$nodeID = "dns-smoke-node"
$appName = "dns-smoke"
$workloadNames = @("dns-backend", "dns-client")
$containerNames = @("orch-default-dns-backend", "orch-default-dns-client")
$clientContainer = "orch-default-dns-client"
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

function Test-IPv4 {
    param([string]$Value)
    return $Value -match '^\d{1,3}(\.\d{1,3}){3}$'
}

function Resolve-WorkloadNameserver {
    if (Test-IPv4 $WorkloadNameserver) {
        return $WorkloadNameserver
    }

    $raw = & docker run --rm busybox:1.36 sh -c "nslookup host.docker.internal 2>/dev/null | awk '/^Address: / { print `$2 }' | grep '^[0-9]' | tail -n 1"
    if ($LASTEXITCODE -eq 0) {
        $candidate = ($raw | Out-String).Trim()
        if (Test-IPv4 $candidate) {
            return $candidate
        }
    }

    $gateway = & docker network inspect bridge --format "{{range .IPAM.Config}}{{.Gateway}}{{end}}"
    if ($LASTEXITCODE -eq 0) {
        $candidate = ($gateway | Out-String).Trim()
        if (Test-IPv4 $candidate) {
            return $candidate
        }
    }

    throw "Unable to auto-detect a Docker workload nameserver. Pass -WorkloadNameserver <IPv4>."
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

function Wait-DNSWorkloadsRunning {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $serverURL, "get", "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $serverURL, "get", "assignments", "--json")

        $runningWorkloads = 0
        $runningAssignments = 0
        foreach ($name in $workloadNames) {
            $workload = $workloads | Where-Object { $_.name -eq $name -and $_.node -eq $nodeID -and $_.status -eq "running" } | Select-Object -First 1
            $assignment = $assignments | Where-Object { $_.key -eq "default/$appName/$name" -and $_.node -eq $nodeID -and $_.status -eq "running" } | Select-Object -First 1
            if ($null -ne $workload) {
                $runningWorkloads++
            }
            if ($null -ne $assignment) {
                $runningAssignments++
            }
        }

        if ($runningWorkloads -eq $workloadNames.Count -and $runningAssignments -eq $workloadNames.Count) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for DNS smoke workloads and assignments to become running"
}

function Wait-DNSProbe {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $logs = & docker logs $clientContainer 2>&1
        if (($logs | Out-String) -match "orch-dns-ok") {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for DNS probe success in $clientContainer logs"
}

function Wait-DNSAppStopped {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson @("--server", $serverURL, "get", "workloads", "--json")
        $assignments = Invoke-CLIJson @("--server", $serverURL, "get", "assignments", "--json")

        $presentWorkloads = 0
        $stoppedAssignments = 0
        foreach ($name in $workloadNames) {
            $workload = $workloads | Where-Object { $_.name -eq $name -and $_.node -eq $nodeID } | Select-Object -First 1
            $assignment = $assignments | Where-Object { $_.key -eq "default/$appName/$name" -and $_.node -eq $nodeID -and $_.status -eq "stopped" } | Select-Object -First 1
            if ($null -ne $workload) {
                $presentWorkloads++
            }
            if ($null -ne $assignment) {
                $stoppedAssignments++
            }
        }

        if ($presentWorkloads -eq 0 -and $stoppedAssignments -eq $workloadNames.Count) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for DNS smoke app to be deleted"
}

function Test-SmokeContainerExists {
    param([Parameter(Mandatory = $true)][string]$Name)
    $ids = & docker ps -a --filter "name=^/$Name$" --format "{{.ID}}"
    if ($LASTEXITCODE -ne 0) {
        throw "docker ps failed"
    }
    return (($ids | Out-String).Trim() -ne "")
}

function Remove-SmokeContainers {
    foreach ($name in $containerNames) {
        if (Test-SmokeContainerExists $name) {
            & docker rm -f $name | Out-Host
            if ($LASTEXITCODE -ne 0) {
                throw "docker rm -f $name failed"
            }
        }
    }
}

function Wait-SmokeContainersRemoved {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $remaining = 0
        foreach ($name in $containerNames) {
            if (Test-SmokeContainerExists $name) {
                $remaining++
            }
        }
        if ($remaining -eq 0) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for DNS smoke containers to be removed"
}

function Set-SmokeEnvironment {
    param([Parameter(Mandatory = $true)][string]$Nameserver)
    $vars = [ordered]@{
        ORCH_DATA_DIR                         = $dataDir
        ORCH_HTTP_ADDR                        = $ServerAddr
        ORCH_RAFT_ENABLED                     = "false"
        ORCH_RAFT_NODE_ID                     = $nodeID
        ORCH_INGRESS_ENABLED                  = "false"
        ORCH_DNS_ENABLED                      = "true"
        ORCH_DNS_LISTEN                       = $DNSListen
        ORCH_DNS_ZONE                         = "orch.local"
        ORCH_DNS_WORKLOAD_NAMESERVER          = $Nameserver
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

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker CLI was not found on PATH"
}
Invoke-Checked docker @("version")

$resolvedNameserver = Resolve-WorkloadNameserver

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
Remove-SmokeContainers

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
    $previousEnv = Set-SmokeEnvironment $resolvedNameserver
    $serverProcess = Start-Process @startArgs
    Wait-OrchHealth $serverProcess

    Write-Host "DNS listen:          $DNSListen"
    Write-Host "Workload nameserver: $resolvedNameserver"
    Invoke-Checked $cliBin @("--server", $serverURL, "apply", "--file", $manifestPath, "--watch", "--timeout", "$($TimeoutSeconds)s")
    Wait-DNSWorkloadsRunning
    Wait-DNSProbe

    Write-Host ""
    Write-Host "DNS smoke probe completed."
    Write-Host "FQDN:        dns-backend.default.svc.orch.local"
    Write-Host "Client logs: docker logs $clientContainer"
    Write-Host ""
    Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
    Invoke-Checked $cliBin @("--server", $serverURL, "describe", "app", $appName, "-n", "default")
    Invoke-Checked $cliBin @("--server", $serverURL, "get", "workloads")
    Invoke-Checked $cliBin @("--server", $serverURL, "get", "assignments")
    & docker logs $clientContainer | Out-Host

    if (-not $KeepContainer) {
        Write-Host ""
        Write-Host "Deleting DNS smoke app..."
        Invoke-Checked $cliBin @("--server", $serverURL, "delete", "app", $appName, "-n", "default")
        Wait-DNSAppStopped
        Wait-SmokeContainersRemoved
        Write-Host "DNS smoke delete completed."
        Invoke-Checked $cliBin @("--server", $serverURL, "get", "apps")
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
            Remove-SmokeContainers
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
