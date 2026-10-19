// Copyright 2026 Forkcloser. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package yamlfmt

// The packages log through log/slog's default logger and never choose the
// output; a debug record carries the code it belongs to as the DebugCodeAttr
// attribute, and the command's -debug flag names the codes its handler lets
// through.
const (
	// DebugCodeAttr is the attribute a debug record carries its code in.
	DebugCodeAttr = "code"
	// DebugCodeConfig is the configuration lookup: which file is used and why.
	DebugCodeConfig = "config"
	// DebugCodePaths is path discovery: the patterns tried, matched and excluded.
	DebugCodePaths = "paths"
	// DebugCodeDiffs is every diff of every formatted file.
	DebugCodeDiffs = "diffs"
)
