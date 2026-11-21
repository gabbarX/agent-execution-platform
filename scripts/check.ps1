# check.ps1 — the quality gate. Every commit must pass this.
# Usage: pwsh -File scripts/check.ps1
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

function Invoke-Step {
    param([string]$Name, [scriptblock]$Body)
    Write-Host "==> $Name" -ForegroundColor Cyan
    & $Body
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $Name" -ForegroundColor Red
        exit 1
    }
}

Invoke-Step 'gofmt' {
    $unformatted = gofmt -l .
    if ($LASTEXITCODE -ne 0) { return }
    if ($unformatted) {
        Write-Host "Files need gofmt:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "  $_" }
        # Signal failure to Invoke-Step via a native-style exit code.
        cmd /c exit 1
    } else {
        cmd /c exit 0
    }
}

Invoke-Step 'go vet' { go vet ./... }

Invoke-Step 'golangci-lint' {
    $lint = Get-Command golangci-lint -ErrorAction SilentlyContinue
    if (-not $lint) {
        $gopathBin = Join-Path (go env GOPATH) 'bin\golangci-lint.exe'
        if (Test-Path $gopathBin) {
            $lint = $gopathBin
        } else {
            Write-Host 'golangci-lint not found. Install it with:' -ForegroundColor Red
            Write-Host '  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest'
            cmd /c exit 1
            return
        }
    }
    & $lint run
}

Invoke-Step 'go test' { go test ./... }

Invoke-Step 'go build' { go build ./... }

Write-Host 'ALL CHECKS PASSED' -ForegroundColor Green
exit 0
