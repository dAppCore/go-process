package process

import (
	"context"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"
)

func commandContext(ctx context.Context, name string, arg ...string) *core.Cmd {
	path := name
	if found, err := lookPath(name); err == nil {
		path = found
	}

	cmd := &core.Cmd{
		Path: path,
		Args: append([]string{name}, arg...),
	}
	return cmd
}

func lookPath(file string) (string, goError) {
	if file == "" {
		return "", coreerr.E("lookPath", "executable file not found in PATH", nil)
	}
	if core.Contains(file, string(core.PathSeparator)) {
		if isExecutable(file) {
			return file, nil
		}
		return "", coreerr.E("lookPath", core.Sprintf("executable file %q not found", file), nil)
	}

	for _, dir := range core.Split(core.Getenv("PATH"), string(core.PathListSeparator)) {
		if dir == "" {
			dir = "."
		}
		path := core.PathJoin(dir, file)
		if isExecutable(path) {
			return path, nil
		}
	}
	return "", coreerr.E("lookPath", core.Sprintf("executable file %q not found in PATH", file), nil)
}

func isExecutable(path string) bool {
	stat := core.Stat(path)
	if !stat.OK {
		return false
	}
	info, ok := stat.Value.(core.FsFileInfo)
	if !ok || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}
