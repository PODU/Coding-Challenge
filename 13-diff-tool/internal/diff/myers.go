package diff

// myersLCS finds a longest common subsequence with Myers's algorithm ("An
// O(ND) Difference Algorithm and Its Variations", 1986), which is also Git's
// default. Time and space are O((n+m)*D), where D is the number of inserted
// plus deleted lines, so similar files are cheap.
func myersLCS(a, b []string) []string {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		return nil
	}

	maxD := n + m
	// v[offset+k] is the furthest x reached on diagonal k, for k in
	// [-maxD, maxD].
	offset := maxD
	v := make([]int, 2*maxD+1)

	// trace[d] is v as it stood at the start of round d, for backtracking.
	var trace [][]int

search:
	for d := 0; d <= maxD; d++ {
		snapshot := make([]int, len(v))
		copy(snapshot, v)
		trace = append(trace, snapshot)

		for k := -d; k <= d; k += 2 {
			var x int
			// Step down from diagonal k+1 (an insertion) or right from k-1
			// (a deletion), whichever got further.
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k

			// Then follow any run of matching lines.
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}

			v[offset+k] = x

			if x >= n && y >= m {
				break search
			}
		}
	}

	// Walk back through the trace. The diagonal steps are the matched lines,
	// collected here in reverse.
	var rev []string
	x, y := n, m
	for d := len(trace) - 1; d >= 0; d-- {
		vd := trace[d]
		k := x - y

		var prevK int
		if k == -d || (k != d && vd[offset+k-1] < vd[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := vd[offset+prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			rev = append(rev, a[x-1])
			x--
			y--
		}
		x, y = prevX, prevY
	}

	for l, r := 0, len(rev)-1; l < r; l, r = l+1, r-1 {
		rev[l], rev[r] = rev[r], rev[l]
	}
	return rev
}
