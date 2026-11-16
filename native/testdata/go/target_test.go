// SPDX-License-Identifier: EPL-2.0
package fixture

import "testing"

func FuzzClassify(f *testing.F) {
	f.Add([]byte("A"))
	f.Add([]byte("B"))
	f.Fuzz(func(t *testing.T, data []byte) { _ = Classify(data) })
}
