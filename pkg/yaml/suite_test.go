//
// Copyright (c) 2011-2019 Canonical Ltd
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

package yaml_test

import (
	"os"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

// setenv sets key for the rest of a test; the returned func puts back what
// was there, unsetting a variable that was unset rather than leaving it empty.
func setenv(c *C, key, value string) func() {
	old, had := os.LookupEnv(key)
	c.Assert(os.Setenv(key, value), IsNil)

	return func() {
		if had {
			c.Check(os.Setenv(key, old), IsNil)
		} else {
			c.Check(os.Unsetenv(key), IsNil)
		}
	}
}
