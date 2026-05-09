[CmdletBinding()]
param(
    [string]$WorkDir = ".orch-resource-bench",
    [int]$SampleSeconds = 15,
    [int]$IntervalMilliseconds = 500,
    [int]$TimeoutSeconds = 120,
    [ValidateSet("process", "docker", "none")]
    [string]$ScheduleRuntime = "process",
    [switch]$SkipBuild,
    [switch]$KeepServers
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

function Assert-UnderRepo {
    param([Parameter(Mandatory = $true)][string]$Path)
    $full = [System.IO.Path]::GetFullPath($Path)
    $repoPrefix = $repoRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if (-not $full.StartsWith($repoPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to use path outside repository: $full"
    }
    return $full
}

function Test-IsWindows {
    return [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::Windows
    )
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

function Set-BenchEnvironment {
    param([Parameter(Mandatory = $true)][hashtable]$Vars)
    $previous = @{}
    foreach ($key in $Vars.Keys) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$Vars[$key], "Process")
    }
    return $previous
}

function Restore-BenchEnvironment {
    param([hashtable]$Previous)
    foreach ($key in $Previous.Keys) {
        [Environment]::SetEnvironmentVariable($key, $Previous[$key], "Process")
    }
}

function New-ProcSample {
    param(
        [Parameter(Mandatory = $true)][object[]]$Servers,
        [Parameter(Mandatory = $true)][int]$LogicalProcessors
    )
    $cpuSeconds = 0.0
    $workingSet = 0L
    $privateBytes = 0L
    $threads = 0
    $handles = 0
    $alive = 0

    foreach ($server in $Servers) {
        if ($server.Process.HasExited) {
            continue
        }
        $p = Get-Process -Id $server.Process.Id -ErrorAction SilentlyContinue
        if ($null -eq $p) {
            continue
        }
        $alive++
        if ($null -ne $p.CPU) {
            $cpuSeconds += [double]$p.CPU
        }
        $workingSet += [int64]$p.WorkingSet64
        $privateBytes += [int64]$p.PrivateMemorySize64
        $threads += [int]$p.Threads.Count
        $handles += [int]$p.HandleCount
    }

    return [pscustomobject]@{
        Time         = Get-Date
        Alive        = $alive
        CPUSeconds   = $cpuSeconds
        WorkingSet   = $workingSet
        PrivateBytes = $privateBytes
        Threads      = $threads
        Handles      = $handles
        HostCPUs     = $LogicalProcessors
    }
}

function Measure-ServerSet {
    param(
        [Parameter(Mandatory = $true)][string]$Label,
        [Parameter(Mandatory = $true)][object[]]$Servers,
        [int]$Seconds = $SampleSeconds,
        [System.Diagnostics.Process]$UntilProcess = $null,
        [int]$MinSeconds = 1
    )

    $logicalProcessors = [Environment]::ProcessorCount
    $samples = New-Object System.Collections.Generic.List[object]
    $start = Get-Date
    while ($true) {
        $samples.Add((New-ProcSample -Servers $Servers -LogicalProcessors $logicalProcessors))
        Start-Sleep -Milliseconds $IntervalMilliseconds
        $elapsed = ((Get-Date) - $start).TotalSeconds
        if ($null -ne $UntilProcess) {
            if ($UntilProcess.HasExited -and $elapsed -ge $MinSeconds) {
                break
            }
            if ($elapsed -ge $TimeoutSeconds) {
                throw "Timed out while measuring $Label"
            }
            continue
        }
        if ($elapsed -ge $Seconds) {
            break
        }
    }
    $samples.Add((New-ProcSample -Servers $Servers -LogicalProcessors $logicalProcessors))

    $first = $samples[0]
    $last = $samples[$samples.Count - 1]
    $duration = [math]::Max((($last.Time) - ($first.Time)).TotalSeconds, 0.001)
    $cpuCoreAvg = (($last.CPUSeconds - $first.CPUSeconds) / $duration)
    $cpuHostPctAvg = ($cpuCoreAvg / $logicalProcessors) * 100
    $wsValues = @($samples | ForEach-Object { [double]$_.WorkingSet / 1MB })
    $privValues = @($samples | ForEach-Object { [double]$_.PrivateBytes / 1MB })
    $threadValues = @($samples | ForEach-Object { [double]$_.Threads })
    $handleValues = @($samples | ForEach-Object { [double]$_.Handles })

    return [pscustomobject]@{
        label             = $Label
        processes         = @($Servers | ForEach-Object { [pscustomobject]@{ name = $_.Name; pid = $_.Process.Id; url = $_.URL } })
        samples           = $samples.Count
        durationSeconds   = [math]::Round($duration, 3)
        cpuCoresAvg       = [math]::Round($cpuCoreAvg, 4)
        cpuHostPercentAvg = [math]::Round($cpuHostPctAvg, 3)
        workingSetMiBAvg  = [math]::Round((($wsValues | Measure-Object -Average).Average), 2)
        workingSetMiBMax  = [math]::Round((($wsValues | Measure-Object -Maximum).Maximum), 2)
        privateMiBAvg     = [math]::Round((($privValues | Measure-Object -Average).Average), 2)
        privateMiBMax     = [math]::Round((($privValues | Measure-Object -Maximum).Maximum), 2)
        threadsAvg        = [math]::Round((($threadValues | Measure-Object -Average).Average), 1)
        handlesAvg        = [math]::Round((($handleValues | Measure-Object -Average).Average), 1)
    }
}

function Wait-OrchHealth {
    param([Parameter(Mandatory = $true)]$Server)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if ($Server.Process.HasExited) {
            throw "orch-server $($Server.Name) exited early with code $($Server.Process.ExitCode). See $($Server.Stdout) and $($Server.Stderr)"
        }
        & $cliBin --server $Server.URL health *> $null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for orch-server health at $($Server.URL)"
}

function Wait-RaftReady {
    param([Parameter(Mandatory = $true)][object[]]$Servers, [int]$ExpectedMembers)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $ready = $true
        foreach ($server in $Servers) {
            try {
                $status = (Invoke-CLIJson -ServerURL $server.URL -Arguments @("raft", "status", "--json"))[0]
                if (-not [bool]$status.ready -or [string]$status.leaderId -eq "" -or $status.members.Count -ne $ExpectedMembers) {
                    $ready = $false
                    break
                }
            }
            catch {
                $ready = $false
                break
            }
        }
        if ($ready) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for Raft readiness"
}

function Wait-WorkloadRunning {
    param(
        [Parameter(Mandatory = $true)][string]$ServerURL,
        [Parameter(Mandatory = $true)][string]$WorkloadName,
        [Parameter(Mandatory = $true)][string]$NodeID
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $items = Invoke-CLIJson -ServerURL $ServerURL -Arguments @("get", "workloads", "--json")
        $found = $items | Where-Object { $_.name -eq $WorkloadName -and $_.node -eq $NodeID -and $_.status -eq "running" } | Select-Object -First 1
        if ($null -ne $found) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for workload $WorkloadName to run on $NodeID"
}

function Start-OrchServer {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$DataDir,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][string]$URL
    )

    New-Item -ItemType Directory -Force $DataDir | Out-Null
    $stdout = Join-Path $logDir "$Name.out.log"
    $stderr = Join-Path $logDir "$Name.err.log"
    $previous = Set-BenchEnvironment @{ ORCH_DATA_DIR = $DataDir }
    try {
        $startArgs = @{
            FilePath               = $serverBin
            WorkingDirectory       = $repoRoot
            ArgumentList           = $Arguments
            RedirectStandardOutput = $stdout
            RedirectStandardError  = $stderr
            PassThru               = $true
        }
        if (Test-IsWindows) {
            $startArgs.WindowStyle = "Hidden"
        }
        $proc = Start-Process @startArgs
        return [pscustomobject]@{
            Name    = $Name
            Process = $proc
            URL     = $URL
            Stdout  = $stdout
            Stderr  = $stderr
        }
    }
    finally {
        Restore-BenchEnvironment $previous
    }
}

