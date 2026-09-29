package dbm

import (
	"fmt"
	"io"
	"os"
)

// Stats describes the physical state of a database.
type Stats struct {
	Path     string
	FileSize int64
	PageSize int
	Hash     string

	Pages          uint64 // every page including the header
	DirectoryPages uint64
	BucketPages    uint64 // bucket head pages (= number of buckets)
	OverflowPages  uint64
	FreePages      uint64

	GlobalDepth      int
	DirectorySlots   uint64
	MinLocalDepth    int
	MaxLocalDepth    int
	LongestChain     int // pages in the longest bucket chain
	OverflowBuckets  uint64
	Keys             uint64 // including expired-but-unpurged entries
	UsedBytes        uint64 // entry bytes stored in bucket/overflow pages
	CapacityBytes    uint64 // payload capacity of bucket/overflow pages
	AvgKeysPerBucket float64
}

// Fill is UsedBytes / CapacityBytes.
func (s Stats) Fill() float64 {
	if s.CapacityBytes == 0 {
		return 0
	}
	return float64(s.UsedBytes) / float64(s.CapacityBytes)
}

// Write prints the stats as an aligned table.
func (s Stats) Write(w io.Writer) {
	row := func(k string, v any) { fmt.Fprintf(w, "%-20s %v\n", k+":", v) }
	row("file", s.Path)
	row("file size", fmt.Sprintf("%d bytes", s.FileSize))
	row("page size", s.PageSize)
	row("hash", s.Hash)
	row("keys", s.Keys)
	row("global depth", s.GlobalDepth)
	row("directory slots", s.DirectorySlots)
	row("directory pages", s.DirectoryPages)
	row("buckets", s.BucketPages)
	row("local depth", fmt.Sprintf("%d..%d", s.MinLocalDepth, s.MaxLocalDepth))
	row("overflow pages", s.OverflowPages)
	row("overflow buckets", s.OverflowBuckets)
	row("longest chain", fmt.Sprintf("%d pages", s.LongestChain))
	row("free pages", s.FreePages)
	row("total pages", s.Pages)
	row("keys per bucket", fmt.Sprintf("%.2f", s.AvgKeysPerBucket))
	row("data bytes", s.UsedBytes)
	row("data capacity", s.CapacityBytes)
	row("fill", fmt.Sprintf("%.1f%%", 100*s.Fill()))
}

// Stats scans every page and reports the database's shape.
func (db *DB) Stats() (Stats, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return Stats{}, err
	}
	st := Stats{
		Path:           db.path,
		PageSize:       db.p.pageSize,
		Hash:           db.HashID().String(),
		Pages:          db.p.numPages,
		GlobalDepth:    int(db.hdr.globalDepth),
		DirectorySlots: uint64(len(db.dir)),
		Keys:           db.hdr.numKeys,
		MinLocalDepth:  -1,
	}
	if fi, err := os.Stat(db.path); err == nil {
		st.FileSize = fi.Size()
	}
	payload := uint64(db.p.payload())
	var heads []uint64
	for n := uint64(1); n < db.p.numPages; n++ {
		hdr, err := db.p.readHdr(n)
		if err != nil {
			return st, err
		}
		switch hdr.typ {
		case pageFree:
			st.FreePages++
		case pageDir:
			st.DirectoryPages++
		case pageBucket:
			st.BucketPages++
			st.UsedBytes += uint64(hdr.used)
			st.CapacityBytes += payload
			d := int(hdr.depth)
			if st.MinLocalDepth < 0 || d < st.MinLocalDepth {
				st.MinLocalDepth = d
			}
			if d > st.MaxLocalDepth {
				st.MaxLocalDepth = d
			}
			if hdr.next != 0 {
				st.OverflowBuckets++
				heads = append(heads, n)
			}
		case pageOverflow:
			st.OverflowPages++
			st.UsedBytes += uint64(hdr.used)
			st.CapacityBytes += payload
		default:
			return st, corrupt("page %d has unknown type %d", n, hdr.typ)
		}
	}
	if st.MinLocalDepth < 0 {
		st.MinLocalDepth = 0
	}
	st.LongestChain = 1
	for _, h := range heads {
		n, length := h, 0
		for n != 0 && length <= int(db.p.numPages) {
			hdr, err := db.p.readHdr(n)
			if err != nil {
				return st, err
			}
			length++
			n = hdr.next
		}
		if length > st.LongestChain {
			st.LongestChain = length
		}
	}
	if st.BucketPages > 0 {
		st.AvgKeysPerBucket = float64(st.Keys) / float64(st.BucketPages)
	}
	return st, nil
}
