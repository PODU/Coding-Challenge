package ustar

import "fmt"

// FitsUSTARName reports whether name fits in the USTAR name and prefix fields.
// If not, the caller needs a GNU 'L' long-name record.
func FitsUSTARName(name string) bool {
	_, _, err := splitName(name)
	return err == nil
}

// splitName splits a path into the USTAR name (<=100) and prefix (<=155)
// fields at a '/', so that prefix+"/"+name gives back the original.
func splitName(name string) (nameField, prefixField string, err error) {
	if len(name) <= sizeName {
		return name, "", nil
	}
	if len(name) > sizeName+sizePrefix+1 {
		return "", "", fmt.Errorf("ustar: name %q too long for USTAR fields", name)
	}
	// Take the earliest '/' that leaves a suffix short enough for the name
	// field, which puts as much as possible into the prefix.
	for i := len(name) - sizeName - 1; i < len(name); i++ {
		if name[i] != '/' {
			continue
		}
		prefix := name[:i]
		suffix := name[i+1:]
		if len(prefix) <= sizePrefix && len(prefix) > 0 && len(suffix) <= sizeName && len(suffix) > 0 {
			return suffix, prefix, nil
		}
	}
	return "", "", fmt.Errorf("ustar: cannot split name %q on a '/' boundary into USTAR fields", name)
}
