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

	"github.com/mitchellh/mapstructure"

	"github.com/forkcloser/yamlfmt"
	yamlFeature "github.com/forkcloser/yamlfmt/formatters/basic/features"
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

		err := decoder.Decode(&docNode)
		if err != nil {
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
		if err := f.YAMLFeatures.ApplyFeatures(d); err != nil {
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
	configMap := map[string]any{}

	err := mapstructure.Decode(f.Config, &configMap)
	if err != nil {
		return nil, err
	}

	configMap["type"] = BasicFormatterType

	return configMap, err
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
	marker := "---" + f.lineBreak()
	encoded, markers := 0, 0

	for _, doc := range documents {
		if isEmptyDocument(&doc) {
			f.writeEmptyDocument(&b, &doc)

			markers++

			continue
		}

		// The encoder's first document starts implicitly unless explicit
		// starts were asked for, and would merge with a marker before it.
		if encoded == 0 && markers > 0 && !f.Config.IncludeDocumentStart {
			b.WriteString(marker)
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
func (f *BasicFormatter) writeEmptyDocument(b *bytes.Buffer, doc *yaml.Node) {
	f.writeComment(b, doc.HeadComment)
	b.WriteString("---" + f.lineBreak())

	scalar := doc.Content[0]
	for _, comment := range []string{scalar.HeadComment, scalar.LineComment, scalar.FootComment, doc.LineComment, doc.FootComment} {
		f.writeComment(b, comment)
	}
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
// the decoder made for it, as against an explicit `null` or `~`.
func isEmptyDocument(doc *yaml.Node) bool {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return false
	}

	c := doc.Content[0]

	return c.Kind == yaml.ScalarNode && c.Tag == "!!null" && c.Value == ""
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
