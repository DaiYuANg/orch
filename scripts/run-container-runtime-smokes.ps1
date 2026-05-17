param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$CliArgs
)

$ErrorActionPreference = "Continue"

function Invoke-Smoke {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Label,

        [Parameter(Mandatory = $true)]
        [string[]]$Arguments
    )

    Write-Host "Running ${Label}..."
    & pwsh -NoProfile -ExecutionPolicy Bypass -File @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "${Label} failed"
    }
}

function Has-Command {
    param([Parameter(Mandatory = $true)][string]$Name)
    return (Get-Command $Name -ErrorAction SilentlyContinue) -ne $null
}

if (Has-Command docker) {
    Invoke-Smoke "Docker deploy smoke" (@("scripts/local-docker-smoke.ps1") + $CliArgs)
    Invoke-Smoke "Docker DNS smoke" (@("scripts/local-docker-dns-smoke.ps1") + $CliArgs)
    Invoke-Smoke "Docker worker dispatch smoke" (@("scripts/local-docker-worker-dispatch-smoke.ps1") + $CliArgs)
} else {
    Write-Host "skip docker runtime smoke: docker not found"
}

if (Has-Command podman) {
    Invoke-Smoke "Podman deploy smoke" @(
        "scripts/local-docker-smoke.ps1",
        "-ContainerRuntime", "podman",
        "-Manifest", "examples/local-podman-smoke.yaml"
    ) + $CliArgs

    Invoke-Smoke "Podman DNS smoke" @(
        "scripts/local-docker-dns-smoke.ps1",
        "-ContainerRuntime", "podman",
        "-Manifest", "examples/local-podman-dns-smoke.yaml"
    ) + $CliArgs

    Invoke-Smoke "Podman worker dispatch smoke" @(
        "scripts/local-docker-worker-dispatch-smoke.ps1",
        "-ContainerRuntime", "podman",
        "-Manifest", "examples/local-podman-worker-dispatch.yaml"
    ) + $CliArgs
} else {
    Write-Host "skip podman runtime smoke: podman not found"
}
