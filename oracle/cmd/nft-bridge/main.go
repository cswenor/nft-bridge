package main

import (
	"context"
	"nft-bridge/internal/algodapi"
	"nft-bridge/internal/config"
	"nft-bridge/internal/indexerapi"
	workers "nft-bridge/internal/workers/algorand"
	"os"
	"os/signal"
	"syscall"

	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stderr)
	log.SetLevel(log.InfoLevel)
}

func main() {
	slog := log.StandardLogger()

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Error("Failed to load configuration")
		return
	}

	// Make us a nice cancellable context
	// Set Ctrl-C as the cancell trigger
	ctx, cf := context.WithCancel(context.Background())
	defer cf()
	{
		cancelCh := make(chan os.Signal, 1)
		signal.Notify(cancelCh, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			<-cancelCh
			log.Error("Stopping due to signal")
			cf()
		}()
	}

	// Initialize the Algorand client
	algoClient, err := algodapi.Make(ctx, &cfg.ChainAPIs.Algorand.Algod, slog)
	if err != nil {
		log.WithError(err).Error("Failed to make Algorand algod client")
		return
	}

	// // Initialize the Indexer client for Algorand
	indexerClient, err := indexerapi.Make(ctx, &cfg.ChainAPIs.Algorand.Indexer, slog)
	if err != nil {
		log.WithError(err).Error("Failed to make Algorand indexer client")
		return
	}

	// Convert the mnemonic to a private key
	algorandMnemonicKey := cfg.PKeys.Algorand
	algorandPrivateKey, err := mnemonic.ToPrivateKey(algorandMnemonicKey)
	if err != nil {
		log.WithError(err).Error("Failed to convert mnemonic to private key")
		return
	}

	// Derive address from the Algorand private key (pkey)
	algoAccount, err := crypto.AccountFromPrivateKey(algorandPrivateKey)
	if err != nil {
		log.WithError(err).Error("Failed to derive account from private key")
		return
	}

	// Convert the mnemonic to a private key
	voiMnemonicKey := cfg.PKeys.Algorand
	voiPrivateKey, err := mnemonic.ToPrivateKey(voiMnemonicKey)
	if err != nil {
		log.WithError(err).Error("Failed to convert mnemonic to private key")
		return
	}

	// Derive address from the Voi private key (pkey)
	voiAccount, err := crypto.AccountFromPrivateKey(voiPrivateKey)
	if err != nil {
		log.WithError(err).Error("Failed to derive account from private key")
		return
	}

	algoBridge := workers.NewAlgoBridge(algoClient, indexerClient, algoAccount, voiAccount)

	// Start the Worker to Monitor the Transactions
	go algoBridge.StartMonitoring(ctx)

	// Start Processing the Transactions
	go algoBridge.StartProcessing(ctx)

	// Keep the main goroutine alive
	select {}
}
