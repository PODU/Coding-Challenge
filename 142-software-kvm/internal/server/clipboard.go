package server

import "github.com/podu2997/cckvm/internal/protocol"

// Clipboard sharing follows Barrier's ownership model:
//
//  1. A screen whose clipboard changes grabs it: a client sends CCLP, the
//     primary does it locally. The server records the owner and tells every
//     other client with CCLP, so they stop claiming it.
//  2. The owner's data follows: a client sends DCLP when it is left (or at
//     any time), the primary's text is read directly.
//  3. Every screen that has not seen the new data is marked dirty and gets
//     DCLP the next time the cursor enters it. The primary's OS clipboard is
//     updated as soon as the data arrives.

// localClipboardChanged handles a ClipboardChanged event from the primary.
func (s *Server) localClipboardChanged(id uint8) {
	if int(id) >= protocol.NumClipboards {
		return
	}
	text, ok := s.cfg.Screen.Clipboard(id)
	if !ok {
		return
	}
	data := protocol.TextClipboard(text)
	if s.clip[id].data == data {
		return // our own SetClipboard echoing back
	}
	s.clip[id] = clipboard{owner: s.primary.name, seq: s.seq, data: data}
	s.broadcast(protocol.Grab{ID: id, Seq: s.seq}, s.primary.name)
	s.markDirty(id, s.primary.name)
	if s.active != s.primary {
		s.flushClipboards(s.active)
	}
}

// grab handles CCLP from a client.
func (s *Server) grab(sc *screen, id uint8, seq uint32) {
	if int(id) >= protocol.NumClipboards {
		return
	}
	if seq < s.clip[id].seq {
		s.log.Debug("ignoring stale clipboard grab", "screen", sc.name, "seq", seq)
		return
	}
	s.clip[id].owner = sc.name
	s.clip[id].seq = seq
	s.broadcast(protocol.Grab{ID: id, Seq: seq}, sc.name)
}

// clipboardData handles a completed DCLP transfer from a client.
func (s *Server) clipboardData(sc *screen, id uint8, seq uint32, data string) {
	c := &s.clip[id]
	if c.owner != sc.name || seq < c.seq {
		s.log.Debug("ignoring clipboard data from non-owner", "screen", sc.name, "owner", c.owner)
		return
	}
	if c.data == data {
		return
	}
	c.data = data
	formats, err := protocol.UnmarshalClipboard(data)
	if err != nil {
		s.log.Warn("bad clipboard data", "screen", sc.name, "err", err)
		return
	}
	if text, ok := formats[protocol.FormatText]; ok {
		s.cfg.Screen.SetClipboard(id, text)
	}
	s.markDirty(id, sc.name)
	if s.active != s.primary && s.active != sc {
		s.flushClipboards(s.active)
	}
}

func (s *Server) markDirty(id uint8, except string) {
	for _, sc := range s.screens {
		sc.dirty[id] = sc.proxy != nil && sc.name != except
	}
}

// flushClipboards sends sc every clipboard it has not seen yet.
func (s *Server) flushClipboards(sc *screen) {
	for id := range s.clip {
		if !sc.dirty[id] || s.clip[id].data == "" {
			continue
		}
		sc.dirty[id] = false
		for _, m := range protocol.ChunkClipboard(uint8(id), s.seq, s.clip[id].data) {
			s.sendTo(sc, m)
		}
	}
}