function Stop-Servers {
    param([object[]]$Servers)
    foreach ($server in $Servers) {
        if ($null -ne $server.Process -and -not $server.Process.HasExited) {
            Stop-Process -Id $server.Process.Id -Force
            if (-not $server.Process.WaitForExit(5000)) {
                Write-Warning "Timed out waiting for orch-server process $($server.Name) to stop"
            }
        }
    }
}

function New-ProcessManifest {
    param([Parameter(Mandatory = $true)][string]$Path)
    if (Test-IsWindows) {
        $command = "powershell.exe"
        $args = '["-NoProfile", "-Command", "Start-Sleep -Seconds 3600"]'
    } else {
        $command = "sh"
        $args = '["-c", "sleep 3600"]'
    }
    @"
apiVersion: warden.arcgolabs.io/v1alpha1
kind: App
metadata:
  name: resource-bench
  namespace: default
workloads:
  - name: sleeper
    kind: worker
    runtime: process
    run:
      exec:
        command: ["$command"]
        args: $args
    scheduling:
      preferredNodes:
        - bench-single
"@ | Set-Content -Path $Path -Encoding utf8
}

$workRoot = Assert-UnderRepo (Join-Path $repoRoot $WorkDir)
$binDir = Assert-UnderRepo (Join-Path $workRoot "bin")
$logDir = Assert-UnderRepo (Join-Path $workRoot "logs")
$dataRoot = Assert-UnderRepo (Join-Path $workRoot "data")
$resultDir = Assert-UnderRepo (Join-Path $workRoot "results")

