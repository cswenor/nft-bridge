package algodapi

import (
	"context"
	"fmt"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
)

func (a *AlgodAPI) OptIntoAsset(ctx context.Context, assetID uint64, account crypto.Account) (string, error) {

	txParams, err := a.Client.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Failed to get suggested tx params: %s\n", err)
		return "", err
	}
	// Construct the transaction
	tx, err := transaction.MakeAssetTransferTxn(account.Address.String(), account.Address.String(), 0, nil, txParams, "", assetID)
	if err != nil {
		fmt.Printf("Failed to make asset transfer txn: %s\n", err)
		return "", err
	}

	// Sign the transaction
	txID, signedTx, err := crypto.SignTransaction(account.PrivateKey, tx)
	if err != nil {
		fmt.Printf("Failed to sign transaction: %s\n", err)
		return "", err
	}

	// Send the transaction
	txID, err = a.Client.SendRawTransaction(signedTx).Do(context.Background())
	if err != nil {
		fmt.Printf("Failed to send transaction: %s\n", err)
		return "", err
	}

	fmt.Printf("Submitted Opt-In Transaction with ID: %s\n", txID)

	// Wait for the transaction to be confirmed
	err = a.waitForConfirmation(ctx, txID)
	if err != nil {
		return "", fmt.Errorf("waiting for confirmation failed: %w", err)
	}

	return txID, nil
}

func (a *AlgodAPI) waitForConfirmation(ctx context.Context, txID string) error {
	for {
		// Check the status of the transaction
		txInfo, _, err := a.Client.PendingTransactionInformation(txID).Do(ctx)
		if err != nil {
			fmt.Printf("Error getting pending transaction: %s\n", err)
			return err
		}

		if txInfo.ConfirmedRound > 0 {
			// Transaction has been confirmed in this round
			fmt.Printf("Transaction %s confirmed in round %d\n", txID, txInfo.ConfirmedRound)
			return nil
		}

		// Wait a bit before checking again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			// Continue to the next iteration
		}
	}
}
