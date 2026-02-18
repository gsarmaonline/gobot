package llm

import (
	"fmt"
	"sync"
)

// Router selects the appropriate LLM provider based on agent configuration.
type Router struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

func NewRouter() *Router {
	return &Router{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the router.
func (r *Router) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Chat routes a request to the specified provider.
func (r *Router) Chat(providerName string, req *Request) (*Response, error) {
	r.mu.RLock()
	provider, ok := r.providers[providerName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown LLM provider: %s", providerName)
	}

	if !provider.Available() {
		return nil, fmt.Errorf("LLM provider %s is not available (missing API key?)", providerName)
	}

	return provider.Chat(req)
}

// AvailableProviders returns the names of all configured providers.
func (r *Router) AvailableProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, provider := range r.providers {
		if provider.Available() {
			names = append(names, name)
		}
	}
	return names
}

// GetProvider returns a provider by name.
func (r *Router) GetProvider(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}
