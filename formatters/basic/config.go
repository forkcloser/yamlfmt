// Copyright 2022 Google LLC
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

package basic

import (
	"github.com/forkcloser/yamlfmt"
	yamlFeatures "github.com/forkcloser/yamlfmt/formatters/basic/features"
)

type Config struct {
	Indent                    int                        `yaml:"indent"`
	IncludeDocumentStart      bool                       `yaml:"include_document_start"`
	LineEnding                yamlfmt.LineBreakStyle     `yaml:"line_ending"`
	LineLength                int                        `yaml:"max_line_length"`
	RetainLineBreaks          bool                       `yaml:"retain_line_breaks"`
	RetainLineBreaksSingle    bool                       `yaml:"retain_line_breaks_single"`
	DisallowAnchors           bool                       `yaml:"disallow_anchors"`
	ScanFoldedAsLiteral       bool                       `yaml:"scan_folded_as_literal"`
	IndentlessArrays          bool                       `yaml:"indentless_arrays"`
	DropMergeTag              bool                       `yaml:"drop_merge_tag"`
	PadLineComments           int                        `yaml:"pad_line_comments"`
	PreserveCommentIndents    bool                       `yaml:"preserve_comment_indents"`
	TrimTrailingWhitespace    bool                       `yaml:"trim_trailing_whitespace"`
	EOFNewline                bool                       `yaml:"eof_newline"`
	StripDirectives           bool                       `yaml:"strip_directives"`
	ArrayIndent               int                        `yaml:"array_indent"`
	IndentRootArray           bool                       `yaml:"indent_root_array"`
	DisableAliasKeyCorrection bool                       `yaml:"disable_alias_key_correction"`
	ForceArrayStyle           yamlFeatures.SequenceStyle `yaml:"force_array_style"`
	ForceQuoteStyle           yamlFeatures.QuoteStyle    `yaml:"force_quote_style"`
}

func DefaultConfig() *Config {
	return &Config{
		Indent:          2,
		LineEnding:      yamlfmt.LineBreakStyleLF,
		PadLineComments: 1,
	}
}
