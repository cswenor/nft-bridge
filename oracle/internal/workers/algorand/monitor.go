package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/types"
	log "github.com/sirupsen/logrus"
)

type noteObj struct {
	AssetID int64         `json:"assetId"`
	To      types.Address `json:"to"`
	Amount  int64         `json:"amount"`
}

type rawNoteObj struct {
	AssetID int64  `json:"assetId"`
	To      string `json:"to"`
	Amount  int64  `json:"amount"`
}

func (b *AlgoBridge) StartMonitoring(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(b.TxnChannel)
			return
		default:
			b.fetchAndStoreTransactions(ctx)
			// Sleep for a short duration before checking again
			time.Sleep(10 * time.Second)
		}
	}
}

func (b *AlgoBridge) fetchAndStoreTransactions(ctx context.Context) {
	// Use the lastKnownRound to fetch only new transactions
	txns, err := b.indexerClient.Client.SearchForTransactions().
		AddressString(b.algoAccount.Address.String()).
		TxType("pay").
		MinRound(b.lastKnownRound + 1).
		Do(ctx)

	if err != nil {
		log.WithError(err).Error("Failed to fetch transactions")
		return
	}

	// Initialize maxRound to the last known round or the current round from the response
	var maxRound uint64 = b.lastKnownRound
	if txns.CurrentRound > maxRound {
		maxRound = txns.CurrentRound
	}

	// Process the transactions
	for _, txn := range txns.Transactions {
		if err := b.parseAndFilterTransaction(txn); err != nil {
			log.WithError(err).WithField("txnID", txn.Id).Error("Failed to parse and filter transaction")
			continue
		}

		// Update maxRound to the highest round seen
		if txn.ConfirmedRound > maxRound {
			maxRound = txn.ConfirmedRound
		}
	}

	// Update lastKnownRound to the highest round we've seen in this fetch
	b.mu.Lock()
	if maxRound > b.lastKnownRound {
		b.lastKnownRound = maxRound
	}
	b.mu.Unlock()
}

func (b *AlgoBridge) parseAndFilterTransaction(txn models.Transaction) error {
	// 1. Check if it's a payment transaction
	if txn.Type != "pay" {
		return errors.New("not a payment transaction")
	}

	// 2. Check if it has a minimum of .2 Algo as payment
	minAmount := big.NewInt(200000) // .2 Algo in microAlgos
	amount := big.NewInt(0).SetUint64(txn.PaymentTransaction.Amount)
	if amount.Cmp(minAmount) == -1 {
		return errors.New("transaction amount is less than the minimum required")
	}

	// 3. Check if it has a Note
	noteStr := string(txn.Note)
	var rawNote rawNoteObj
	if err := json.Unmarshal([]byte(noteStr), &rawNote); err != nil {
		return fmt.Errorf("note is not a valid JSON object: %s", err)
	}

	// Convert the To field from string to types.Address
	var note noteObj
	var err error
	note.To, err = types.DecodeAddress(rawNote.To)
	if err != nil {
		return fmt.Errorf("invalid 'To' address: %s", err)
	}
	note.AssetID = rawNote.AssetID
	note.Amount = rawNote.Amount

	// If all checks pass, send the transaction to the channel
	b.TxnChannel <- txn
	return nil
}
