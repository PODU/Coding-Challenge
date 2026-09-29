package dbm

import (
	"fmt"
	"os"
)

// ReorgResult reports what Reorganise did.
type ReorgResult struct {
	Keys          uint64
	Expired       uint64 // entries dropped because they had expired
	BytesBefore   int64
	BytesAfter    int64
	PagesBefore   uint64
	PagesAfter    uint64
	FreedPages    uint64 // free-list pages the old file carried
	OverflowPages uint64 // overflow pages the old file carried
}

// Reorganise rebuilds the database at path into a compact file: every live
// entry is inserted into a fresh database with the same page size and
// hash function, then the new file replaces the old one. Free pages,
// overflow chains left by deleted entries and expired entries all
// disappear. The database must not be open by this process.
func Reorganise(path string) (ReorgResult, error) {
	var res ReorgResult
	src, err := Open(path, Options{ReadOnly: true})
	if err != nil {
		return res, err
	}
	stBefore, err := src.Stats()
	if err != nil {
		src.Close()
		return res, err
	}
	res.BytesBefore = stBefore.FileSize
	res.PagesBefore = stBefore.Pages
	res.FreedPages = stBefore.FreePages
	res.OverflowPages = stBefore.OverflowPages

	tmp := path + ".reorg"
	os.Remove(tmp)
	dst, err := Create(tmp, Options{PageSize: src.PageSize(), Hash: src.HashID(), NoSync: true})
	if err != nil {
		src.Close()
		return res, err
	}
	fail := func(err error) (ReorgResult, error) {
		dst.Close()
		src.Close()
		os.Remove(tmp)
		return res, err
	}
	var putErr error
	res.Expired = src.Len()
	err = src.Each(func(k, v []byte, exp uint64) bool {
		if putErr = dst.PutExpires(k, v, exp); putErr != nil {
			return false
		}
		res.Keys++
		return true
	})
	if err != nil {
		return fail(err)
	}
	if putErr != nil {
		return fail(putErr)
	}
	res.Expired -= res.Keys
	stAfter, err := dst.Stats()
	if err != nil {
		return fail(err)
	}
	res.PagesAfter = stAfter.Pages
	if err := dst.Close(); err != nil {
		src.Close()
		os.Remove(tmp)
		return res, err
	}
	if err := src.Close(); err != nil {
		os.Remove(tmp)
		return res, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return res, fmt.Errorf("dbm: replace %s: %w", path, err)
	}
	if fi, err := os.Stat(path); err == nil {
		res.BytesAfter = fi.Size()
	}
	return res, nil
}
