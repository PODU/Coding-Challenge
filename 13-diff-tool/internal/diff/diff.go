package diff

// dpCellLimit is the largest n*m table DiffAuto will build before switching
// to Myers, whose memory grows with the edit distance instead.
const dpCellLimit = 4_000_000

// diffFromLCS builds the edit script from a common subsequence of a and b.
// Lines of a outside it are deletions, lines of b are additions, and within a
// hunk deletions come first, as in GNU diff.
func diffFromLCS(a, b, common []string) []string {
	out := make([]string, 0, len(a)+len(b)-len(common))
	i, j, k := 0, 0, 0
	for k < len(common) {
		for i < len(a) && a[i] != common[k] {
			out = append(out, "< "+a[i])
			i++
		}
		for j < len(b) && b[j] != common[k] {
			out = append(out, "> "+b[j])
			j++
		}
		// a[i] and b[j] are both common[k], which isn't printed.
		i++
		j++
		k++
	}
	for ; i < len(a); i++ {
		out = append(out, "< "+a[i])
	}
	for ; j < len(b); j++ {
		out = append(out, "> "+b[j])
	}
	return out
}

// Diff returns the lines that differ between a and b, prefixed "< " for lines
// only in a and "> " for lines only in b.
func Diff(a, b []string) []string {
	return diffFromLCS(a, b, LCS(a, b))
}

// DiffMyers gives the same output as Diff but uses Myers's O(ND) algorithm.
func DiffMyers(a, b []string) []string {
	return diffFromLCS(a, b, myersLCS(a, b))
}

// DiffAuto uses Diff for small inputs and DiffMyers once the DP table would
// exceed dpCellLimit cells.
func DiffAuto(a, b []string) []string {
	if int64(len(a))*int64(len(b)) > dpCellLimit {
		return DiffMyers(a, b)
	}
	return Diff(a, b)
}
