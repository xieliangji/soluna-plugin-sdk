// Package semver 解析并比较语义化版本 2.0.0 值。
package semver

import (
	"fmt"
	"strings"
	"unicode"
)

// Version 是已校验的语义化版本。
type Version struct {
	major      string
	minor      string
	patch      string
	prerelease []string
}

// Parse 校验完整的语义化版本字符串。
func Parse(value string) (Version, error) {
	original := value
	if value == "" || strings.TrimSpace(value) != value {
		return Version{}, fmt.Errorf("语义版本 %q 无效", original)
	}
	withoutBuild, build, hasBuild := strings.Cut(value, "+")
	if hasBuild && !validIdentifiers(build, false) {
		return Version{}, fmt.Errorf("语义版本 %q 无效", original)
	}
	core, prerelease, hasPrerelease := strings.Cut(withoutBuild, "-")
	if hasPrerelease && !validIdentifiers(prerelease, true) {
		return Version{}, fmt.Errorf("语义版本 %q 无效", original)
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 || !validCoreNumber(parts[0]) || !validCoreNumber(parts[1]) || !validCoreNumber(parts[2]) {
		return Version{}, fmt.Errorf("语义版本 %q 无效", original)
	}
	result := Version{major: parts[0], minor: parts[1], patch: parts[2]}
	if hasPrerelease {
		result.prerelease = strings.Split(prerelease, ".")
	}
	return result, nil
}

// ParseCommandOutput 校验命令报告的语义化版本。在此进程边界允许并移除一个约定的 v 前缀。
func ParseCommandOutput(value string) (Version, error) {
	return Parse(strings.TrimPrefix(value, "v"))
}

// Compare 在 Version 小于、等于或大于 other 时分别返回 -1、0 或 1。
// 构建元数据不影响优先级。
func (version Version) Compare(other Version) int {
	for _, pair := range [][2]string{{version.major, other.major}, {version.minor, other.minor}, {version.patch, other.patch}} {
		if comparison := compareNumeric(pair[0], pair[1]); comparison != 0 {
			return comparison
		}
	}
	if len(version.prerelease) == 0 && len(other.prerelease) == 0 {
		return 0
	}
	if len(version.prerelease) == 0 {
		return 1
	}
	if len(other.prerelease) == 0 {
		return -1
	}
	count := min(len(version.prerelease), len(other.prerelease))
	for index := 0; index < count; index++ {
		left, right := version.prerelease[index], other.prerelease[index]
		leftNumeric, rightNumeric := numeric(left), numeric(right)
		switch {
		case leftNumeric && rightNumeric:
			if comparison := compareNumeric(left, right); comparison != 0 {
				return comparison
			}
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		case left < right:
			return -1
		case left > right:
			return 1
		}
	}
	switch {
	case len(version.prerelease) < len(other.prerelease):
		return -1
	case len(version.prerelease) > len(other.prerelease):
		return 1
	default:
		return 0
	}
}

func validCoreNumber(value string) bool {
	return numeric(value) && (value == "0" || value[0] != '0')
}

func validIdentifiers(value string, prerelease bool) bool {
	if value == "" {
		return false
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" || (prerelease && numeric(identifier) && len(identifier) > 1 && identifier[0] == '0') {
			return false
		}
		for _, character := range identifier {
			if !unicode.IsDigit(character) && !unicode.IsLetter(character) && character != '-' {
				return false
			}
			if character > unicode.MaxASCII {
				return false
			}
		}
	}
	return true
}

func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func compareNumeric(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if left == "" {
		left = "0"
	}
	if right == "" {
		right = "0"
	}
	switch {
	case len(left) < len(right):
		return -1
	case len(left) > len(right):
		return 1
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
