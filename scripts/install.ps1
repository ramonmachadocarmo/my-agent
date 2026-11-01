<#
.SYNOPSIS
  Installs my-agent on Windows and sets up the "my-agent" command in
  PowerShell, pwsh, cmd and Git Bash.

.DESCRIPTION
  - Uses the my-agent.exe next to this script (release archive), or builds it
    from source when run inside the repository with Go installed, or uses
    bin\my-agent.exe.
  - Copies it to %LOCALAPPDATA%\Programs\my-agent (or -InstallDir).
  - Adds that folder to the user PATH (new terminals see it; no admin needed).
  - Adds tab completion to the PowerShell profiles and to ~/.bashrc (Git Bash).
  Run it again to update. -Uninstall removes everything it added.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts\install.ps1
.EXAMPLE
  .\install.ps1 -Uninstall
#>
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\my-agent'),
    [string]$Binary = '',
    [switch]$Uninstall,
    [switch]$Purge,          # with -Uninstall: also delete the configuration and sessions
    [switch]$NoShell,        # do not touch PATH and shell profiles
    [switch]$NoBuild,        # never build from source
    [switch]$Yes             # answer yes to every question
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2

# ---------------------------------------------------------------- messages --
$lang = $env:MY_AGENT_LANG
if (-not $lang) { $lang = (Get-UICulture).Name }
$LangIndex = 0
if ($lang -like 'pt*') { $LangIndex = 1 } elseif ($lang -like 'es*') { $LangIndex = 2 }
$Messages = @{
    building    = @('Building my-agent from source...', 'Compilando o my-agent a partir do código-fonte...', 'Compilando my-agent desde el código fuente...')
    nobinary    = @('my-agent.exe not found. Run this script from a release archive, or from the repository with Go installed (https://go.dev/dl).', 'my-agent.exe não encontrado. Rode este script a partir do pacote de release, ou no repositório com o Go instalado (https://go.dev/dl).', 'No se encontró my-agent.exe. Ejecuta este script desde el paquete de release, o en el repositorio con Go instalado (https://go.dev/dl).')
    copying     = @('Installing to {0}', 'Instalando em {0}', 'Instalando en {0}')
    inuse       = @('Could not replace {0}: close every running my-agent and try again.', 'Não foi possível substituir {0}: feche todo my-agent em execução e tente de novo.', 'No se pudo reemplazar {0}: cierra todo my-agent en ejecución e inténtalo de nuevo.')
    pathadded   = @('Added to the user PATH: {0}', 'Adicionado ao PATH do usuário: {0}', 'Añadido al PATH del usuario: {0}')
    pathok      = @('Already in the user PATH: {0}', 'Já está no PATH do usuário: {0}', 'Ya está en el PATH del usuario: {0}')
    pathremoved = @('Removed from the user PATH: {0}', 'Removido do PATH do usuário: {0}', 'Eliminado del PATH del usuario: {0}')
    profile     = @('Tab completion configured: {0}', 'Autocompletar configurado: {0}', 'Autocompletado configurado: {0}')
    unprofile   = @('Removed from: {0}', 'Removido de: {0}', 'Eliminado de: {0}')
    policy      = @('PowerShell does not run profile scripts on this machine (policy {0}), so tab completion would not load. Allow local scripts for your user (RemoteSigned)?', 'O PowerShell não executa scripts de perfil nesta máquina (política {0}), então o autocompletar não carregaria. Permitir scripts locais para o seu usuário (RemoteSigned)?', 'PowerShell no ejecuta scripts de perfil en este equipo (política {0}), así que el autocompletado no se cargaría. ¿Permitir scripts locales para tu usuario (RemoteSigned)?')
    policyskip  = @('Skipped: tab completion needs "Set-ExecutionPolicy -Scope CurrentUser RemoteSigned".', 'Ignorado: o autocompletar precisa de "Set-ExecutionPolicy -Scope CurrentUser RemoteSigned".', 'Omitido: el autocompletado necesita "Set-ExecutionPolicy -Scope CurrentUser RemoteSigned".')
    yesno       = @('[Y/n]', '[S/n]', '[S/n]')
    done        = @('my-agent {0} installed.', 'my-agent {0} instalado.', 'my-agent {0} instalado.')
    next        = @('Open a NEW terminal (PowerShell, cmd, Windows Terminal or Git Bash) and type:  my-agent', 'Abra um NOVO terminal (PowerShell, cmd, Windows Terminal ou Git Bash) e digite:  my-agent', 'Abre una NUEVA terminal (PowerShell, cmd, Windows Terminal o Git Bash) y escribe:  my-agent')
    vscode      = @('(VS Code and other apps that were already open need to be restarted to see the new PATH.)', '(O VS Code e outros apps que já estavam abertos precisam ser reiniciados para ver o novo PATH.)', '(VS Code y otras aplicaciones que ya estaban abiertas deben reiniciarse para ver el nuevo PATH.)')
    removed     = @('my-agent removed.', 'my-agent removido.', 'my-agent eliminado.')
    kept        = @('Your configuration and sessions were kept: {0}, {1} (use -Purge to delete them).', 'Sua configuração e sessões foram mantidas: {0}, {1} (use -Purge para apagá-las).', 'Se conservaron tu configuración y sesiones: {0}, {1} (usa -Purge para borrarlas).')
    purged      = @('Deleted: {0}', 'Apagado: {0}', 'Borrado: {0}')
}
function T([string]$key) {
    $s = $Messages[$key][$LangIndex]
    if ($args.Count -gt 0) { $s = $s -f $args }
    return $s
}
function Say([string]$text, [string]$color = 'Gray') { Write-Host $text -ForegroundColor $color }
function Ask([string]$question) {
    if ($Yes) { return $true }
    $a = Read-Host "$question $(T yesno)"
    return ($a -eq '' -or $a -match '^[ysYS]')
}

