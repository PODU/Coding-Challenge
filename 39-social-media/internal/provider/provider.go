// Package provider publishes posts to social networks: LinkedIn, X, and a Mock
// for development and tests.
package provider

import (
	"context"
	"errors"
	"fmt"

	"socialmedia/internal/model"
)

// ErrAuth means the network rejected our credentials. Retrying won't help.
var ErrAuth = errors.New("provider: authentication failed")

type Provider interface {
	// Name is the value of Post.Target for this network, e.g. "linkedin".
	Name() string

	// Validate applies the network's own limits without contacting it.
	// Failures wrap model.ErrValidation.
	Validate(p model.Post) error

	// Publish posts p and returns the network's ID for it.
	Publish(ctx context.Context, p model.Post) (externalID string, err error)
}

// Registry finds the Provider for a post's target.
type Registry struct {
	providers map[string]Provider
}

func NewRegistry(ps ...Provider) *Registry {
	r := &Registry{providers: make(map[string]Provider, len(ps))}
	for _, p := range ps {
		r.providers[p.Name()] = p
	}
	return r
}

// Get returns the Provider for target. An unknown target is a validation error.
func (r *Registry) Get(target string) (Provider, error) {
	p, ok := r.providers[target]
	if !ok {
		return nil, fmt.Errorf("%w: unknown target %q", model.ErrValidation, target)
	}
	return p, nil
}

// Names returns the registered targets in no particular order.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.providers))
	for name := range r.providers {
		out = append(out, name)
	}
	return out
}
