// SPDX-License-Identifier: EPL-2.0
package fixture

func Classify(data []byte) int {
	if len(data) > 0 && data[0] == 'A' {
		return 10
	}
	if len(data) > 0 && data[0] == 'B' {
		return 20
	}
	return 30
}
