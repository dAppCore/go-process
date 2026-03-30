package process

import (
	"path"
	"strconv"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
)

// DaemonEntry records a running daemon in the registry.
//
//	entry := process.DaemonEntry{Code: "myapp", Daemon: "serve", PID: 1234}
type DaemonEntry struct {
	Code    string    `json:"code"`
	Daemon  string    `json:"daemon"`
	PID     int       `json:"pid"`
	Health  string    `json:"health,omitempty"`
	Project string    `json:"project,omitempty"`
	Binary  string    `json:"binary,omitempty"`
	Started time.Time `json:"started"`
}

// Registry tracks running daemons via JSON files in a directory.
//
//	reg := process.NewRegistry("/tmp/process-daemons")
type Registry struct {
	dir string
}

// NewRegistry creates a registry backed by the given directory.
//
//	reg := process.NewRegistry("/tmp/process-daemons")
func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir}
}

// DefaultRegistry returns a registry using ~/.core/daemons/.
//
//	reg := process.DefaultRegistry()
func DefaultRegistry() *Registry {
	home, err := userHomeDir()
	if err != nil {
		home = tempDir()
	}
	return NewRegistry(path.Join(home, ".core", "daemons"))
}

// Register writes a daemon entry to the registry directory.
// If Started is zero, it is set to the current time.
// The directory is created if it does not exist.
func (r *Registry) Register(entry DaemonEntry) error {
	if entry.Started.IsZero() {
		entry.Started = time.Now()
	}

	if err := coreio.Local.EnsureDir(r.dir); err != nil {
		return core.E("registry.register", "failed to create registry directory", err)
	}

	data, err := marshalDaemonEntry(entry)
	if err != nil {
		return core.E("registry.register", "failed to marshal entry", err)
	}

	if err := coreio.Local.Write(r.entryPath(entry.Code, entry.Daemon), data); err != nil {
		return core.E("registry.register", "failed to write entry file", err)
	}
	return nil
}

// Unregister removes a daemon entry from the registry.
func (r *Registry) Unregister(code, daemon string) error {
	if err := coreio.Local.Delete(r.entryPath(code, daemon)); err != nil {
		return core.E("registry.unregister", "failed to delete entry file", err)
	}
	return nil
}

// Get reads a single daemon entry and checks whether its process is alive.
// If the process is dead, the stale file is removed and (nil, false) is returned.
func (r *Registry) Get(code, daemon string) (*DaemonEntry, bool) {
	path := r.entryPath(code, daemon)

	data, err := coreio.Local.Read(path)
	if err != nil {
		return nil, false
	}

	entry, err := unmarshalDaemonEntry(data)
	if err != nil {
		_ = coreio.Local.Delete(path)
		return nil, false
	}

	if !isAlive(entry.PID) {
		_ = coreio.Local.Delete(path)
		return nil, false
	}

	return &entry, true
}

// List returns all alive daemon entries, pruning any with dead PIDs.
func (r *Registry) List() ([]DaemonEntry, error) {
	if !coreio.Local.Exists(r.dir) {
		return nil, nil
	}

	entries, err := coreio.Local.List(r.dir)
	if err != nil {
		return nil, core.E("registry.list", "failed to list registry directory", err)
	}

	var alive []DaemonEntry
	for _, entryFile := range entries {
		if entryFile.IsDir() || !core.HasSuffix(entryFile.Name(), ".json") {
			continue
		}
		path := path.Join(r.dir, entryFile.Name())
		data, err := coreio.Local.Read(path)
		if err != nil {
			continue
		}

		entry, err := unmarshalDaemonEntry(data)
		if err != nil {
			_ = coreio.Local.Delete(path)
			continue
		}

		if !isAlive(entry.PID) {
			_ = coreio.Local.Delete(path)
			continue
		}

		alive = append(alive, entry)
	}

	return alive, nil
}

// entryPath returns the filesystem path for a daemon entry.
func (r *Registry) entryPath(code, daemon string) string {
	name := sanitizeRegistryComponent(code) + "-" + sanitizeRegistryComponent(daemon) + ".json"
	return path.Join(r.dir, name)
}

