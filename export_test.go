package yamlfmt

// PathForPattern exposes pathInPatternForm to the external tests.
func PathForPattern(cwd, pattern, path string) string {
	return pathInPatternForm(cwd, pattern, path)
}
