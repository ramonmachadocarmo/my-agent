# Build my-agent into .\bin. Usage: .\scripts\build.ps1 [-Task test|build|all]
param([ValidateSet('test', 'build', 'all')][string]$Task = 'all')
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$version = 'dev'
$m = Select-String -Path CHANGELOG.md -Pattern '^## \[(\d[^\]]*)\]' | Select-Object -First 1
if ($m) { $version = $m.Matches[0].Groups[1].Value }
$exe = if ($IsLinux -or $IsMacOS) { 'my-agent' } else { 'my-agent.exe' }

function Invoke-Native($cmd) { & $cmd[0] $cmd[1..($cmd.Length - 1)]; if ($LASTEXITCODE -ne 0) { throw "failed: $($cmd -join ' ')" } }

if ($Task -in 'test', 'all') {
    Invoke-Native @('go', 'vet', './...')
    Invoke-Native @('go', 'test', './...')
}
if ($Task -in 'build', 'all') {
    New-Item -ItemType Directory -Force bin | Out-Null
    Invoke-Native @('go', 'build', '-trimpath', '-ldflags', "-s -w -X my-agent/internal/cli.Version=$version", '-o', "bin/$exe", './cmd/my-agent')
    Write-Host "built bin/$exe ($version)"
}
