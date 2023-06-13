// Package diff computes line diffs, in the style of Unix diff, from a longest
// common subsequence.
package diff

// lcs returns a longest common subsequence of a and b using the O(n*m)
// dynamic-programming table, where dp[i][j] is the LCS length of a[i:] and
// b[j:].
func lcs[T comparable](a, b []T) []T {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		return nil
	}

	// The extra row and column are the empty-suffix base cases.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	res := make([]T, 0, dp[0][0])
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			res = append(res, a[i])
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			i++
		default:
			j++
		}
	}
	return res
}

// LCS returns a longest common subsequence of two slices of lines.
func LCS(a, b []string) []string {
	return lcs(a, b)
}

// LCSString returns a longest common subsequence of two strings, compared rune
// by rune.
func LCSString(a, b string) string {
	return string(lcs([]rune(a), []rune(b)))
}
