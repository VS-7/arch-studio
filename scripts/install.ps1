# Instalador do CLI do ArchCode Studio para Windows (PowerShell 5.1+ ou 7+).
#
#   irm https://raw.githubusercontent.com/VS-7/arch-studio/main/scripts/install.ps1 | iex
#
# Baixa o binário da última release do GitHub, confere o SHA-256 com o
# SHA256SUMS da release, instala em %LOCALAPPDATA%\Programs\ArchCode Studio
# (sem administrador) e coloca essa pasta no PATH do usuário.
# Rodar de novo atualiza para a versão mais recente.
#
# Opções (variáveis de ambiente, definidas antes de rodar):
#   $env:ARCHCODE_VERSION = '1.0.0'          instala uma versão específica
#   $env:ARCHCODE_INSTALL_DIR = 'C:\Tools'   outra pasta de instalação

# Tudo dentro de uma função: com `irm | iex`, um `exit` fecharia o terminal do usuário.
function Install-ArchCodeStudio {
    $ErrorActionPreference = 'Stop'
    # A barra de progresso deixa o Invoke-WebRequest muito lento no PowerShell 5.1.
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = 'VS-7/arch-studio'
    # Só há build x64; no Windows 11 ARM ele roda pela emulação do sistema.
    $asset = 'archcode-studio-windows-amd64.exe'
    if ($env:ARCHCODE_VERSION) {
        $version = 'v' + $env:ARCHCODE_VERSION.TrimStart('v')
        $base = "https://github.com/$repo/releases/download/$version"
    } else {
        $version = 'mais recente'
        $base = "https://github.com/$repo/releases/latest/download"
    }
    $dir = if ($env:ARCHCODE_INSTALL_DIR) { $env:ARCHCODE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ArchCode Studio' }

    Write-Host "ArchCode Studio - instalando o CLI (windows/amd64, versão $version)"
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('archcode-' + [IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    try {
        $exe = Join-Path $tmp $asset
        $sums = Join-Path $tmp 'SHA256SUMS'
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $exe
            Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $sums
        } catch {
            throw "não foi possível baixar de $base ($($_.Exception.Message))"
        }

        $line = Get-Content $sums | Where-Object { ($_ -split '\s+')[1] -in @($asset, "*$asset") } | Select-Object -First 1
        if (-not $line) { throw "$asset não consta no SHA256SUMS da release" }
        $expected = ($line -split '\s+')[0].ToLower()
        $actual = (Get-FileHash -Algorithm SHA256 -Path $exe).Hash.ToLower()
        if ($expected -ne $actual) { throw "o SHA-256 do download não confere (esperado $expected, obtido $actual)" }
        Write-Host '  Download conferido (SHA-256).'

        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $target = Join-Path $dir 'archcode-studio.exe'
        Move-Item -Force -Path $exe -Destination $target
        Write-Host "  Instalado em $target"
    } finally {
        Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
    }

    # PATH do usuário (vale para os próximos terminais) e da sessão atual.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { $userPath = '' }
    if (($userPath -split ';') -notcontains $dir) {
        $newPath = (($userPath.TrimEnd(';')), $dir | Where-Object { $_ }) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host '  PATH do usuário configurado.'
    }
    if (($env:Path -split ';') -notcontains $dir) { $env:Path = "$env:Path;$dir" }

    Write-Host ''
    & $target version
    Write-Host ''
    Write-Host 'Pronto! Próximos passos (neste terminal ou num novo):'
    Write-Host '  mkdir meu-projeto; cd meu-projeto'
    Write-Host '  archcode-studio init --name "Meu Projeto"     # cria .arch/, docs/ e api/'
    Write-Host '  archcode-studio serve                          # abre a interface em http://127.0.0.1:8765'
    Write-Host '  claude mcp add archcode-studio -- archcode-studio mcp --dir "$PWD"   # conecta o Claude Code'
}

Install-ArchCodeStudio
