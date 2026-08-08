// Package lookpath resolves a command name to a runnable path.
//
// It exists because this module carried TWO copies of that logic — one in
// exec/, one behind Program.Find — and only the first was fixed when the
// Windows defects were found. go-inference calls the second, so its `git`
// resolution stayed broken through a release that claimed to fix it. One
// implementation, two consumers, is the fix for that class of miss as much as
// for the defects themselves.
//
// The Windows defects, for the record:
//   - executability was tested as mode&0111, which Windows never sets — Stat
//     synthesises 0666, or 0444 for a read-only file — so every file on the
//     platform was rejected, git.exe included
//   - %PATHEXT% was never expanded, and a command is written "git", not
//     "git.exe"
//   - the path-vs-name check matched only the platform separator, so a name
//     spelled "bin/tool" was hunted on PATH instead of checked directly
package lookpath

import core "dappco.re/go"

// defaultPathExt is the extension list Windows itself assumes when %PATHEXT%
// is unset.
const defaultPathExt = ".COM;.EXE;.BAT;.CMD"

// Look resolves file to a runnable path, searching PATH when file carries
// no directory component.
//
// On Windows a command is named without its extension — "git", not "git.exe" —
// so every candidate is also tried with each %PATHEXT% suffix. Without that,
// no Windows executable is ever found by its bare name.
func Look(file string) core.Result {
	return LookWith(file, Extensions())
}

// LookWith is Look with the extension list supplied rather than read
// from the environment. Taking it as an argument is what lets the Windows
// resolution rules be pinned on a POSIX runner — the tests drive it with a
// fixture PATH and a fake %PATHEXT%, so no Windows box is needed to prove the
// logic and the CI lane is left to prove only the wiring.
func LookWith(file string, extensions []string) core.Result {
	if file == "" {
		return core.Fail(core.E("Look", "executable file not found in PATH", nil))
	}
	if ContainsSeparator(file) {
		if path, ok := firstExecutable(file, extensions); ok {
			return core.Ok(path)
		}
		return core.Fail(core.E("Look", core.Sprintf("executable file %q not found", file), nil))
	}

	for _, dir := range core.Split(core.Getenv("PATH"), string(core.PathListSeparator)) {
		if dir == "" {
			dir = "."
		}
		if path, ok := firstExecutable(core.PathJoin(dir, file), extensions); ok {
			return core.Ok(path)
		}
	}
	return core.Fail(core.E("Look", core.Sprintf("executable file %q not found in PATH", file), nil))
}

// firstExecutable returns the first of base's candidate spellings that names a
// runnable file.
func firstExecutable(base string, extensions []string) (string, bool) {
	for _, candidate := range Candidates(base, extensions) {
		if IsExecutableWith(candidate, extensions) {
			return candidate, true
		}
	}
	return "", false
}

// Candidates returns the spellings of base to try, in order. With no
// extensions in play — every POSIX case — base stands alone. On Windows a base
// that already ends in a listed extension also stands alone; anything else is
// tried once per extension, so "git" becomes "git.com", "git.exe" and so on.
func Candidates(base string, extensions []string) []string {
	if len(extensions) == 0 || HasExtension(base, extensions) {
		return []string{base}
	}
	candidates := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		candidates = append(candidates, base+extension)
	}
	return candidates
}

// HasExtension reports whether base already ends in one of the
// listed extensions. Windows filenames are case-insensitive, so the comparison
// is too.
func HasExtension(base string, extensions []string) bool {
	lowered := core.Lower(base)
	for _, extension := range extensions {
		if core.HasSuffix(lowered, extension) {
			return true
		}
	}
	return false
}

// Extensions returns the %PATHEXT% list, or nil off Windows where a
// command name is used exactly as written.
func Extensions() []string {
	if string(core.PathSeparator) != `\` {
		return nil
	}
	return ParsePathExt(core.Getenv("PATHEXT"))
}

// ParsePathExt normalises a %PATHEXT% value into lower-cased, dot-prefixed
// extensions, dropping blanks and duplicates. An unset or unusable value falls
// back to the set Windows assumes, so a stripped environment still resolves
// the common executables.
func ParsePathExt(value string) []string {
	extensions := make([]string, 0, 8)
	seen := make(map[string]bool, 8)
	for _, field := range core.Split(value, ";") {
		extension := core.Lower(core.Trim(field))
		if extension == "" || extension == "." {
			continue
		}
		if !core.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		if seen[extension] {
			continue
		}
		seen[extension] = true
		extensions = append(extensions, extension)
	}
	if len(extensions) == 0 && value != defaultPathExt {
		return ParsePathExt(defaultPathExt)
	}
	return extensions
}

// ContainsSeparator reports whether file carries a directory component under
// either convention. Windows accepts '/' as well as '\', so a name spelled
// "bin/tool" there is a path to check directly, not a name to hunt on PATH.
func ContainsSeparator(file string) bool {
	if core.Contains(file, "/") {
		return true
	}
	separator := string(core.PathSeparator)
	return separator != "/" && core.Contains(file, separator)
}

func IsExecutable(path string) bool {
	return IsExecutableWith(path, Extensions())
}

// IsExecutableWith applies the platform's own rule for "this can be run".
//
// POSIX asks the mode bits. Windows has no execute bit — os.Stat synthesises
// 0666, or 0444 for a read-only file — so mode&0111 is never set there and a
// mode test rejects every file, git.exe included. Under a non-empty extension
// list the question becomes whether the suffix is one %PATHEXT% names, which
// is what Windows itself keys on.
func IsExecutableWith(path string, extensions []string) bool {
	stat := core.Stat(path)
	if !stat.OK {
		return false
	}
	info, ok := stat.Value.(core.FsFileInfo)
	if !ok || info.IsDir() {
		return false
	}
	if len(extensions) > 0 {
		return HasExtension(path, extensions)
	}
	return info.Mode()&0111 != 0
}
