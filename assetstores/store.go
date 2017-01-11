package assetstores

import (
	"fmt"

	"github.com/netlify/netlify-commerce/conf"
)

type Store interface {
	SignURL(string) (string, error)
}

func NewStore(config *conf.Configuration) (Store, error) {
	switch config.Downloads.Provider {
	case "netlify":
		return NewNetlifyProvider(config.Downloads.NetlifyToken)
	case "":
		return NewNOOPProvider()
	default:
		return nil, fmt.Errorf("Unknown asset store provider '%v'", config.Downloads.Provider)
	}
}
