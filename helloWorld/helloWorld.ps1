param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$PluginArgs
)

$ErrorActionPreference = "Stop"

$target = "Trish"
if ($PluginArgs.Count -gt 0 -and -not [string]::IsNullOrWhiteSpace($PluginArgs[0])) {
    $target = $PluginArgs[0]
}

Write-Output "Hello, $target!"
Write-Output "This output comes from the helloWorld dynamic plugin."
Write-Output "Remote machine: $env:COMPUTERNAME"
Write-Output "PowerShell: $($PSVersionTable.PSVersion)"
