package main

import (
	"context"
	"fmt"
	"nft-bridge/internal/algodapi"
	"nft-bridge/internal/config"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/algorand/go-algorand-sdk/crypto"
	"github.com/algorand/go-algorand-sdk/mnemonic"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/types"

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
	// indexerClient, err := indexerapi.Make(ctx, &cfg.Algorand.Indexer, slog)
	// if err != nil {
	// 	log.WithError(err).Error("Failed to make Algorand indexer client")
	// 	return
	// }

	// Convert the mnemonic to a private key
	mnemonicKey := cfg.PKeys.Algorand
	privateKey, err := mnemonic.ToPrivateKey(mnemonicKey)
	if err != nil {
		log.WithError(err).Error("Failed to convert mnemonic to private key")
		return
	}

	// Derive address from the Algorand private key (pkey)
	account, err := crypto.AccountFromPrivateKey(privateKey)
	if err != nil {
		log.WithError(err).Error("Failed to derive account from private key")
		return
	}

	// Define the specific wallet address to monitor
	walletAddress := account.Address.String()

	// Start the monitoring process in a new goroutine
	go monitorTransactions(ctx, algoClient, walletAddress)

	// Keep the main goroutine alive
	select {}
}

// monitorTransactions continuously checks for transactions to the specified wallet.
func monitorTransactions(ctx context.Context, api *algodapi.AlgodAPI, address string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Fetch account information
			accountInfo, err := api.GetAccountInfo(ctx, address)
			if err != nil {
				log.WithError(err).Error("Failed to get account information")
				// Decide how you want to handle the error. Maybe continue with a delay?
				time.Sleep(10 * time.Second)
				continue
			}

			log.Infof("Monitoring transactions for account: %+v", accountInfo)

			// Implement the logic to fetch and check transactions
			// Example: transactions, err := api.Client.PendingTransactionsByAddress(address).Do(ctx)
			// You'll need to filter transactions for a payment with a note indicating the asset to opt-in

			// Handle the found transaction and opt-in to the asset
			// Example: if transactionFound { optInToAsset(api.Client, transaction) }

			// Sleep for a short duration before checking again
			time.Sleep(10 * time.Second)
		}
	}
}

// optInToAsset handles the opt-in process for the specified asset
func optInToAsset(client *algod.Client, transaction types.Transaction) {
	// Implement the asset opt-in logic here
	fmt.Println("Opting in to asset:", transaction.Note)
}
