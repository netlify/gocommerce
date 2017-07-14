package assetstores

import (
	"fmt"

	"github.com/netlify/gocommerce/conf"
)

// Store is the interface wrapping an asset store that can sign download URLs.
type Store interface {
	SignURL(string) (string, error)
}

// NewStore creates an asset store based on the provided configuration.
func NewStore(config *conf.Configuration) (Store, error) {
	switch config.Downloads.Provider {
	case "netlify":
		return newNetlifyProvider(config.Downloads.NetlifyToken)
	case "":
		return newNoopProvider()
	default:
		return nil, fmt.Errorf("Unknown asset store provider '%v'", config.Downloads.Provider)
	}
}
