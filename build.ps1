$ErrorActionPreference = "Stop"

$env:GOTELEMETRY = "off"
$env:GOCACHE = "M:\trish\.gocache"
$envFile = Join-Path $PSScriptRoot ".env"

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

$cfg = Read-DotEnv $envFile

$version = Get-ConfigValue $cfg "TRISH_VERSION" "1.23.00406"
$serverAddr = Get-ConfigValue $cfg "TRISH_SERVER_ADDR" "127.0.0.1"
$serverPort = Get-ConfigValue $cfg "TRISH_SERVER_PORT" "9999"
$agentListenPort = Get-ConfigValue $cfg "TRISH_AGENT_LISTEN_PORT" "2222"
$serverRegistryPath = Get-ConfigValue $cfg "TRISH_SERVER_REGISTRY_PATH" ""
$serverLockPath = Get-ConfigValue $cfg "TRISH_SERVER_LOCK_PATH" ""
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
    "-X `"trish/buildcfg.AllowLoopbackServer=$(Quote-LdValue $allowLoopbackServer)`""
) -join " "

Write-Host "Building Trish $version..."
Write-Host "Embedded server: $serverAddr`:$serverPort"
Write-Host "Embedded agent listen port: $agentListenPort"

go build -ldflags $ldflags -o (Join-Path $outputRoot $mainName) .
go build -ldflags $ldflags -o (Join-Path $outputRoot $cliName) ./cmd/cli
go build -ldflags $ldflags -o (Join-Path $outputRoot $serverName) ./cmd/server
go build -ldflags $ldflags -o (Join-Path $outputRoot $agentName) ./cmd/agent

Write-Host "Done."
