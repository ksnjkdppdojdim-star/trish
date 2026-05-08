# helloWorld Trish Plugin

This directory is a minimal example of a Trish dynamic plugin. It is meant to be copied by plugin developers.

## Files

- `trish-plugin.json`: plugin manifest read by the Trish CLI during `plugin install`.
- `helloWorld.ps1`: PowerShell entry script executed on the remote Windows agent.

## Manifest

```json
{
  "name": "helloWorld",
  "version": "1.0.0",
  "entry": "helloWorld.ps1",
  "shell": "powershell",
  "os": ["windows"],
  "commands": [
    { "name": "hello" }
  ]
}
```

Important fields:

- `name`: unique plugin name stored on the Trish server.
- `version`: version shown by `trish plugin list`.
- `entry`: script file to execute.
- `shell`: currently `powershell`, `pwsh`, or `cmd`.
- `os`: target agent operating systems. Use `["windows"]` for Windows agents.
- `commands`: command names exposed in `trish info <agent>`.

## Install

From the admin machine:

```powershell
.\trish.exe plugin install M:\trish\trish\helloWorld
```

Then check:

```powershell
.\trish.exe plugin list
.\trish.exe info DESKTOP-0JSGE83
```

You should see `hello` in the command list.

## Run

Direct mode:

```powershell
.\trish.exe exec DESKTOP-0JSGE83 hello Codex
```

Shell mode:

```text
trish> use DESKTOP-0JSGE83
trish[DESKTOP-0JSGE83]> hello Codex
```

## Update

After editing this plugin:

```powershell
.\trish.exe plugin update M:\trish\trish\helloWorld
```

For Git-hosted plugins, the server remembers the original source, so later you can use:

```powershell
.\trish.exe plugin update all
```

## Developer Notes

The CLI packages every file in this folder, sends it to the server, and the server dispatches the package to the agent at execution time. The agent writes the files to a temporary directory, runs the entry script, then removes the temporary directory.

Keep plugin scripts deterministic and self-contained. If a plugin needs helper files, put them next to the manifest and reference them relative to `$PSScriptRoot` inside the script.

This example exposes one command for clarity. If a future plugin needs several commands, the cleanest pattern is one plugin directory per command until the runtime passes the invoked command name to the entry script.