$exeName = 'my-agent.exe'
$exePath = Join-Path $InstallDir $exeName
$begin = '# >>> my-agent >>>'
$end = '# <<< my-agent <<<'

# ------------------------------------------------------ files with a block --
# Reads and writes text keeping the file's encoding (UTF-8 with or without
# BOM), so existing profiles are never garbled.
function Read-Text([string]$path) {
    # a new PowerShell profile gets a BOM (Windows PowerShell 5.1 needs it
    # for non-ASCII text); a new .bashrc must not have one
    if (-not (Test-Path -LiteralPath $path)) { return @{ Text = ''; Bom = $path.EndsWith('.ps1') } }
    $bytes = [IO.File]::ReadAllBytes($path)
    $bom = $bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF
    return @{ Text = [Text.Encoding]::UTF8.GetString($bytes).TrimStart([char]0xFEFF); Bom = $bom }
}
function Write-Text([string]$path, [string]$text, [bool]$bom) {
    $dir = Split-Path -Parent $path
    if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    [IO.File]::WriteAllText($path, $text, (New-Object Text.UTF8Encoding($bom)))
}
function Remove-Block([string]$text) {
    $pattern = '(?ms)\r?\n?' + [regex]::Escape($begin) + '.*?' + [regex]::Escape($end) + '[^\n]*\r?\n?'
    return [regex]::Replace($text, $pattern, "`n").TrimEnd()
}
# Set-Block puts content between the markers, replacing an older block.
function Set-Block([string]$path, [string]$content, [string]$nl = "`r`n") {
    $f = Read-Text $path
    $text = Remove-Block $f.Text
    $block = ($begin, $content, $end) -join $nl
    if ($text -ne '') { $text += $nl + $nl }
    Write-Text $path ($text + $block + $nl) $f.Bom
}
function Clear-Block([string]$path) {
    if (-not (Test-Path -LiteralPath $path)) { return $false }
    $f = Read-Text $path
    if ($f.Text -notmatch [regex]::Escape($begin)) { return $false }
    $text = Remove-Block $f.Text
    if ($text -ne '') { $text += "`r`n" }
    Write-Text $path $text $f.Bom
    return $true
}