// isAlive checks whether a process with the given PID is running.
func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := processHandle(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func sanitizeRegistryComponent(value string) string {
	buf := make([]byte, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '/' {
			buf[i] = '-'
			continue
		}
		buf[i] = value[i]
	}
	return string(buf)
}

func marshalDaemonEntry(entry DaemonEntry) (string, error) {
	fields := []struct {
		key   string
		value string
	}{
		{key: "code", value: quoteJSONString(entry.Code)},
		{key: "daemon", value: quoteJSONString(entry.Daemon)},
		{key: "pid", value: strconv.Itoa(entry.PID)},
	}

	if entry.Health != "" {
		fields = append(fields, struct {
			key   string
			value string
		}{key: "health", value: quoteJSONString(entry.Health)})
	}
	if entry.Project != "" {
		fields = append(fields, struct {
			key   string
			value string
		}{key: "project", value: quoteJSONString(entry.Project)})
	}
	if entry.Binary != "" {
		fields = append(fields, struct {
			key   string
			value string
		}{key: "binary", value: quoteJSONString(entry.Binary)})
	}

	fields = append(fields, struct {
		key   string
		value string
	}{
		key:   "started",
		value: quoteJSONString(entry.Started.Format(time.RFC3339Nano)),
	})

	builder := core.NewBuilder()
	builder.WriteString("{\n")
	for i, field := range fields {
		builder.WriteString(core.Concat("  ", quoteJSONString(field.key), ": ", field.value))
		if i < len(fields)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("}")
	return builder.String(), nil
}

func unmarshalDaemonEntry(data string) (DaemonEntry, error) {
	values, err := parseJSONObject(data)
	if err != nil {
		return DaemonEntry{}, err
	}

	entry := DaemonEntry{
		Code:    values["code"],
		Daemon:  values["daemon"],
		Health:  values["health"],
		Project: values["project"],
		Binary:  values["binary"],
	}

	pidValue, ok := values["pid"]
	if !ok {
		return DaemonEntry{}, core.E("Registry.unmarshalDaemonEntry", "missing pid", nil)
	}
	entry.PID, err = strconv.Atoi(pidValue)
	if err != nil {
		return DaemonEntry{}, core.E("Registry.unmarshalDaemonEntry", "invalid pid", err)
	}

	startedValue, ok := values["started"]
	if !ok {
		return DaemonEntry{}, core.E("Registry.unmarshalDaemonEntry", "missing started", nil)
	}
	entry.Started, err = time.Parse(time.RFC3339Nano, startedValue)
	if err != nil {
		return DaemonEntry{}, core.E("Registry.unmarshalDaemonEntry", "invalid started timestamp", err)
	}

	return entry, nil
}

func parseJSONObject(data string) (map[string]string, error) {
	trimmed := core.Trim(data)
	if trimmed == "" {
		return nil, core.E("Registry.parseJSONObject", "empty JSON object", nil)
	}
	if trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return nil, core.E("Registry.parseJSONObject", "invalid JSON object", nil)
	}

	values := make(map[string]string)
	index := skipJSONSpace(trimmed, 1)
	for index < len(trimmed) {
		if trimmed[index] == '}' {
			return values, nil
		}

		key, next, err := parseJSONString(trimmed, index)
		if err != nil {
			return nil, err
		}

		index = skipJSONSpace(trimmed, next)
		if index >= len(trimmed) || trimmed[index] != ':' {
			return nil, core.E("Registry.parseJSONObject", "missing key separator", nil)
		}

		index = skipJSONSpace(trimmed, index+1)
		if index >= len(trimmed) {
			return nil, core.E("Registry.parseJSONObject", "missing value", nil)
		}

		var value string
		if trimmed[index] == '"' {
			value, index, err = parseJSONString(trimmed, index)
			if err != nil {
				return nil, err
			}
		} else {
			start := index
			for index < len(trimmed) && trimmed[index] != ',' && trimmed[index] != '}' {
				index++
			}
			value = core.Trim(trimmed[start:index])
		}
		values[key] = value

		index = skipJSONSpace(trimmed, index)
		if index >= len(trimmed) {
			break
		}
		if trimmed[index] == ',' {
			index = skipJSONSpace(trimmed, index+1)
			continue
		}
		if trimmed[index] == '}' {
			return values, nil
		}
		return nil, core.E("Registry.parseJSONObject", "invalid object separator", nil)
	}

	return nil, core.E("Registry.parseJSONObject", "unterminated JSON object", nil)
}

func parseJSONString(data string, start int) (string, int, error) {
	if start >= len(data) || data[start] != '"' {
		return "", 0, core.E("Registry.parseJSONString", "expected quoted string", nil)
	}

	builder := core.NewBuilder()
	for index := start + 1; index < len(data); index++ {
		ch := data[index]
		if ch == '"' {
			return builder.String(), index + 1, nil
		}
		if ch != '\\' {
			builder.WriteByte(ch)
			continue
		}

		index++
		if index >= len(data) {
			return "", 0, core.E("Registry.parseJSONString", "unterminated escape sequence", nil)
		}

		switch data[index] {
		case '"', '\\', '/':
			builder.WriteByte(data[index])
		case 'b':
			builder.WriteByte('\b')
		case 'f':
			builder.WriteByte('\f')
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case 'u':
			if index+4 >= len(data) {
				return "", 0, core.E("Registry.parseJSONString", "short unicode escape", nil)
			}
			r, err := strconv.ParseInt(data[index+1:index+5], 16, 32)
			if err != nil {
				return "", 0, core.E("Registry.parseJSONString", "invalid unicode escape", err)
			}
			builder.WriteRune(rune(r))
			index += 4
		default:
			return "", 0, core.E("Registry.parseJSONString", "invalid escape sequence", nil)
		}
	}

	return "", 0, core.E("Registry.parseJSONString", "unterminated string", nil)
}

func skipJSONSpace(data string, index int) int {
	for index < len(data) {
		switch data[index] {
		case ' ', '\n', '\r', '\t':
			index++
		default:
			return index
		}
	}
	return index
}

func quoteJSONString(value string) string {
	builder := core.NewBuilder()
	builder.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\\', '"':
			builder.WriteByte('\\')
			builder.WriteByte(value[i])
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if value[i] < 0x20 {
				builder.WriteString(core.Sprintf("\\u%04x", value[i]))
				continue
			}
			builder.WriteByte(value[i])
		}
	}
	builder.WriteByte('"')
	return builder.String()
}
