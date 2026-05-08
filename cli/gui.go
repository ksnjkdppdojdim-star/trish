// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"trish/core"
)

type browseEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	Error string `json:"error,omitempty"`
}

type browseResponse struct {
	AgentID string        `json:"agent_id"`
	Path    string        `json:"path"`
	Parent  string        `json:"parent,omitempty"`
	Entries browseEntries `json:"entries"`
}

type browseEntries []browseEntry

func (e *browseEntries) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*e = nil
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var entries []browseEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
		*e = entries
		return nil
	}

	var entry browseEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	*e = []browseEntry{entry}
	return nil
}

type downloadInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type downloadChunk struct {
	Offset int64  `json:"offset"`
	Read   int    `json:"read"`
	Base64 string `json:"base64"`
}

func runGUI(client *core.Client, args []string) int {
	listenAddr := "127.0.0.1:8088"
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-listen="):
			listenAddr = strings.TrimPrefix(arg, "-listen=")
		case strings.HasPrefix(arg, "--listen="):
			listenAddr = strings.TrimPrefix(arg, "--listen=")
		}
	}
	if client.Timeout < 60*time.Second {
		client.Timeout = 60 * time.Second
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(guiHTML))
	})
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		guiLog("agents: listing")
		agents, err := client.ListAgents()
		if err != nil {
			guiLog("agents: error: %v", err)
			writeJSONError(w, err, http.StatusBadGateway)
			return
		}
		guiLog("agents: %d found", len(agents))
		writeJSON(w, agents)
	})
	mux.HandleFunc("/api/browse", func(w http.ResponseWriter, r *http.Request) {
		agentID := strings.TrimSpace(r.URL.Query().Get("agent"))
		targetPath := strings.TrimSpace(r.URL.Query().Get("path"))
		if agentID == "" {
			writeJSONError(w, fmt.Errorf("missing agent"), http.StatusBadRequest)
			return
		}
		if targetPath == "" {
			targetPath = `C:\`
		}

		guiLog("browse: agent=%s path=%q", agentID, targetPath)
		resp, err := browseRemotePath(client, agentID, targetPath)
		if err != nil {
			guiLog("browse: error: %v", err)
			writeJSONError(w, err, http.StatusBadGateway)
			return
		}
		guiLog("browse: ok path=%q entries=%d", resp.Path, len(resp.Entries))
		writeJSON(w, resp)
	})
	mux.HandleFunc("/api/download", func(w http.ResponseWriter, r *http.Request) {
		agentID := strings.TrimSpace(r.URL.Query().Get("agent"))
		targetPath := strings.TrimSpace(r.URL.Query().Get("path"))
		if agentID == "" || targetPath == "" {
			writeJSONError(w, fmt.Errorf("missing agent or path"), http.StatusBadRequest)
			return
		}

		guiLog("download: start agent=%s path=%q", agentID, targetPath)
		if err := streamRemoteFile(client, agentID, targetPath, w); err != nil {
			guiLog("download: error: %v", err)
			writeJSONError(w, err, http.StatusBadGateway)
			return
		}
	})

	fmt.Printf("GUI listening on http://%s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func browseRemotePath(client *core.Client, agentID, targetPath string) (*browseResponse, error) {
	script := fmt.Sprintf(`
$path = '%s'
if (-not (Test-Path -LiteralPath $path)) { throw "Path not found: $path" }
$item = Get-Item -LiteralPath $path
$entries = @()
if ($item.PSIsContainer) {
  $entries = @(Get-ChildItem -LiteralPath $path | Sort-Object PSIsContainer, Name | ForEach-Object {
    [PSCustomObject]@{
      name = $_.Name
      path = $_.FullName
      type = $(if ($_.PSIsContainer) { 'dir' } else { 'file' })
      size = $(if ($_.PSIsContainer) { 0 } else { [int64]$_.Length })
    }
  })
} else {
  $entries = @([PSCustomObject]@{
    name = $item.Name
    path = $item.FullName
    type = 'file'
    size = [int64]$item.Length
  })
}
[PSCustomObject]@{
  agent_id = '%s'
  path = $item.FullName
  parent = $(if ($item.Parent) { $item.Parent.FullName } else { '' })
  entries = @($entries)
} | ConvertTo-Json -Depth 4 -Compress
`, escapePowerShellSingleQuoted(targetPath), escapePowerShellSingleQuoted(agentID))

	result, err := client.ExecuteOnAgent(agentID, "superexec", []string{"powershell", script})
	if err != nil {
		return nil, err
	}

	var resp browseResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(result)), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse browse result: %w: %s", err, result)
	}
	return &resp, nil
}

func downloadRemoteFileInfo(client *core.Client, agentID, targetPath string) (*downloadInfo, error) {
	script := fmt.Sprintf(`
$path = '%s'
if (-not (Test-Path -LiteralPath $path)) { throw "Path not found: $path" }
$item = Get-Item -LiteralPath $path
if ($item.PSIsContainer) { throw "Path is a directory: $path" }
[PSCustomObject]@{
  name = $item.Name
  path = $item.FullName
  size = [int64]$item.Length
} | ConvertTo-Json -Compress
`, escapePowerShellSingleQuoted(targetPath))

	result, err := client.ExecuteOnAgent(agentID, "superexec", []string{"powershell", script})
	if err != nil {
		return nil, err
	}

	var resp downloadInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(result)), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse download info: %w: %s", err, result)
	}
	return &resp, nil
}

func downloadRemoteFileChunk(client *core.Client, agentID, targetPath string, offset int64, count int) (*downloadChunk, error) {
	script := fmt.Sprintf(`
$path = '%s'
$offset = [int64]%d
$count = [int]%d
if (-not (Test-Path -LiteralPath $path)) { throw "Path not found: $path" }
$item = Get-Item -LiteralPath $path
if ($item.PSIsContainer) { throw "Path is a directory: $path" }
$fs = [IO.File]::Open($item.FullName, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
try {
  [void]$fs.Seek($offset, [IO.SeekOrigin]::Begin)
  $buffer = New-Object byte[] $count
  $read = $fs.Read($buffer, 0, $count)
  if ($read -lt $count) {
    $actual = New-Object byte[] $read
    if ($read -gt 0) { [Array]::Copy($buffer, 0, $actual, 0, $read) }
    $buffer = $actual
  }
  [PSCustomObject]@{
    offset = $offset
    read = $read
    base64 = [Convert]::ToBase64String($buffer)
  } | ConvertTo-Json -Compress
} finally {
  $fs.Dispose()
}
`, escapePowerShellSingleQuoted(targetPath), offset, count)

	result, err := client.ExecuteOnAgent(agentID, "superexec", []string{"powershell", script})
	if err != nil {
		return nil, err
	}

	var resp downloadChunk
	if err := json.Unmarshal([]byte(strings.TrimSpace(result)), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse download chunk: %w: %s", err, result)
	}
	return &resp, nil
}

func streamRemoteFile(client *core.Client, agentID, targetPath string, w http.ResponseWriter) error {
	info, err := downloadRemoteFileInfo(client, agentID, targetPath)
	if err != nil {
		return err
	}

	const chunkSize = 256 * 1024
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(info.Name))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, _ := w.(http.Flusher)
	if info.Size == 0 {
		guiLog("download: complete name=%q size=0", info.Name)
		return nil
	}

	var offset int64
	for offset < info.Size {
		remaining := info.Size - offset
		count := chunkSize
		if remaining < int64(count) {
			count = int(remaining)
		}

		chunk, err := downloadRemoteFileChunk(client, agentID, targetPath, offset, count)
		if err != nil {
			return err
		}
		if chunk.Offset != offset {
			return fmt.Errorf("unexpected chunk offset: got %d, want %d", chunk.Offset, offset)
		}
		if chunk.Read <= 0 {
			return fmt.Errorf("download stopped at byte %d of %d", offset, info.Size)
		}

		data, err := base64.StdEncoding.DecodeString(chunk.Base64)
		if err != nil {
			return fmt.Errorf("failed to decode download chunk at byte %d: %w", offset, err)
		}
		if len(data) != chunk.Read {
			return fmt.Errorf("bad chunk size at byte %d: got %d, want %d", offset, len(data), chunk.Read)
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}

		offset += int64(chunk.Read)
		guiLog("download: %q %d/%d bytes", info.Name, offset, info.Size)
	}

	guiLog("download: complete name=%q size=%d", info.Name, info.Size)
	return nil
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func contentDisposition(name string) string {
	clean := strings.ReplaceAll(name, `"`, "'")
	if strings.TrimSpace(clean) == "" {
		clean = "download.bin"
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, clean, url.PathEscape(clean))
}

func guiLog(format string, args ...interface{}) {
	fmt.Printf("[%s] gui: %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

var guiHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Trish GUI</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f3f6fb;
      --panel: #ffffff;
      --text: #15202b;
      --muted: #5f6b7a;
      --line: #d9e1ea;
      --accent: #0f766e;
      --accent2: #164e63;
      --accent-hover: #0d9488;
      --row-hover: #f0f7ff;
      --shadow: 0 2px 8px rgba(0,0,0,.08);
    }
    [data-theme="dark"] {
      --bg: #0d1117;
      --panel: #161b22;
      --text: #e6edf3;
      --muted: #8b949e;
      --line: #30363d;
      --accent: #2dd4bf;
      --accent2: #38bdf8;
      --accent-hover: #5eead4;
      --row-hover: #1c2128;
      --shadow: 0 2px 8px rgba(0,0,0,.4);
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: 'Segoe UI', system-ui, sans-serif; background: var(--bg); color: var(--text); transition: background .2s, color .2s; }
    .app { display: grid; grid-template-columns: 300px 1fr; min-height: 100vh; }

    /* Sidebar */
    aside {
      border-right: 1px solid var(--line);
      background: var(--panel);
      display: flex;
      flex-direction: column;
      padding: 0;
      position: sticky;
      top: 0;
      height: 100vh;
      overflow: hidden;
    }
    .sidebar-header {
      padding: 18px 16px 14px;
      border-bottom: 1px solid var(--line);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
    }
    .sidebar-header h1 { font-size: 16px; font-weight: 700; letter-spacing: .3px; }
    .sidebar-scroll { flex: 1; overflow-y: auto; padding: 12px; }
    .sidebar-scroll::-webkit-scrollbar { width: 4px; }
    .sidebar-scroll::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
    .section-label { font-size: 10px; font-weight: 600; letter-spacing: 1px; text-transform: uppercase; color: var(--muted); margin: 0 0 8px 4px; }

    /* Theme toggle */
    #themeBtn {
      background: none;
      border: 1px solid var(--line);
      color: var(--text);
      padding: 6px 10px;
      border-radius: 6px;
      cursor: pointer;
      font-size: 15px;
      line-height: 1;
      flex-shrink: 0;
    }
    #themeBtn:hover { background: var(--row-hover); }

    /* Agent cards */
    .agent {
      width: 100%;
      text-align: left;
      border: 1px solid var(--line);
      background: transparent;
      padding: 10px 12px;
      margin-bottom: 6px;
      border-radius: 8px;
      cursor: pointer;
      color: var(--text);
      transition: border-color .15s, background .15s, box-shadow .15s;
    }
    .agent:hover { background: var(--row-hover); }
    .agent.active {
      border-color: var(--accent);
      background: var(--row-hover);
      box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 18%, transparent);
    }
    .agent-id { font-weight: 600; font-size: 13px; }
    .agent-meta { font-size: 11px; color: var(--muted); margin-top: 2px; display: flex; align-items: center; gap: 6px; }
    .dot { width: 6px; height: 6px; border-radius: 50%; background: #22c55e; display: inline-block; flex-shrink: 0; }
    .dot.offline { background: #ef4444; }

    /* Main */
    main { display: flex; flex-direction: column; padding: 20px 24px; gap: 14px; min-width: 0; }

    /* Toolbar */
    .toolbar {
      display: flex;
      gap: 8px;
      align-items: center;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 10px;
      padding: 10px 12px;
      box-shadow: var(--shadow);
    }
    .toolbar input {
      flex: 1;
      border: none;
      outline: none;
      background: transparent;
      font: inherit;
      font-size: 13px;
      color: var(--text);
      font-family: 'Consolas', monospace;
      min-width: 0;
    }
    .toolbar input::placeholder { color: var(--muted); }
    .divider { width: 1px; height: 20px; background: var(--line); flex-shrink: 0; }
    .tb-btn {
      border: none;
      background: none;
      color: var(--muted);
      cursor: pointer;
      padding: 4px 8px;
      border-radius: 6px;
      font-size: 16px;
      line-height: 1;
      transition: background .15s, color .15s;
      flex-shrink: 0;
    }
    .tb-btn:hover { background: var(--row-hover); color: var(--text); }
    .tb-btn.go { background: var(--accent); color: #fff; font-size: 13px; font-weight: 600; padding: 5px 14px; font-family: inherit; }
    .tb-btn.go:hover { background: var(--accent-hover); }

    /* Breadcrumb */
    .breadcrumb { font-size: 12px; color: var(--muted); font-family: 'Consolas', monospace; padding: 0 2px; display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
    .breadcrumb span { color: var(--text); }
    .breadcrumb .sep { color: var(--line); }

    /* Panel + table */
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 10px;
      overflow: hidden;
      box-shadow: var(--shadow);
      flex: 1;
    }
    table { width: 100%; border-collapse: collapse; }
    thead { position: sticky; top: 0; background: var(--panel); z-index: 1; }
    th {
      padding: 10px 14px;
      border-bottom: 1px solid var(--line);
      font-size: 11px;
      font-weight: 600;
      letter-spacing: .6px;
      text-transform: uppercase;
      color: var(--muted);
      text-align: left;
    }
    td { padding: 9px 14px; border-bottom: 1px solid var(--line); font-size: 13px; }
    tr:last-child td { border-bottom: none; }
    tbody tr { transition: background .1s; }
    tbody tr:hover { background: var(--row-hover); }

    /* File icons */
    .icon { font-style: normal; margin-right: 6px; }
    .name-cell { display: flex; align-items: center; }

    /* Type badge */
    .badge {
      display: inline-block;
      font-size: 10px;
      font-weight: 600;
      letter-spacing: .5px;
      padding: 2px 7px;
      border-radius: 99px;
      text-transform: uppercase;
    }
    .badge-dir { background: color-mix(in srgb, var(--accent) 14%, transparent); color: var(--accent); }
    .badge-file { background: color-mix(in srgb, var(--accent2) 12%, transparent); color: var(--accent2); }

    /* Size */
    .size-val { font-family: 'Consolas', monospace; color: var(--muted); font-size: 12px; }

    /* Action */
    .action-btn {
      border: 1px solid var(--line);
      background: none;
      color: var(--accent);
      cursor: pointer;
      padding: 4px 10px;
      border-radius: 6px;
      font-size: 12px;
      font-weight: 600;
      font-family: inherit;
      transition: background .15s, border-color .15s;
    }
    .action-btn:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); border-color: var(--accent); }
    .action-btn.dl { color: var(--accent2); }
    .action-btn.dl:hover { background: color-mix(in srgb, var(--accent2) 10%, transparent); border-color: var(--accent2); }

    /* Empty state */
    .empty { padding: 40px; text-align: center; color: var(--muted); font-size: 14px; }

    /* Status bar */
    .statusbar {
      font-size: 11px;
      color: var(--muted);
      padding: 0 2px;
      display: flex;
      justify-content: space-between;
    }
  </style>
</head>
<body>
  <div class="app">
    <aside>
      <div class="sidebar-header">
        <h1>🖧 Trish GUI</h1>
        <button id="themeBtn" title="Toggle theme">🌙</button>
      </div>
      <div class="sidebar-scroll">
        <div class="section-label">Agents</div>
        <div id="agents"></div>
      </div>
    </aside>
    <main>
      <div class="toolbar">
        <span style="font-size:15px;color:var(--muted)" title="Go up">📁</span>
        <div class="divider"></div>
        <input id="path" value="C:\" placeholder="Entrer un chemin…" />
        <div class="divider"></div>
        <button class="tb-btn" id="upBtn" title="Parent">↑</button>
        <button class="tb-btn go" id="browseBtn">Go</button>
      </div>
      <div class="breadcrumb" id="breadcrumb"></div>
      <div class="panel">
        <table>
          <thead>
            <tr><th>Nom</th><th>Type</th><th>Taille</th><th>Action</th></tr>
          </thead>
          <tbody id="entries"></tbody>
        </table>
      </div>
      <div class="statusbar">
        <span id="statusLeft">—</span>
        <span id="statusRight"></span>
      </div>
    </main>
  </div>
  <script>
    // Theme
    const root = document.documentElement;
    const themeBtn = document.getElementById('themeBtn');
    let dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    function applyTheme() {
      root.setAttribute('data-theme', dark ? 'dark' : 'light');
      themeBtn.textContent = dark ? '☀️' : '🌙';
    }
    applyTheme();
    themeBtn.onclick = () => { dark = !dark; applyTheme(); };

    // State
    let activeAgent = "";
    let currentParent = "";

    function formatSize(bytes) {
      if (bytes === 0) return '—';
      if (bytes < 1024) return bytes + ' B';
      if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
      return (bytes / 1048576).toFixed(1) + ' MB';
    }

    function updateBreadcrumb(agentId, path) {
      const el = document.getElementById('breadcrumb');
      const parts = path.replace(/\\/g, '/').split('/').filter(Boolean);
      let html = '<span>' + agentId + '</span><span class="sep">::</span>';
      let built = '';
      parts.forEach((p, i) => {
        built += (i === 0 ? '' : '\\') + (i === 0 ? p + '\\' : p);
        const cap = built;
        html += '<span class="sep">›</span><span style="cursor:pointer;text-decoration:underline" data-nav="' + cap + '">' + p + '</span>';
      });
      el.innerHTML = html;
      el.querySelectorAll('[data-nav]').forEach(s => s.onclick = () => browse(s.dataset.nav));
    }

    async function loadAgents() {
      const res = await fetch('/api/agents');
      const agents = await res.json();
      const root = document.getElementById('agents');
      root.innerHTML = '';
      agents.forEach(agent => {
        const btn = document.createElement('button');
        btn.className = 'agent' + (agent.id === activeAgent ? ' active' : '');
        const online = agent.status === 'online' || !agent.status;
        btn.innerHTML =
          '<div class="agent-id">' + agent.id + '</div>' +
          '<div class="agent-meta">' +
            '<span class="dot ' + (online ? '' : 'offline') + '"></span>' +
            agent.ip_address + ':' + agent.port +
            ' &nbsp;&middot;&nbsp; ' + (agent.status || 'unknown') +
          '</div>';
        btn.onclick = () => {
          activeAgent = agent.id;
          loadAgents();
          browse();
        };
        root.appendChild(btn);
      });
      if (!activeAgent && agents.length) {
        activeAgent = agents[0].id;
        loadAgents();
        browse();
      }
    }

    async function browse(pathOverride) {
      if (!activeAgent) return;
      const path = pathOverride || document.getElementById('path').value;
      document.getElementById('statusLeft').textContent = 'Chargement...';
      const res = await fetch('/api/browse?agent=' + encodeURIComponent(activeAgent) + '&path=' + encodeURIComponent(path));
      const data = await res.json();
      if (data.error) { alert(data.error); document.getElementById('statusLeft').textContent = 'Erreur'; return; }
      document.getElementById('path').value = data.path;
      updateBreadcrumb(activeAgent, data.path);
      currentParent = data.parent || '';
      const tbody = document.getElementById('entries');
      tbody.innerHTML = '';
      if (!data.entries || data.entries.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="empty">Dossier vide</td></tr>';
        document.getElementById('statusLeft').textContent = '0 éléments';
        return;
      }
      let dirs = 0, files = 0;
      data.entries.forEach(entry => {
        const tr = document.createElement('tr');
        const isDir = entry.type === 'dir';
        if (isDir) dirs++; else files++;
        const ico = isDir ? '📁' : getIcon(entry.name);
        const badge = isDir
          ? '<span class="badge badge-dir">Dossier</span>'
          : '<span class="badge badge-file">Fichier</span>';
        const action = isDir
          ? '<button class="action-btn" data-open="' + entry.path + '">Ouvrir</button>'
          : '<button class="action-btn dl" data-download="' + entry.path + '">&darr; Telecharger</button>';
        tr.innerHTML =
          '<td><div class="name-cell"><span class="icon">' + ico + '</span>' + entry.name + '</div></td>' +
          '<td>' + badge + '</td>' +
          '<td><span class="size-val">' + formatSize(entry.size) + '</span></td>' +
          '<td>' + action + '</td>';
        tbody.appendChild(tr);
      });
      tbody.querySelectorAll('[data-open]').forEach(el => el.onclick = () => browse(el.dataset.open));
      tbody.querySelectorAll('[data-download]').forEach(el => el.onclick = () => downloadFile(el.dataset.download));
      document.getElementById('statusLeft').textContent = dirs + ' dossier' + (dirs !== 1 ? 's' : '') + ', ' + files + ' fichier' + (files !== 1 ? 's' : '');
      document.getElementById('statusRight').textContent = data.path;
    }

    function getIcon(name) {
      const ext = name.split('.').pop().toLowerCase();
      const map = { exe:'⚙️', dll:'🔧', txt:'📄', pdf:'📕', png:'🖼️', jpg:'🖼️', jpeg:'🖼️', gif:'🖼️', mp4:'🎬', mp3:'🎵', zip:'📦', rar:'📦', '7z':'📦', json:'📋', xml:'📋', csv:'📊', xlsx:'📊', doc:'📝', docx:'📝', ps1:'🔷', bat:'🖥️', cmd:'🖥️', log:'📓' };
      return map[ext] || '📄';
    }

    async function downloadFile(path) {
      document.getElementById('statusLeft').textContent = 'Telechargement...';
      document.getElementById('statusRight').textContent = path;
      const res = await fetch('/api/download?agent=' + encodeURIComponent(activeAgent) + '&path=' + encodeURIComponent(path));
      if (!res.ok) {
        let message = 'Download failed';
        try {
          const data = await res.json();
          message = data.error || message;
        } catch (_) {
          message = await res.text() || message;
        }
        alert(message);
        document.getElementById('statusLeft').textContent = 'Erreur';
        return;
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      const name = filenameFromDisposition(res.headers.get('content-disposition')) || path.split(/[\\/]/).pop() || 'download.bin';
      a.href = url; a.download = name; a.click();
      URL.revokeObjectURL(url);
      document.getElementById('statusLeft').textContent = 'Telecharge : ' + name;
    }

    function filenameFromDisposition(value) {
      if (!value) return '';
      const utf = value.match(/filename\*=UTF-8''([^;]+)/i);
      if (utf) return decodeURIComponent(utf[1]);
      const ascii = value.match(/filename="([^"]+)"/i);
      return ascii ? ascii[1] : '';
    }

    document.getElementById('browseBtn').onclick = () => browse();
    document.getElementById('upBtn').onclick = () => { if (currentParent) browse(currentParent); };
    document.getElementById('path').addEventListener('keydown', e => { if (e.key === 'Enter') browse(); });
    loadAgents();
  </script>
</body>
</html>`
