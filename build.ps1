# Copyright (c) 2026 Jules MAHOUNOU
# Project  : TRISH
# Initiated: 17/04/2026
# Origin   : Benin
# Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
# License  : MIT — see LICENSE file for details

param(
    [string[]]$Targets = @("main", "cli", "server", "agent")
)

$ErrorActionPreference = "Stop"

$env:GOTELEMETRY = "off"
$env:GOCACHE = "M:\trish\.gocache"
$buildTmpRoot = "M:\trish\.gotmp"
$env:GOTMPDIR = Join-Path $buildTmpRoot ("build-" + [guid]::NewGuid().ToString("N"))
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:GOTMPDIR
$env:GOTELEMETRYDIR = "M:\trish\.tmp\gotelemetry"
$envFile = Join-Path $PSScriptRoot ".env"

New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
New-Item -ItemType Directory -Force -Path $buildTmpRoot | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOTELEMETRYDIR | Out-Null

function Read-DotEnv {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path $Path)) {
        return $values
    }

    foreach ($line in Get-Content $Path) {
        $trimmed = $line.Trim()
        if ([string]::IsNullOrWhiteSpace($trimmed)) { continue }
        if ($trimmed.StartsWith("#")) { continue }
        if ($trimmed.StartsWith("export ")) {
            $trimmed = $trimmed.Substring(7).Trim()
        }

        $parts = $trimmed.Split("=", 2)
        if ($parts.Count -ne 2) { continue }

        $key = $parts[0].Trim()
        $value = $parts[1].Trim()
        $value = $value.Trim("'")
        $value = $value.Trim('"')
        if ([string]::IsNullOrWhiteSpace($key)) { continue }
        $values[$key] = $value
    }

    return $values
}

function Get-ConfigValue {
    param(
        [hashtable]$Config,
        [string]$Key,
        [string]$DefaultValue
    )

    if ($Config.ContainsKey($Key) -and -not [string]::IsNullOrWhiteSpace($Config[$Key])) {
        return $Config[$Key]
    }
    return $DefaultValue
}

function Quote-LdValue {
    param([string]$Value)

    return $Value.Replace('\', '\\').Replace('"', '\"')
}

function Invoke-GoBuild {
    param(
        [string[]]$Arguments,
        [string]$Label
    )

    & go build @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed for $Label (exit code $LASTEXITCODE)"
    }
}

function Has-Target {
    param([string]$Name)

    return $Targets -contains $Name
}

$cfg = Read-DotEnv $envFile

$version = Get-ConfigValue $cfg "TRISH_VERSION" "1.23.00406"
$serverAddr = Get-ConfigValue $cfg "TRISH_SERVER_ADDR" "127.0.0.1"
$serverPort = Get-ConfigValue $cfg "TRISH_SERVER_PORT" "9999"
$agentListenPort = Get-ConfigValue $cfg "TRISH_AGENT_LISTEN_PORT" "2222"
$serverRegistryPath = Get-ConfigValue $cfg "TRISH_SERVER_REGISTRY_PATH" ""
$serverLockPath = Get-ConfigValue $cfg "TRISH_SERVER_LOCK_PATH" ""
$adminSecret = Get-ConfigValue $cfg "TRISH_ADMIN_SECRET" ""
$allowLoopbackServer = Get-ConfigValue $cfg "TRISH_ALLOW_LOOPBACK_SERVER" "false"
$outputDir = Get-ConfigValue $cfg "TRISH_BUILD_OUTPUT_DIR" "."
$mainName = Get-ConfigValue $cfg "TRISH_BUILD_MAIN_NAME" "trish.exe"
$cliName = Get-ConfigValue $cfg "TRISH_BUILD_CLI_NAME" "trish-cli.exe"
$serverName = Get-ConfigValue $cfg "TRISH_BUILD_SERVER_NAME" "trish-server.exe"
$agentName = Get-ConfigValue $cfg "TRISH_BUILD_AGENT_NAME" "trish-agent.exe"

$outputRoot = Join-Path $PSScriptRoot $outputDir
New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null

$ldflags = @(
    "-X `"trish/buildcfg.Version=$(Quote-LdValue $version)`"",
    "-X `"trish/buildcfg.DefaultServerAddr=$(Quote-LdValue $serverAddr)`"",
    "-X `"trish/buildcfg.DefaultServerPort=$(Quote-LdValue $serverPort)`"",
    "-X `"trish/buildcfg.DefaultAgentListenPort=$(Quote-LdValue $agentListenPort)`"",
    "-X `"trish/buildcfg.DefaultServerRegistry=$(Quote-LdValue $serverRegistryPath)`"",
    "-X `"trish/buildcfg.DefaultServerLock=$(Quote-LdValue $serverLockPath)`"",
    "-X `"trish/buildcfg.DefaultAdminSecret=$(Quote-LdValue $adminSecret)`"",
    "-X `"trish/buildcfg.AllowLoopbackServer=$(Quote-LdValue $allowLoopbackServer)`""
) -join " "

Write-Host "Building Trish $version..."
Write-Host "Embedded server: $serverAddr`:$serverPort"
Write-Host "Embedded agent listen port: $agentListenPort"
Write-Host "Server binary modes: run-foreground, install, repair, uninstall, run-service"

if (Has-Target "main") {
    Invoke-GoBuild -Arguments @("-ldflags", $ldflags, "-o", (Join-Path $outputRoot $mainName), ".") -Label $mainName
}
if (Has-Target "cli") {
    Invoke-GoBuild -Arguments @("-ldflags", $ldflags, "-o", (Join-Path $outputRoot $cliName), "./cmd/cli") -Label $cliName
}
if (Has-Target "server") {
    Invoke-GoBuild -Arguments @("-ldflags", $ldflags, "-o", (Join-Path $outputRoot $serverName), "./cmd/server") -Label $serverName
}
if (Has-Target "agent") {
    Invoke-GoBuild -Arguments @("-ldflags", $ldflags, "-o", (Join-Path $outputRoot $agentName), "./cmd/agent") -Label $agentName
}

Write-Host "Done."
