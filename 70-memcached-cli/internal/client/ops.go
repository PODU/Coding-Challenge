package client

import (
	"github.com/codingchallenges/ccmc/internal/protocol"
)

// Store runs a storage command. With sc.NoReply it returns protocol.NoReply
// without waiting for the server.
func (c *Client) Store(sc protocol.StorageCommand) (protocol.StoreStatus, error) {
	if err := c.armDeadline(); err != nil {
		return 0, err
	}
	if err := c.send(protocol.EncodeStorage(sc)); err != nil {
		return 0, err
	}
	if sc.NoReply {
		return protocol.NoReply, nil
	}
	return protocol.ParseStorageReply(c.r)
}

// Get retrieves one or more keys (without CAS).
func (c *Client) Get(keys []string) ([]protocol.Value, error) {
	return c.retrieve("get", keys)
}

// Gets retrieves one or more keys with their CAS unique values.
func (c *Client) Gets(keys []string) ([]protocol.Value, error) {
	return c.retrieve("gets", keys)
}

func (c *Client) retrieve(command string, keys []string) ([]protocol.Value, error) {
	if err := c.armDeadline(); err != nil {
		return nil, err
	}
	if err := c.send(protocol.EncodeRetrieval(command, keys)); err != nil {
		return nil, err
	}
	return protocol.ParseValues(c.r)
}

// Delete removes key and reports whether it existed. With noreply it returns
// true without waiting.
func (c *Client) Delete(key string, noreply bool) (bool, error) {
	if err := c.armDeadline(); err != nil {
		return false, err
	}
	if err := c.send(protocol.EncodeDelete(key, noreply)); err != nil {
		return false, err
	}
	if noreply {
		return true, nil
	}
	return protocol.ParseDeleteReply(c.r)
}

// IncrDecr runs incr or decr and returns the new value.
func (c *Client) IncrDecr(command, key string, delta uint64, noreply bool) (uint64, bool, error) {
	if err := c.armDeadline(); err != nil {
		return 0, false, err
	}
	if err := c.send(protocol.EncodeIncrDecr(command, key, delta, noreply)); err != nil {
		return 0, false, err
	}
	if noreply {
		return 0, true, nil
	}
	return protocol.ParseNumericReply(c.r)
}

// Stats fetches server statistics; arg selects a group and may be empty.
func (c *Client) Stats(arg string) ([]protocol.Stat, error) {
	if err := c.armDeadline(); err != nil {
		return nil, err
	}
	if err := c.send(protocol.EncodeStats(arg)); err != nil {
		return nil, err
	}
	return protocol.ParseStats(c.r)
}
