// Copyright 2026 Forkcloser
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
