[CmdletBinding()]
param([string]$Version = 'dev')

$ErrorActionPreference = 'Stop'
if ($Version -cne 'dev' -and $Version -cnotmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
    throw 'Version must be dev or MAJOR.MINOR.PATCH'
}
Push-Location $PSScriptRoot
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    New-Item -ItemType Directory -Force dist | Out-Null
    go build -trimpath -ldflags="-s -w -H windowsgui -X main.version=$Version" -o dist/LiuguangTV.exe ./cmd/liuguangtv
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
    Write-Host 'Built: dist/LiuguangTV.exe'
} finally {
    Pop-Location
}
