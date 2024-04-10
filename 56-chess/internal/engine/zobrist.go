package engine

// Zobrist keys come from splitmix64 with a fixed seed, so hashes are the same
// on every run.

var (
	zPiece    [16][128]uint64 // [piece][square]
	zSide     uint64          // XORed in when Black is to move
	zCastling [16]uint64      // by castling-rights mask
	zEnPas    [8]uint64       // by en-passant file
)

func init() {
	var s uint64 = 0x9E3779B97F4A7C15
	next := func() uint64 {
		s += 0x9E3779B97F4A7C15
		z := s
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		return z ^ (z >> 31)
	}
	for p := 0; p < 16; p++ {
		for sq := 0; sq < 128; sq++ {
			zPiece[p][sq] = next()
		}
	}
	zSide = next()
	for i := range zCastling {
		zCastling[i] = next()
	}
	for i := range zEnPas {
		zEnPas[i] = next()
	}
}

func (b *Board) computeHash() uint64 {
	var h uint64
	for sq := Square(0); sq < 128; sq++ {
		if !sq.OnBoard() {
			continue
		}
		p := b.squares[sq]
		if p != Empty {
			h ^= zPiece[p][sq]
		}
	}
	if b.side == Black {
		h ^= zSide
	}
	h ^= zCastling[b.castling]
	if b.ep != NoSquare {
		h ^= zEnPas[b.ep.File()]
	}
	return h
}
