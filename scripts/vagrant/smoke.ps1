[CmdletBinding()]
param(
    [string]$Manifest = "examples/local-vagrant-smoke.yaml",
    [string]$WorkDir = ".orch-vagrant",
    [string]$AppName = "vagrant-smoke",
    [string]$WorkloadName = "smoke-worker",
    [int]$TimeoutSeconds = 180,
    [switch]$KeepNodes,
    [switch]$DestroyNodes,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $repoRoot
$vagrantHome = $env:VAGRANT_HOME
if ([string]::IsNullOrWhiteSpace($vagrantHome)) {
    $vagrantHome = Join-Path $repoRoot ".vagrant-home"
    New-Item -ItemType Directory -Force $vagrantHome | Out-Null
    $env:VAGRANT_HOME = $vagrantHome
}

$isWindowsHost = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)

if ([string]::IsNullOrWhiteSpace($env:VAGRANT_DEFAULT_PROVIDER)) {
    if ([string]::IsNullOrWhiteSpace($env:ORCH_VAGRANT_PROVIDER)) {
        if ($isWindowsHost) {
            $env:ORCH_VAGRANT_PROVIDER = "hyperv"
        } else {
            $env:ORCH_VAGRANT_PROVIDER = "virtualbox"
        }
    }
    $env:VAGRANT_DEFAULT_PROVIDER = $env:ORCH_VAGRANT_PROVIDER
}

Write-Host "Using Vagrant provider: $($env:VAGRANT_DEFAULT_PROVIDER)"

$nodes = @(
    @{ Name = "node1"; ID = "node1"; IP = "192.168.56.11"; HttpPort = 17443; RaftPort = 17451 },
    @{ Name = "node2"; ID = "node2"; IP = "192.168.56.12"; HttpPort = 17444; RaftPort = 17452 },
    @{ Name = "node3"; ID = "node3"; IP = "192.168.56.13"; HttpPort = 17445; RaftPort = 17453 }
)

$workDirPath = Join-Path $repoRoot $WorkDir
$artifactDir = Join-Path $workDirPath "dist"
$artifactBinDir = Join-Path $artifactDir "bin"

$hostCli = Join-Path $artifactBinDir ("orch" + $(if ($isWindowsHost) { ".exe" } else { "" }))
$linuxCli = Join-Path $artifactBinDir "orch"
$linuxServer = Join-Path $artifactBinDir "orch-server"
$manifestPath = (Resolve-Path (Join-Path $repoRoot $Manifest)).Path

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)] [string[]]$Arguments
    )
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed ($LASTEXITCODE): $FilePath $($Arguments -join ' ')"
    }
}

