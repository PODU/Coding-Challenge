package jq

import "regexp"

// regexTest implements test(re) with Go's RE2 engine, so jq's Oniguruma flags
// and syntax extensions are not supported.
func regexTest(s, pattern string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, evalErrorf("%s is not a valid regex: %v", pattern, err)
	}
	return re.MatchString(s), nil
}
