package string_utils

import (
	"regexp"
)

// Kind of a ternary, if condition is met, return a else return b
func IfThenElse(condition bool, a interface{}, b interface{}) interface{} {
	if condition {
		return a
	}
	return b
}

// parse a string to kubernetes-like args list, ie.
//
//	arglist := ArgsToList(`-foo 1 -bar arg "quoted arg"`)
//	expected := []string{ "-foo", "1", "-bar", "args", "quoted arg" }
func ArgsToList(strArgs string) []string {
	re := regexp.MustCompile(`"([^"]*)"|'([^']*)'|(\S+)`)
	matches := re.FindAllStringSubmatch(strArgs, -1)
	var results []string
	for _, match := range matches {
		if match[1] != "" {
			// Double-quoted string
			results = append(results, match[1])
		} else if match[2] != "" {
			// Single-quoted string
			results = append(results, match[2])
		} else {
			// Unquoted word
			results = append(results, match[3])
		}
	}
	return results

}
