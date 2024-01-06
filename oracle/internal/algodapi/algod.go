package algodapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"nft-bridge/internal/config"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/sirupsen/logrus"
)

type AlgodAPI struct {
	cfg    *config.ServiceConfig // Updated type to ServiceConfig
	log    *logrus.Logger
	Client *algod.Client
}

// Updated function signature to accept a ServiceConfig pointer
func Make(ctx context.Context, acfg *config.ServiceConfig, log *logrus.Logger) (*AlgodAPI, error) {

	// Create an algod client
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

	algodClient, err := algod.MakeClientWithTransport(acfg.Address, acfg.Token, nil, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to make algod client: %s", err)
	}

	return &AlgodAPI{
		cfg:    acfg,
		log:    log,
		Client: algodClient,
	}, nil
}
