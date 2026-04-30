package process

import (
	"context"

	core "dappco.re/go"
)

func commandContext(ctx context.Context, name string, arg ...string) *core.Cmd {
	path := name
	if result := lookPath(name); result.OK {
		path = result.Value.(string)
	}

	cmd := &core.Cmd{
		Path: path,
		Args: append([]string{name}, arg...),
	}
	return cmd
}

func lookPath(file string) core.Result {
	if file == "" {
		return core.Fail(core.E("lookPath", "executable file not found in PATH", nil))
	}
	if core.Contains(file, string(core.PathSeparator)) {
		if isExecutable(file) {
			return core.Ok(file)
		}
		return core.Fail(core.E("lookPath", core.Sprintf("executable file %q not found", file), nil))
	}

	for _, dir := range core.Split(core.Getenv("PATH"), string(core.PathListSeparator)) {
		if dir == "" {
			dir = "."
		}
		path := core.PathJoin(dir, file)
		if isExecutable(path) {
			return core.Ok(path)
		}
	}
	return core.Fail(core.E("lookPath", core.Sprintf("executable file %q not found in PATH", file), nil))
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
