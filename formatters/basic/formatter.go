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
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/forkcloser/yamlfmt"
	yamlFeature "github.com/forkcloser/yamlfmt/formatters/basic/features"
	"github.com/forkcloser/yamlfmt/internal/configmap"
	"github.com/forkcloser/yamlfmt/pkg/yaml"
)

const BasicFormatterType string = "basic"

type BasicFormatter struct {
	Config       *Config
	Features     yamlfmt.FeatureList
	YAMLFeatures yamlFeature.YAMLFeatureList
}

// yamlfmt.Formatter interface

func (f *BasicFormatter) Type() string {
	return BasicFormatterType
}

func (f *BasicFormatter) Format(input []byte) ([]byte, error) {
	// Run all features with BeforeActions
	ctx := context.Background()

	ctx, yamlContent, err := f.Features.ApplyFeatures(ctx, input, yamlfmt.FeatureApplyBefore)
	if err != nil {
		return nil, err
	}

	// Format the yaml content
	reader := bytes.NewReader(yamlContent)
	decoder := f.getNewDecoder(reader)
	documents := []yaml.Node{}

	for {
		var docNode yaml.Node

		if err = decoder.Decode(&docNode); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		documents = append(documents, docNode)
	}

	if len(documents) == 0 {
		return input, nil
	}

	// Run all YAML features.
	for _, d := range documents {
		if err = f.YAMLFeatures.ApplyFeatures(d); err != nil {
			return nil, err
		}
	}

	encoded, err := f.encodeDocuments(documents)
	if err != nil {
		return nil, err
	}

	// Run all features with AfterActions
	_, resultYaml, err := f.Features.ApplyFeatures(ctx, encoded, yamlfmt.FeatureApplyAfter)
	if err != nil {
		return nil, err
	}

	return resultYaml, nil
}

func (f *BasicFormatter) ConfigMap() (map[string]any, error) {
	configMap := configmap.Map(f.Config)
	configMap["type"] = BasicFormatterType

	return configMap, nil
}

func (f *BasicFormatter) getNewDecoder(reader io.Reader) *yaml.Decoder {
	d := yaml.NewDecoder(reader)
	if f.Config.ScanFoldedAsLiteral {
		d.SetScanBlockScalarAsLiteral(true)
	}

	return d
}

// encodeDocuments writes the documents in order. An empty document decodes
// to a null scalar with no text and encodes to nothing, and nothing between
// two markers is one document, not two: its marker is its only form, so it
// is written here, and a document encoded after one opens with a marker of
// its own, as the encoder does from its second document on.
func (f *BasicFormatter) encodeDocuments(documents []yaml.Node) ([]byte, error) {
	var b bytes.Buffer

	e := f.getNewEncoder(&b)
	encoded, markers := 0, 0

	for i, doc := range documents {
		if isEmptyDocument(&doc) {
			f.writeEmptyDocument(&b, &doc, i == 0)

			markers++

			continue
		}

		// The encoder's first document starts implicitly unless explicit
		// starts were asked for, and would merge with a marker before it:
		// after an empty document the encoder is asked for the marker, so
		// that it knows a marker precedes its first content and sets a
		// head comment apart from it, as it does for its later documents.
		if encoded == 0 && markers > 0 && !f.Config.IncludeDocumentStart {
			e.SetExplicitDocumentStart(true)
		}

		if err := e.Encode(&doc); err != nil {
			return nil, err
		}

		encoded++
	}

	return b.Bytes(), nil
}

// writeEmptyDocument writes an empty document: its comments before the
// marker stay before it, the marker, then the comments after it, each
// comment block as the lines it holds. The decoder puts a comment on the
// marker's own line among those after it, so it comes back on its own line.
// After another document, a comment on the line right after the marker is
// read as that document's foot, and an empty line between them is not kept
// when line breaks are retained, so there the first comment line goes on
// the marker's line, the form that reads back as this document's whatever
// the configuration.
func (f *BasicFormatter) writeEmptyDocument(b *bytes.Buffer, doc *yaml.Node, first bool) {
	f.writeComment(b, doc.HeadComment)
	b.WriteString("---")

	scalar := doc.Content[0]
	onMarker := !first

	for _, comment := range []string{scalar.HeadComment, scalar.LineComment, scalar.FootComment, doc.LineComment, doc.FootComment} {
		// A head keeps the break an empty line after it added to its text,
		// for the parser's own reading of it; here it would be an empty
		// line of its own, which the next pass does not read as comment.
		comment = strings.TrimRight(comment, "\n")

		for line := range strings.SplitSeq(comment, "\n") {
			if comment == "" {
				break
			}

			if onMarker {
				b.WriteString(" " + line)

				onMarker = false

				continue
			}

			b.WriteString(f.lineBreak() + line)
		}
	}

	b.WriteString(f.lineBreak())
}

// writeComment writes a comment block's lines, if any.
func (f *BasicFormatter) writeComment(b *bytes.Buffer, comment string) {
	if comment == "" {
		return
	}

	for line := range strings.SplitSeq(comment, "\n") {
		b.WriteString(line + f.lineBreak())
	}
}

// isEmptyDocument reports a document with no text at all: a null scalar
// the decoder made for it, as against an explicit `null` or `~`, or an
// anchored one, whose anchor is text the marker alone would lose.
func isEmptyDocument(doc *yaml.Node) bool {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return false
	}

	c := doc.Content[0]

	return c.Kind == yaml.ScalarNode && c.Tag == "!!null" && c.Value == "" && c.Anchor == ""
}

// lineBreak is the configured line terminator.
func (f *BasicFormatter) lineBreak() string {
	if f.Config.LineEnding == yamlfmt.LineBreakStyleCRLF {
		return "\r\n"
	}

	return "\n"
}

func (f *BasicFormatter) getNewEncoder(buf *bytes.Buffer) *yaml.Encoder {
	e := yaml.NewEncoder(buf)
	e.SetIndent(f.Config.Indent)

	if f.Config.LineLength > 0 {
		e.SetWidth(f.Config.LineLength)
	}

	if f.Config.LineEnding == yamlfmt.LineBreakStyleCRLF {
		e.SetLineBreakStyle(yaml.LineBreakStyleCRLF)
	}

	e.SetExplicitDocumentStart(f.Config.IncludeDocumentStart)
	e.SetAssumeBlockAsLiteral(f.Config.ScanFoldedAsLiteral)
	e.SetIndentlessBlockSequence(f.Config.IndentlessArrays)
	e.SetDropMergeTag(f.Config.DropMergeTag)
	e.SetPadLineComments(f.Config.PadLineComments)
	e.SetPreserveCommentIndents(f.Config.PreserveCommentIndents)

	if f.Config.ArrayIndent > 0 {
		e.SetArrayIndent(f.Config.ArrayIndent)
	}

	e.SetIndentRootArray(f.Config.IndentRootArray)

	// Yes I know I could SetCorrectAliasKeys(!f.Config.DisableAliasKeyCorrection)
	// but I know myself and I know I'll get confused and have to go look up
	// the source again next time I look and forget.
	if !f.Config.DisableAliasKeyCorrection {
		e.SetCorrectAliasKeys(true)
	}

	return e
}
