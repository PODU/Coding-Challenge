package object

// Reachable is a collected object ready to be packed.
type Reachable struct {
	OID     string
	Type    Type
	Payload []byte
}

// CollectReachable returns every commit, tree and blob reachable from tips
// but not from have, i.e. what the remote is missing, ready for BuildPack.
func CollectReachable(gitDir string, tips []string, have map[string]bool) ([]Reachable, error) {
	seen := map[string]bool{}

	// Everything reachable from a have is already on the remote.
	for h := range have {
		if h == "" {
			continue
		}
		if err := markReachable(gitDir, h, seen); err != nil {
			// The remote may have commits we don't; ignore those.
			continue
		}
	}

	var out []Reachable
	collected := map[string]bool{}
	var visit func(oid string, typ Type) error
	visit = func(oid string, typ Type) error {
		if oid == "" || collected[oid] || seen[oid] {
			return nil
		}
		t, payload, err := Read(gitDir, oid)
		if err != nil {
			return err
		}
		collected[oid] = true
		out = append(out, Reachable{OID: oid, Type: t, Payload: payload})

		switch t {
		case TypeCommit:
			c, err := DecodeCommit(payload)
			if err != nil {
				return err
			}
			if err := visit(c.Tree, TypeTree); err != nil {
				return err
			}
			for _, p := range c.Parents {
				if err := visit(p, TypeCommit); err != nil {
					return err
				}
			}
		case TypeTree:
			entries, err := DecodeTree(payload)
			if err != nil {
				return err
			}
			for _, e := range entries {
				childType := TypeBlob
				if e.Mode == ModeTree {
					childType = TypeTree
				}
				if err := visit(e.OID, childType); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, tip := range tips {
		if err := visit(tip, TypeCommit); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// markReachable adds every object reachable from a commit OID to seen.
func markReachable(gitDir, oid string, seen map[string]bool) error {
	if oid == "" {
		return nil
	}
	t, payload, err := Read(gitDir, oid)
	if err != nil {
		return err
	}
	if seen[oid] {
		return nil
	}
	seen[oid] = true
	switch t {
	case TypeCommit:
		c, err := DecodeCommit(payload)
		if err != nil {
			return err
		}
		_ = markReachable(gitDir, c.Tree, seen)
		for _, p := range c.Parents {
			_ = markReachable(gitDir, p, seen)
		}
	case TypeTree:
		entries, err := DecodeTree(payload)
		if err != nil {
			return err
		}
		for _, e := range entries {
			_ = markReachable(gitDir, e.OID, seen)
		}
	}
	return nil
}