# -------------------------------------------------------------- user PATH --
# The user PATH is a REG_EXPAND_SZ value; [Environment]::SetEnvironmentVariable
# would turn it into REG_SZ and break entries such as %USERPROFILE%\bin.
function Get-UserPath {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    $v = $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $key.Close()
    return [string]$v
}
function Set-UserPath([string]$value) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
    # keep the value's type; a new or %VAR%-using PATH must be REG_EXPAND_SZ
    $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
    if ($key.GetValueNames() -contains 'Path' -and $key.GetValueKind('Path') -eq 'String' -and $value -notmatch '%') {
        $kind = [Microsoft.Win32.RegistryValueKind]::String
    }
    $key.SetValue('Path', $value, $kind)
    $key.Close()
    # tell Explorer and new terminals that the environment changed
    if (-not ('MyAgentInstall.Native' -as [type])) {
        Add-Type -Namespace MyAgentInstall -Name Native -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    }
    $r = [UIntPtr]::Zero
    [void][MyAgentInstall.Native]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$r)
}
function Test-SameDir([string]$a, [string]$b) {
    $x = [Environment]::ExpandEnvironmentVariables($a).TrimEnd('\')
    return $x -ieq $b.TrimEnd('\')
}

# --------------------------------------------------------------- profiles --
$docs = [Environment]::GetFolderPath('MyDocuments')
$psProfiles = @(Join-Path $docs 'WindowsPowerShell\profile.ps1')     # Windows PowerShell 5.1
$pwshProfile = Join-Path $docs 'PowerShell\profile.ps1'              # PowerShell 7 (pwsh)
if ((Get-Command pwsh -ErrorAction SilentlyContinue) -or (Test-Path -LiteralPath (Split-Path $pwshProfile))) {
    $psProfiles += $pwshProfile
}
$bashrc = Join-Path $HOME '.bashrc'
$hasGitBash = [bool](Get-Command bash.exe -ErrorAction SilentlyContinue) -or (Test-Path -LiteralPath $bashrc)

# ============================================================== uninstall ==
if ($Uninstall) {
    if (Test-Path -LiteralPath $InstallDir) {
        Remove-Item -LiteralPath $InstallDir -Recurse -Force
    }
    $parts = @((Get-UserPath) -split ';' | Where-Object { $_ -ne '' })
    $keep = @($parts | Where-Object { -not (Test-SameDir $_ $InstallDir) })
    if ($keep.Count -ne $parts.Count) {
        Set-UserPath ($keep -join ';')
        Say (T pathremoved $InstallDir)
    }
    foreach ($p in $psProfiles + $bashrc) {
        if (Clear-Block $p) { Say (T unprofile $p) }
    }
    $cfgDir = Join-Path $env:APPDATA 'my-agent'
    $dataDir = Join-Path $env:LOCALAPPDATA 'my-agent'
    if ($Purge) {
        foreach ($d in $cfgDir, $dataDir) {
            if (Test-Path -LiteralPath $d) { Remove-Item -LiteralPath $d -Recurse -Force; Say (T purged $d) }
        }
    } else {
        Say (T kept $cfgDir $dataDir) 'DarkGray'
    }
    Say (T removed) 'Green'
    exit 0
}

# ================================================================ install ==
# 1. find (or build) the binary
$here = $PSScriptRoot
$root = Split-Path -Parent $here
$src = ''
if ($Binary) {
    $src = (Resolve-Path -LiteralPath $Binary).Path
} elseif (Test-Path -LiteralPath (Join-Path $here $exeName)) {
    $src = Join-Path $here $exeName                                   # release archive
} elseif (-not $NoBuild -and (Test-Path -LiteralPath (Join-Path $root 'go.mod')) -and (Get-Command go -ErrorAction SilentlyContinue)) {
    Say (T building) 'Cyan'
    $version = 'dev'
    # (PowerShell names are case-insensitive: avoid short names that may clash)
    $release = Select-String -Path (Join-Path $root 'CHANGELOG.md') -Pattern '^## \[(\d[^\]]*)\]' -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($release) { $version = $release.Matches[0].Groups[1].Value }
    $out = Join-Path $root "bin\$exeName"
    Push-Location $root
    try {
        & go build -trimpath -ldflags "-s -w -X my-agent/internal/cli.Version=$version" -o $out ./cmd/my-agent
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    } finally { Pop-Location }
    $src = $out
} elseif (Test-Path -LiteralPath (Join-Path $root "bin\$exeName")) {
    $src = Join-Path $root "bin\$exeName"
}
if (-not $src) { Say (T nobinary) 'Red'; exit 1 }

# 2. copy it
Say (T copying $InstallDir) 'Cyan'
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
if (-not ((Resolve-Path -LiteralPath $src).Path -ieq $exePath)) {
    try {
        Copy-Item -LiteralPath $src -Destination $exePath -Force
    } catch {
        # a running exe cannot be overwritten, but it can be renamed
        try {
            $old = "$exePath.old"
            if (Test-Path -LiteralPath $old) { Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue }
            Rename-Item -LiteralPath $exePath -NewName "$exeName.old" -Force
            Copy-Item -LiteralPath $src -Destination $exePath -Force
        } catch {
            Say (T inuse $exePath) 'Red'; exit 1
        }
    }
}
Unblock-File -LiteralPath $exePath -ErrorAction SilentlyContinue   # downloaded archives are marked as unsafe

if (-not $NoShell) {
    # 3. user PATH (also for cmd and Git Bash, which inherit it)
    $parts = @((Get-UserPath) -split ';' | Where-Object { $_ -ne '' })
    if (@($parts | Where-Object { Test-SameDir $_ $InstallDir }).Count -eq 0) {
        Set-UserPath ((@($parts) + $InstallDir) -join ';')
        Say (T pathadded $InstallDir) 'Green'
    } else {
        Say (T pathok $InstallDir) 'DarkGray'
    }
    if (-not (($env:Path -split ';') | Where-Object { Test-SameDir $_ $InstallDir })) {
        $env:Path = "$env:Path;$InstallDir"                            # this session too
    }

    # 4. tab completion in the PowerShell profiles
    $policy = Get-ExecutionPolicy
    $profilesRun = $policy -notin @('Restricted', 'AllSigned')
    if (-not $profilesRun) {
        if (Ask (T policy $policy)) {
            try {
                Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned -Force
                $profilesRun = $true
            } catch { Say $_.Exception.Message 'Yellow' }
        }
        if (-not $profilesRun) { Say (T policyskip) 'Yellow' }
    }
    if ($profilesRun) {
        $psBlock = 'if (Get-Command my-agent -ErrorAction SilentlyContinue) { my-agent completion powershell | Out-String | Invoke-Expression }'
        foreach ($p in $psProfiles) {
            Set-Block $p $psBlock
            Say (T profile $p) 'Green'
        }
    }

    # 5. Git Bash: it inherits PATH; add completion
    if ($hasGitBash) {
        $bashBlock = 'command -v my-agent >/dev/null 2>&1 && eval "$(my-agent completion bash | tr -d ''\r'')"'
        Set-Block $bashrc $bashBlock "`n"
        Say (T profile $bashrc) 'Green'
    }
}

# 6. check
$ver = (& $exePath version) -join ' '
Write-Host ''
Say (T done ($ver -replace '^my-agent\s*', '')) 'Green'
Say (T next) 'White'
Say (T vscode) 'DarkGray'
