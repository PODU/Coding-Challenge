// Package ustar encodes and decodes POSIX USTAR 512-byte header blocks. It
// works on bytes only and does not use archive/tar.
package ustar

// BlockSize is the size of every USTAR block, in bytes.
const BlockSize = 512

// USTAR magic and version, written at offsets 257 and 263.
const (
	Magic   = "ustar\x00"
	Version = "00"
)

// Type flags (header byte at offset 156).
const (
	TypeReg     = '0'    // regular file
	TypeRegA    = '\x00' // regular file (legacy alias for '0')
	TypeLink    = '1'    // hard link
	TypeSymlink = '2'    // symbolic link
	TypeChar    = '3'    // character device
	TypeBlock   = '4'    // block device
	TypeDir     = '5'    // directory
	TypeFifo    = '6'    // FIFO
	TypeCont    = '7'    // contiguous file
	TypeGNULong = 'L'    // GNU long-name extension (body holds the real name)
)

// Field offsets and widths within the 512-byte header block.
const (
	offName      = 0
	sizeName     = 100
	offMode      = 100
	sizeMode     = 8
	offUID       = 108
	sizeUID      = 8
	offGID       = 116
	sizeGID      = 8
	offSize      = 124
	sizeSize     = 12
	offMtime     = 136
	sizeMtime    = 12
	offChksum    = 148
	sizeChksum   = 8
	offTypeflag  = 156
	offLinkname  = 157
	sizeLinkname = 100
	offMagic     = 257
	sizeMagic    = 6
	offVersion   = 263
	sizeVersion  = 2
	offUname     = 265
	sizeUname    = 32
	offGname     = 297
	sizeGname    = 32
	offDevmajor  = 329
	sizeDevmajor = 8
	offDevminor  = 337
	sizeDevminor = 8
	offPrefix    = 345
	sizePrefix   = 155
)

// GNULongName is the synthetic member name GNU tar uses for 'L' records.
const GNULongName = "././@LongLink"
