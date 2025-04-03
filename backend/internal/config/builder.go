package config

// Builder helps construct ServerConfig with a fluent interface
type Builder struct {
	config *ServerConfig
}

// NewBuilder creates a new configuration builder
func NewBuilder() *Builder {
	return &Builder{
		config: New(),
	}
}

// WithNodeID sets the node ID
func (b *Builder) WithNodeID(id string) *Builder {
	b.config.Node.ID = id
	return b
}

// WithHTTPAddress sets the HTTP address
func (b *Builder) WithHTTPAddress(addr string) *Builder {
	b.config.HTTP.Address = addr
	return b
}

// WithRaftAddress sets the Raft address
func (b *Builder) WithRaftAddress(addr string) *Builder {
	b.config.Raft.Address = addr
	return b
}

// WithRaftAdvertiseAddress sets the Raft advertise address
func (b *Builder) WithRaftAdvertiseAddress(addr string) *Builder {
	b.config.Raft.AdvertiseAddr = addr
	return b
}

// WithRaftDirectory sets the Raft directory
func (b *Builder) WithRaftDirectory(dir string) *Builder {
	b.config.Raft.Directory = dir
	return b
}

// WithBootstrap sets whether to bootstrap the cluster
func (b *Builder) WithBootstrap(bootstrap bool) *Builder {
	b.config.Raft.Bootstrap = bootstrap
	return b
}

// WithPreWarm sets whether to pre-warm the CEL cache
func (b *Builder) WithPreWarm(preWarm bool) *Builder {
	b.config.Node.PreWarm = preWarm
	return b
}

// Build creates the final configuration
func (b *Builder) Build() (*ServerConfig, error) {
	if err := b.config.Validate(); err != nil {
		return nil, err
	}
	return b.config, nil
}