function Assert-Command {
    param([Parameter(Mandatory = $true)][string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' was not found on PATH"
    }
}

function Invoke-CLI {
    param(
        [Parameter(Mandatory = $true)][string]$ServerURL,
        [Parameter(Mandatory = $true)] [string[]]$Arguments
    )
    $args = @("--server", $ServerURL) + $Arguments
    $raw = & $hostCli @args
    if ($LASTEXITCODE -ne 0) {
        throw "orch CLI failed ($LASTEXITCODE): --server $ServerURL $($Arguments -join ' ')"
    }
    return $raw
}

function Invoke-CLIJson {
    param(
        [Parameter(Mandatory = $true)][string]$ServerURL,
        [Parameter(Mandatory = $true)] [string[]]$Arguments
    )
    $text = (Invoke-CLI -ServerURL $ServerURL -Arguments $Arguments | Out-String).Trim()
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

function Normalize-ApiResponse {
    param(
        [Parameter(Mandatory = $true)]$Payload
    )

    if ($Payload -is [array] -and $Payload.Count -gt 0) {
        $first = $Payload[0]
        if ($null -ne $first) {
            return $first
        }
    }
    if ($Payload -is [array]) {
        return $null
    }

    $body = Get-JSONProperty -Object $Payload -Name "body" -Default $null
    if ($null -ne $body) {
        if ($body -is [string] -and $body -ne "") {
            try {
                return ($body | ConvertFrom-Json)
            } catch {
                return $body
            }
        }
        return $body
    }
    return $Payload
}

function Build-Artifacts {
    New-Item -ItemType Directory -Force $artifactBinDir | Out-Null

    if (-not (Test-Path $manifestPath)) {
        throw "Manifest not found: $manifestPath"
    }

    if (-not $SkipBuild) {
        $oldGoos = $env:GOOS
        $oldGoarch = $env:GOARCH
        try {
            $env:GOOS = "linux"
            $env:GOARCH = "amd64"
            Invoke-Checked go @("build", "-o", $linuxServer, "./cmd/orch-server")
            Invoke-Checked go @("build", "-o", $linuxCli, "./cmd/orch-cli")
        }
        finally {
            if ($null -eq $oldGoos) {
                Remove-Item Env:GOOS -ErrorAction Ignore
            } else {
                $env:GOOS = $oldGoos
            }
            if ($null -eq $oldGoarch) {
                Remove-Item Env:GOARCH -ErrorAction Ignore
            } else {
                $env:GOARCH = $oldGoarch
            }
        }

        Invoke-Checked go @("build", "-o", $hostCli, "./cmd/orch-cli")
    }
}

function Assert-Binaries {
    if (-not (Test-Path $linuxServer)) {
        throw "Linux server binary not found: $linuxServer"
    }
    if (-not (Test-Path $linuxCli)) {
        throw "Linux CLI binary not found: $linuxCli"
    }
    if (-not (Test-Path $hostCli)) {
        throw "Host CLI binary not found: $hostCli"
    }
}

function Escape-BashArg {
    param([Parameter(Mandatory = $true)][string]$Value)
    $escaped = $Value -replace "'", "'\\''"
    return "'" + $escaped + "'"
}

function Invoke-Vagrant {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    & vagrant @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "vagrant failed ($LASTEXITCODE): vagrant $($Arguments -join ' ')"
    }
}

function Invoke-VagrantNodeCommand {
    param(
        [Parameter(Mandatory = $true)] [string]$NodeName,
        [Parameter(Mandatory = $true)] [string]$Command
    )
    Invoke-Vagrant @("ssh", $NodeName, "--command", $Command)
}

function Setup-Nodes {
    param([string]$ClusterNodes, [string]$RaftPeers)

    function Invoke-NodeWithRetry {
        param(
            [Parameter(Mandatory = $true)][string]$NodeName,
            [Parameter(Mandatory = $true)][string]$Command
        )

        $attempts = 0
        while ($attempts -lt 30) {
            try {
                Invoke-VagrantNodeCommand -NodeName $NodeName -Command $Command
                return
            }
            catch {
                $attempts++
                Start-Sleep -Milliseconds 1000
            }
        }
        throw "Timed out configuring $NodeName"
    }

    foreach ($node in $nodes) {
        $httpAddr = "{0}:{1}" -f $node.IP, $node.HttpPort
        $raftBind = "{0}:{1}" -f $node.IP, $node.RaftPort

        $args = @(
            (Escape-BashArg $node.ID),
            (Escape-BashArg $httpAddr),
            (Escape-BashArg $node.ID),
            (Escape-BashArg $raftBind),
            (Escape-BashArg $raftBind),
            (Escape-BashArg $RaftPeers),
            (Escape-BashArg $ClusterNodes)
        )
        $command = "sudo ORCH_INSTALL_SOURCE=$(Escape-BashArg \"/vagrant/$WorkDir/dist/bin\") /vagrant/scripts/vagrant/orch-node-setup.sh " + ($args -join " ")
        Invoke-NodeWithRetry -NodeName $node.Name -Command $command
        Write-Host "Configured $($node.Name)"
    }
}

function Wait-ClusterReady {
    param([int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        foreach ($node in $nodes) {
            $url = "http://{0}:{1}" -f $node.IP, $node.HttpPort
            try {
                $statusList = Invoke-CLIJson -ServerURL $url -Arguments @("raft", "status", "--json")
                if (($statusList -is [array]) -and $statusList.Count -eq 0) {
                    continue
                }
                $status = Normalize-ApiResponse -Payload $statusList
                if ($null -eq $status) {
                    continue
                }

                $ready = [bool](Get-JSONProperty -Object $status -Name "ready" -Default $false)
                $isLeader = [bool](Get-JSONProperty -Object $status -Name "isLeader" -Default $false)
                if ($ready -and $isLeader) {
                    return $node
                }
            }
            catch {
            }
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for Raft leader"
}

function Wait-AppReplicated {
    param(
        [string]$ServerURL,
        [int]$TimeoutSeconds,
        [string]$ExpectedStatus
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $apps = Invoke-CLIJson -ServerURL $ServerURL -Arguments @("get", "apps", "--json")
        $exists = $apps | Where-Object { $_.name -eq $AppName -and $_.namespace -eq "default" } | Select-Object -First 1
        if ($ExpectedStatus -eq "present") {
            if ($null -ne $exists) {
                return
            }
        } else {
            if ($null -eq $exists) {
                return
            }
        }
        Start-Sleep -Milliseconds 500
    }
    if ($ExpectedStatus -eq "present") {
        throw "Timed out waiting for app to become present"
    }
    throw "Timed out waiting for app to be removed"
}

function Wait-WorkloadRunning {
    param([string]$ServerURL, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $workloads = Invoke-CLIJson -ServerURL $ServerURL -Arguments @("get", "workloads", "--json")
        $assignments = Invoke-CLIJson -ServerURL $ServerURL -Arguments @("get", "assignments", "--json")
        $running = $workloads | Where-Object { $_.name -eq $WorkloadName -and $_.status -eq "running" } | Select-Object -First 1
        if ($null -eq $running) {
            Start-Sleep -Milliseconds 500
            continue
        }

        $assignmentKey = "default/$AppName/$WorkloadName"
        $runningAssignment = $assignments | Where-Object {
            (Get-JSONProperty -Object $_ -Name "key" -Default "") -eq $assignmentKey -and
            (Get-JSONProperty -Object $_ -Name "status" -Default "") -eq "running"
        } | Select-Object -First 1
        if ($null -ne $runningAssignment) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for workload $AppName/$WorkloadName running"
}

function Teardown-Nodes {
    if ($DestroyNodes) {
        Invoke-Vagrant @("destroy", "-f")
        return
    }

    if (-not $KeepNodes) {
        Invoke-Vagrant @("halt")
    }
}

try {
    Assert-Command go
    Assert-Command vagrant
    Build-Artifacts
    Assert-Binaries

    Invoke-Vagrant @("up")

    $clusterNodes = ($nodes | ForEach-Object { "{0}=http://{1}:{2}" -f $_.ID, $_.IP, $_.HttpPort }) -join ","
    $raftPeers = ($nodes | ForEach-Object { "{0}={1}:{2}" -f $_.ID, $_.IP, $_.RaftPort }) -join ","

    Setup-Nodes -ClusterNodes $clusterNodes -RaftPeers $raftPeers

    $leaderNode = Wait-ClusterReady -TimeoutSeconds $TimeoutSeconds
    $leaderURL = "http://{0}:{1}" -f $leaderNode.IP, $leaderNode.HttpPort
    Write-Host "Raft leader: $($leaderNode.ID)"

    Invoke-Checked $hostCli @(
        "--server", $leaderURL,
        "apply", "--file", $manifestPath, "--watch", "--timeout", "$($TimeoutSeconds)s"
    )
    Invoke-Checked $hostCli @(
        "--server", $leaderURL,
        "wait", "app", $AppName, "-n", "default", "--for", "running", "--timeout", "$($TimeoutSeconds)s"
    )
    Wait-WorkloadRunning -ServerURL $leaderURL -TimeoutSeconds $TimeoutSeconds

    Write-Host "Deployment visible in cluster"
    Write-Host "  Workloads:  $hostCli --server $leaderURL get workloads"
    Write-Host "  App:        $hostCli --server $leaderURL describe app $AppName -n default"
    Write-Host ""

    Invoke-Checked $hostCli @("--server", $leaderURL, "get", "apps")
    Invoke-Checked $hostCli @("--server", $leaderURL, "get", "workloads")
    Invoke-Checked $hostCli @("--server", $leaderURL, "get", "assignments")
    Invoke-Checked $hostCli @("--server", $leaderURL, "describe", "app", $AppName, "-n", "default")
    Invoke-Checked $hostCli @("--server", $leaderURL, "describe", "node", $leaderNode.ID)
    Invoke-Checked $hostCli @("--server", $leaderURL, "events")

    Invoke-Checked $hostCli @("--server", $leaderURL, "delete", "app", $AppName, "-n", "default")
    Wait-AppReplicated -ServerURL $leaderURL -TimeoutSeconds $TimeoutSeconds -ExpectedStatus "absent"

    Write-Host "Vagrant e2e smoke completed"
}
finally {
    Teardown-Nodes
    Write-Host "Workdir: $workDirPath"
}
