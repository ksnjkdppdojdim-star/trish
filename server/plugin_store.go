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
	mu      sync.RWMutex
	path    string
	records map[string]*pluginRecord
}

type pluginRecord struct {
	Name          string                      `json:"name"`
	ActiveVersion string                      `json:"active_version"`
	Disabled      bool                        `json:"disabled,omitempty"`
	Versions      []core.DynamicPluginPackage `json:"versions"`
}

func newPluginStore(path string) (*pluginStore, error) {
	store := &pluginStore{
		path:    path,
		records: make(map[string]*pluginRecord),
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

	var records []pluginRecord
	if err := json.Unmarshal(data, &records); err == nil && len(records) > 0 && records[0].Name != "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i := range records {
			record := records[i]
			if err := normalizePluginRecord(&record); err != nil {
				return err
			}
			s.records[record.Name] = &record
		}
		return nil
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
		record := recordFromPackage(&pkg)
		s.records[record.Name] = record
	}
	return nil
}

func (s *pluginStore) install(pkg *core.DynamicPluginPackage) error {
	if err := pkg.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	record, ok := s.records[pkg.Manifest.Name]
	if !ok {
		record = recordFromPackage(pkg)
		s.records[record.Name] = record
	} else {
		upsertPluginVersion(record, pkg)
		record.ActiveVersion = pkg.Manifest.Version
		record.Disabled = false
	}
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
	if _, ok := s.records[name]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("plugin %s not found", name)
	}
	delete(s.records, name)
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(snapshot)
}

func (s *pluginStore) setEnabled(name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	s.mu.Lock()
	record, ok := s.records[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("plugin %s not found", name)
	}
	record.Disabled = !enabled
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(snapshot)
}

func (s *pluginStore) rollback(name string, version string) error {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" || version == "" {
		return fmt.Errorf("plugin name and version are required")
	}
	s.mu.Lock()
	record, ok := s.records[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("plugin %s not found", name)
	}
	if !recordHasVersion(record, version) {
		s.mu.Unlock()
		return fmt.Errorf("plugin %s version %s not found", name, version)
	}
	record.ActiveVersion = version
	record.Disabled = false
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(snapshot)
}

func (s *pluginStore) list() []core.DynamicPluginManifest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugins := make([]core.DynamicPluginManifest, 0, len(s.records))
	for _, record := range s.records {
		pkg := activePluginPackage(record)
		if pkg == nil {
			continue
		}
		manifest := pkg.Manifest
		manifest.Disabled = record.Disabled
		plugins = append(plugins, manifest)
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	return plugins
}

func (s *pluginStore) versions(name string) ([]core.DynamicPluginManifest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	versions := make([]core.DynamicPluginManifest, 0, len(record.Versions))
	for _, pkg := range record.Versions {
		manifest := pkg.Manifest
		manifest.Disabled = record.Disabled || manifest.Version != record.ActiveVersion
		versions = append(versions, manifest)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})
	return versions, nil
}

func (s *pluginStore) commandNames() []string {
	seen := make(map[string]bool)
	names := []string{}
	for _, plugin := range s.list() {
		if plugin.Disabled {
			continue
		}
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
	for _, record := range s.records {
		if record.Disabled {
			continue
		}
		pkg := activePluginPackage(record)
		if pkg == nil {
			continue
		}
		for _, command := range pkg.Manifest.Commands {
			if command.Name == commandName {
				return clonePluginPackage(pkg), true
			}
		}
	}
	return nil, false
}

func (s *pluginStore) snapshotLocked() []pluginRecord {
	records := make([]pluginRecord, 0, len(s.records))
	for _, record := range s.records {
		cloned := clonePluginRecord(record)
		records = append(records, *cloned)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})
	return records
}

func (s *pluginStore) save(records []pluginRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
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
	cloned.Manifest.Permissions = append([]string(nil), pkg.Manifest.Permissions...)
	cloned.Manifest.Commands = append([]core.DynamicPluginCommand(nil), pkg.Manifest.Commands...)
	cloned.Files = append([]core.DynamicPluginFile(nil), pkg.Files...)
	return &cloned
}

func clonePluginRecord(record *pluginRecord) *pluginRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	cloned.Versions = make([]core.DynamicPluginPackage, 0, len(record.Versions))
	for i := range record.Versions {
		cloned.Versions = append(cloned.Versions, *clonePluginPackage(&record.Versions[i]))
	}
	return &cloned
}

func recordFromPackage(pkg *core.DynamicPluginPackage) *pluginRecord {
	cloned := clonePluginPackage(pkg)
	return &pluginRecord{
		Name:          cloned.Manifest.Name,
		ActiveVersion: cloned.Manifest.Version,
		Disabled:      cloned.Manifest.Disabled,
		Versions:      []core.DynamicPluginPackage{*cloned},
	}
}

func normalizePluginRecord(record *pluginRecord) error {
	record.Name = strings.TrimSpace(record.Name)
	if record.Name == "" {
		return fmt.Errorf("plugin record name is required")
	}
	if len(record.Versions) == 0 {
		return fmt.Errorf("plugin %s has no versions", record.Name)
	}
	for i := range record.Versions {
		if err := record.Versions[i].Validate(); err != nil {
			return err
		}
		if record.Versions[i].Manifest.Name != record.Name {
			return fmt.Errorf("plugin record %s contains package %s", record.Name, record.Versions[i].Manifest.Name)
		}
	}
	if strings.TrimSpace(record.ActiveVersion) == "" || !recordHasVersion(record, record.ActiveVersion) {
		record.ActiveVersion = record.Versions[len(record.Versions)-1].Manifest.Version
	}
	return nil
}

func upsertPluginVersion(record *pluginRecord, pkg *core.DynamicPluginPackage) {
	cloned := clonePluginPackage(pkg)
	for i := range record.Versions {
		if record.Versions[i].Manifest.Version == cloned.Manifest.Version {
			record.Versions[i] = *cloned
			return
		}
	}
	record.Versions = append(record.Versions, *cloned)
}

func recordHasVersion(record *pluginRecord, version string) bool {
	for i := range record.Versions {
		if record.Versions[i].Manifest.Version == version {
			return true
		}
	}
	return false
}

func activePluginPackage(record *pluginRecord) *core.DynamicPluginPackage {
	if record == nil {
		return nil
	}
	for i := range record.Versions {
		if record.Versions[i].Manifest.Version == record.ActiveVersion {
			pkg := clonePluginPackage(&record.Versions[i])
			pkg.Manifest.Disabled = record.Disabled
			return pkg
		}
	}
	if len(record.Versions) == 0 {
		return nil
	}
	pkg := clonePluginPackage(&record.Versions[len(record.Versions)-1])
	pkg.Manifest.Disabled = record.Disabled
	return pkg
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
