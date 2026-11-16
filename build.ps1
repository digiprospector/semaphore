$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$publicPlaceholder = Join-Path $repoRoot 'api\public\.gitkeep'
$previousCgoEnabled = $env:CGO_ENABLED
$previousBuildType = $env:VUE_APP_BUILD_TYPE
$placeholderBytes = $null
if (Test-Path -LiteralPath $publicPlaceholder) {
    $placeholderBytes = [System.IO.File]::ReadAllBytes($publicPlaceholder)
}

Push-Location $repoRoot
try {
    $env:VUE_APP_BUILD_TYPE = ''
    Push-Location (Join-Path $repoRoot 'web')
    try {
        if (-not (Test-Path -LiteralPath (Join-Path (Get-Location) 'node_modules\vite'))) {
            npm ci
            if ($LASTEXITCODE -ne 0) {
                throw "Frontend dependency installation failed with exit code $LASTEXITCODE"
            }
        }

        npm run build
        if ($LASTEXITCODE -ne 0) {
            throw "Frontend build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    $binDirectory = Join-Path $repoRoot 'bin'
    New-Item -ItemType Directory -Path $binDirectory -Force | Out-Null

    $env:CGO_ENABLED = '0'
    go build -tags netgo -o (Join-Path $binDirectory 'semaphore.exe') .\cli
    if ($LASTEXITCODE -ne 0) {
        throw "Backend build failed with exit code $LASTEXITCODE"
    }

    Write-Host "Build complete: $(Join-Path $binDirectory 'semaphore.exe')"
}
finally {
    if ($null -eq $previousBuildType) {
        Remove-Item Env:VUE_APP_BUILD_TYPE -ErrorAction SilentlyContinue
    }
    else {
        $env:VUE_APP_BUILD_TYPE = $previousBuildType
    }

    if ($null -eq $previousCgoEnabled) {
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    }
    else {
        $env:CGO_ENABLED = $previousCgoEnabled
    }

    if ($null -ne $placeholderBytes) {
        [System.IO.File]::WriteAllBytes($publicPlaceholder, $placeholderBytes)
    }
    elseif (-not (Test-Path -LiteralPath $publicPlaceholder)) {
        New-Item -ItemType File -Path $publicPlaceholder | Out-Null
    }
    Pop-Location
}
