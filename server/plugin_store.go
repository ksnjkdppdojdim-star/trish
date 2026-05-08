package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"trish/core"
)

type pluginStore struct {
	mu       sync.RWMutex
	path     string
	packages map[string]*core.DynamicPluginPackage
}

func newPluginStore(path string) (*pluginStore, error) {
	store := &pluginStore{
		path:     path,
		packages: make(map[string]*core.DynamicPluginPackage),
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return store, nil
}

func (s *pluginStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var packages []core.DynamicPluginPackage
	if err := json.Unmarshal(data, &packages); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range packages {
		pkg := packages[i]
		if err := pkg.Validate(); err != nil {
			return err
		}
		s.packages[pkg.Manifest.Name] = &pkg
	}
	return nil
}

func (s *pluginStore) install(pkg *core.DynamicPluginPackage) error {
	if err := pkg.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	s.packages[pkg.Manifest.Name] = pkg
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(snapshot)
}

func (s *pluginStore) remove(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	s.mu.Lock()
	if _, ok := s.packages[name]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("plugin %s not found", name)
	}
	delete(s.packages, name)
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(snapshot)
}

func (s *pluginStore) list() []core.DynamicPluginManifest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugins := make([]core.DynamicPluginManifest, 0, len(s.packages))
	for _, pkg := range s.packages {
		plugins = append(plugins, pkg.Manifest)
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	return plugins
}

func (s *pluginStore) commandNames() []string {
	seen := make(map[string]bool)
	names := []string{}
	for _, plugin := range s.list() {
		for _, name := range plugin.CommandNames() {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func (s *pluginStore) findCommand(commandName string) (*core.DynamicPluginPackage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pkg := range s.packages {
		for _, command := range pkg.Manifest.Commands {
			if command.Name == commandName {
				return clonePluginPackage(pkg), true
			}
		}
	}
	return nil, false
}

func (s *pluginStore) snapshotLocked() []core.DynamicPluginPackage {
	packages := make([]core.DynamicPluginPackage, 0, len(s.packages))
	for _, pkg := range s.packages {
		packages = append(packages, *clonePluginPackage(pkg))
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Manifest.Name < packages[j].Manifest.Name
	})
	return packages
}

func (s *pluginStore) save(packages []core.DynamicPluginPackage) error {
	data, err := json.MarshalIndent(packages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func clonePluginPackage(pkg *core.DynamicPluginPackage) *core.DynamicPluginPackage {
	if pkg == nil {
		return nil
	}
	cloned := *pkg
	cloned.Manifest.OS = append([]string(nil), pkg.Manifest.OS...)
	cloned.Manifest.Commands = append([]core.DynamicPluginCommand(nil), pkg.Manifest.Commands...)
	cloned.Files = append([]core.DynamicPluginFile(nil), pkg.Files...)
	return &cloned
}

func buildPluginPowerShell(pkg *core.DynamicPluginPackage, args []string) (string, error) {
	if err := pkg.Validate(); err != nil {
		return "", err
	}
	if !pkg.Manifest.SupportsOS("windows") {
		return "", fmt.Errorf("plugin %s does not support windows agents", pkg.Manifest.Name)
	}

	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString("$pluginDir = Join-Path $env:TEMP ('trish-plugin-' + [guid]::NewGuid().ToString('N'))\n")
	b.WriteString("New-Item -ItemType Directory -Force -Path $pluginDir | Out-Null\n")
	b.WriteString("try {\n")
	for _, file := range pkg.Files {
		decoded, err := base64.StdEncoding.DecodeString(file.ContentBase64)
		if err != nil {
			return "", err
		}
		encoded := base64.StdEncoding.EncodeToString(decoded)
		rel := escapePowerShellSingleQuoted(filepath.ToSlash(file.Path))
		b.WriteString(fmt.Sprintf("  $file = Join-Path $pluginDir '%s'\n", rel))
		b.WriteString("  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $file) | Out-Null\n")
		b.WriteString(fmt.Sprintf("  [IO.File]::WriteAllBytes($file, [Convert]::FromBase64String('%s'))\n", encoded))
	}
	b.WriteString(fmt.Sprintf("  $entry = Join-Path $pluginDir '%s'\n", escapePowerShellSingleQuoted(pkg.Manifest.Entry)))
	b.WriteString("  $pluginArgs = @(")
	for i, arg := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("'")
		b.WriteString(escapePowerShellSingleQuoted(arg))
		b.WriteString("'")
	}
	b.WriteString(")\n")
	switch pkg.Manifest.Shell {
	case "cmd":
		b.WriteString("  & cmd.exe @('/C', $entry) $pluginArgs\n")
	default:
		b.WriteString("  & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $entry @pluginArgs\n")
	}
	b.WriteString("} finally {\n")
	b.WriteString("  Remove-Item -LiteralPath $pluginDir -Recurse -Force -ErrorAction SilentlyContinue\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
