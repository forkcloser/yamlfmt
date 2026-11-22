// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kyaml

import (
	"bytes"
	"errors"
	"fmt"

	yaml "go.yaml.in/yaml/v4"
	"sigs.k8s.io/yaml/kyaml"
)

const KYAMLFormatterType string = "kyaml"

var errAnchorHoldsItself = errors.New("an alias inside the node its anchor names has no KYAML form")

type KYAMLFormatter struct{}

func (f *KYAMLFormatter) Type() string {
	return KYAMLFormatterType
}

func (f *KYAMLFormatter) Format(input []byte) ([]byte, error) {
	if err := checkAliases(input); err != nil {
		return input, err
	}

	encoder := &kyaml.Encoder{}
	in := bytes.NewReader(input)

	out := bytes.Buffer{}
	if err := encoder.FromYAML(in, &out); err != nil {
		return input, err
	}

	return out.Bytes(), nil
}

// checkAliases refuses an input in which an anchor's node holds an alias to
// it: KYAML has no aliases, the encoder writes each as a copy of its anchor's
// node, and does not return from that one. The input is read by the parser
// the encoder reads it with, so that the two see the same aliases; what that
// parser refuses is left for the encoder to report.
func checkAliases(input []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(input))

	for {
		var doc yaml.Node

		if err := decoder.Decode(&doc); err != nil {
			return nil //nolint:nilerr // the encoder meets the same error and reports it
		}

		if alias := aliasToAncestor(&doc, map[*yaml.Node]struct{}{}); alias != nil {
			return fmt.Errorf("line %d: *%s: %w", alias.Line, alias.Value, errAnchorHoldsItself)
		}
	}
}

// aliasToAncestor is the first alias under node to a node it is inside of,
// open being the anchored nodes around node. An alias refers to an anchor
// before it, a node that is complete or one that is still open: only the
// second makes a cycle, so the aliases' own nodes need no visit.
func aliasToAncestor(node *yaml.Node, open map[*yaml.Node]struct{}) *yaml.Node {
	if node.Kind == yaml.AliasNode {
		if _, inside := open[node.Alias]; inside {
			return node
		}

		return nil
	}

	if node.Anchor != "" {
		open[node] = struct{}{}
		defer delete(open, node)
	}

	for _, child := range node.Content {
		if alias := aliasToAncestor(child, open); alias != nil {
			return alias
		}
	}

	return nil
}

func (f *KYAMLFormatter) ConfigMap() (map[string]any, error) {
	return map[string]any{"type": KYAMLFormatterType}, nil
}