if (-not $KeepServers) {
    foreach ($path in @($logDir, $dataRoot, $resultDir)) {
        if (Test-Path $path) {
            Remove-Item -LiteralPath $path -Recurse -Force
        }
    }
}
New-Item -ItemType Directory -Force $binDir, $logDir, $dataRoot, $resultDir | Out-Null

$binExt = ""
if (Test-IsWindows) {
    $binExt = ".exe"
}
$serverBin = Join-Path $binDir "orch-server$binExt"
$cliBin = Join-Path $binDir "orch$binExt"

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

$allServers = @()
$results = New-Object System.Collections.Generic.List[object]
$startedAt = Get-Date
$hostInfo = [pscustomobject]@{
    os                = [System.Runtime.InteropServices.RuntimeInformation]::OSDescription
    arch              = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    logicalProcessors = [Environment]::ProcessorCount
    sampleSeconds     = $SampleSeconds
    intervalMillis    = $IntervalMilliseconds
    scheduleRuntime   = $ScheduleRuntime
}

try {
    $singleData = Join-Path $dataRoot "single"
    $singleURL = "http://127.0.0.1:17601"
    $single = Start-OrchServer -Name "single" -DataDir $singleData -URL $singleURL -Arguments @(
        "--http-addr", "127.0.0.1:17601",
        "--raft-enabled=true",
        "--raft-node-id", "bench-single",
        "--raft-bind", "127.0.0.1:7461",
        "--raft-advertise", "127.0.0.1:7461",
        "--raft-peers", "bench-single=127.0.0.1:7461",
        "--raft-bootstrap=true",
        "--raft-data-dir", (Join-Path $singleData "dragonboat"),
        "--ingress-enabled=false",
        "--dns-enabled=true",
        "--dns-listen", "127.0.0.1:17653",
        "--observability-prometheus-enabled=true",
        "--observability-otlp-enabled=false",
        "--log-level", "info"
    )
    $allServers += $single
    Wait-OrchHealth $single
    Wait-RaftReady -Servers @($single) -ExpectedMembers 1
    $results.Add((Measure-ServerSet -Label "orch.single.idle" -Servers @($single)))

    if ($ScheduleRuntime -ne "none") {
        if ($ScheduleRuntime -eq "docker") {
            if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
                throw "Docker CLI was not found on PATH"
            }
            Invoke-Checked docker @("version")
            $manifestPath = (Resolve-Path (Join-Path $repoRoot "examples/local-docker-smoke.yaml")).Path
            $workloadName = "smoke"
            $appName = "smoke"
        } else {
            $manifestPath = Join-Path $workRoot "resource-bench-process.yaml"
            New-ProcessManifest -Path $manifestPath
            $workloadName = "sleeper"
            $appName = "resource-bench"
        }

        $applyStdout = Join-Path $logDir "schedule-apply.out.log"
        $applyStderr = Join-Path $logDir "schedule-apply.err.log"
        $applyProc = Start-Process -FilePath $cliBin -WorkingDirectory $repoRoot -ArgumentList @(
            "--server", $singleURL,
            "apply",
            "--file", $manifestPath,
            "--watch",
            "--timeout", "$($TimeoutSeconds)s"
        ) -RedirectStandardOutput $applyStdout -RedirectStandardError $applyStderr -PassThru
        $results.Add((Measure-ServerSet -Label "orch.single.schedule_apply.$ScheduleRuntime" -Servers @($single) -UntilProcess $applyProc -MinSeconds 1))
        if ($applyProc.ExitCode -ne 0) {
            throw "orch apply failed with code $($applyProc.ExitCode). See $applyStdout and $applyStderr"
        }
        Wait-WorkloadRunning -ServerURL $singleURL -WorkloadName $workloadName -NodeID "bench-single"
        $results.Add((Measure-ServerSet -Label "orch.single.scheduled_idle.$ScheduleRuntime" -Servers @($single)))

        try {
            Invoke-Checked $cliBin @("--server", $singleURL, "delete", "app", $appName, "-n", "default")
        }
        catch {
            Write-Warning $_
        }
    }

    Stop-Servers -Servers @($single)
    $allServers = @()

    $nodes = @(
        @{ Name = "node-a"; HTTP = "127.0.0.1:17611"; Raft = "127.0.0.1:7471"; DNS = "127.0.0.1:17661" },
        @{ Name = "node-b"; HTTP = "127.0.0.1:17612"; Raft = "127.0.0.1:7472"; DNS = "127.0.0.1:17662" },
        @{ Name = "node-c"; HTTP = "127.0.0.1:17613"; Raft = "127.0.0.1:7473"; DNS = "127.0.0.1:17663" }
    )
    $clusterNodes = ($nodes | ForEach-Object { "$($_.Name)=http://$($_.HTTP)" }) -join ","
    $raftPeers = ($nodes | ForEach-Object { "$($_.Name)=$($_.Raft)" }) -join ","
    $clusterServers = @()
    foreach ($node in $nodes) {
        $nodeData = Join-Path $dataRoot $node.Name
        $server = Start-OrchServer -Name $node.Name -DataDir $nodeData -URL "http://$($node.HTTP)" -Arguments @(
            "--http-addr", $node.HTTP,
            "--raft-enabled=true",
            "--raft-node-id", $node.Name,
            "--raft-bind", $node.Raft,
            "--raft-advertise", $node.Raft,
            "--raft-peers", $raftPeers,
            "--raft-bootstrap=true",
            "--raft-data-dir", (Join-Path $nodeData "dragonboat"),
            "--cluster-nodes", $clusterNodes,
            "--ingress-enabled=false",
            "--dns-enabled=true",
            "--dns-listen", $node.DNS,
            "--observability-prometheus-enabled=true",
            "--observability-otlp-enabled=false",
            "--log-level", "info"
        )
        $clusterServers += $server
        $allServers += $server
    }
    foreach ($server in $clusterServers) {
        Wait-OrchHealth $server
    }
    Wait-RaftReady -Servers $clusterServers -ExpectedMembers 3
    $results.Add((Measure-ServerSet -Label "orch.cluster3.idle.combined" -Servers $clusterServers))
    foreach ($server in $clusterServers) {
        $results.Add((Measure-ServerSet -Label "orch.cluster3.idle.$($server.Name)" -Servers @($server) -Seconds ([math]::Max(3, [math]::Min($SampleSeconds, 5)))))
    }
}
finally {
    if (-not $KeepServers) {
        Stop-Servers -Servers $allServers
    }
}

$payload = [pscustomobject]@{
    startedAt = $startedAt.ToString("o")
    finishedAt = (Get-Date).ToString("o")
    host = $hostInfo
    results = @($results.ToArray())
    logs = $logDir
}
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$jsonPath = Join-Path $resultDir "orch-resource-$timestamp.json"
$payload | ConvertTo-Json -Depth 8 | Set-Content -Path $jsonPath -Encoding utf8

Write-Host ""
Write-Host "Resource benchmark results:"
$results | Format-Table label, processes, cpuCoresAvg, cpuHostPercentAvg, workingSetMiBAvg, workingSetMiBMax, privateMiBAvg, threadsAvg -AutoSize
Write-Host ""
Write-Host "JSON: $jsonPath"
Write-Host "Logs: $logDir"
