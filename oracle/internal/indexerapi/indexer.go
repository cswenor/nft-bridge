package indexerapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"nft-bridge/internal/config"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/indexer"
	"github.com/sirupsen/logrus"
)

type IndexerAPI struct {
	cfg    *config.ServiceConfig // Configuration for the Indexer service
	log    *logrus.Logger
	Client *indexer.Client
}

// Make initializes a new IndexerAPI client with the given configuration.
func Make(ctx context.Context, icfg *config.ServiceConfig, log *logrus.Logger) (*IndexerAPI, error) {

	// Create an indexer client
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		MaxIdleConnsPerHost:   100,
		MaxIdleConns:          100,
	}

	indexerClient, err := indexer.MakeClientWithTransport(icfg.Address, icfg.Token, nil, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to make indexer client: %s", err)
	}

	return &IndexerAPI{
		cfg:    icfg,
		log:    log,
		Client: indexerClient,
	}, nil
}
