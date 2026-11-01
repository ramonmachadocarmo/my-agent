# Builds release archives for Windows, Linux and macOS into .\dist, each with
# the binary and its installer. Usage: .\scripts\package.ps1 [-Targets windows/amd64,linux/arm64]
param(
    [string[]]$Targets = @('windows/amd64', 'windows/arm64', 'linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64'),
    [switch]$SkipTests
)
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$version = 'dev'
$m = Select-String -Path CHANGELOG.md -Pattern '^## \[(\d[^\]]*)\]' | Select-Object -First 1
if ($m) { $version = $m.Matches[0].Groups[1].Value }

function Invoke-Native($cmd) { & $cmd[0] $cmd[1..($cmd.Length - 1)]; if ($LASTEXITCODE -ne 0) { throw "failed: $($cmd -join ' ')" } }

if (-not $SkipTests) {
    Invoke-Native @('go', 'vet', './...')
    Invoke-Native @('go', 'test', './...')
}

$dist = Join-Path (Get-Location) 'dist'
if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
New-Item -ItemType Directory $dist | Out-Null

$saved = @{ GOOS = $env:GOOS; GOARCH = $env:GOARCH; CGO_ENABLED = $env:CGO_ENABLED }
try {
    foreach ($t in $Targets) {
        $os, $arch = $t -split '/'
        $name = "my-agent-$version-$os-$arch"
        $dir = Join-Path $dist $name
        New-Item -ItemType Directory $dir | Out-Null
        $exe = if ($os -eq 'windows') { 'my-agent.exe' } else { 'my-agent' }

        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $os, $arch, '0'
        Invoke-Native @('go', 'build', '-trimpath', '-ldflags', "-s -w -X my-agent/internal/cli.Version=$version", '-o', (Join-Path $dir $exe), './cmd/my-agent')

        Copy-Item README.md, README.pt-BR.md, README.es.md, CHANGELOG.md $dir
        Copy-Item docs (Join-Path $dir 'docs') -Recurse
        if ($os -eq 'windows') {
            Copy-Item scripts\install.ps1, scripts\install.cmd $dir
            Compress-Archive -Path $dir -DestinationPath "$dir.zip"
        } else {
            Copy-Item scripts\install.sh $dir
            # tar.exe ships with Windows 10+; install.sh restores the execute bit
            Invoke-Native @('tar', '-czf', "$dir.tar.gz", '-C', $dist, $name)
        }
        Remove-Item $dir -Recurse -Force
        Write-Host "built $name"
    }
} finally {
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $saved.GOOS, $saved.GOARCH, $saved.CGO_ENABLED
}

# checksums, in the format of "sha256sum -c"
$sums = Get-ChildItem $dist -File | Sort-Object Name | ForEach-Object {
    "{0}  {1}" -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower(), $_.Name
}
[IO.File]::WriteAllLines((Join-Path $dist 'SHA256SUMS'), $sums)
Write-Host "archives in $dist"
