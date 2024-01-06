package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/types"
)

func (b *AlgoBridge) StartProcessing(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return // No need to close the channel here, it should be closed by the sender
		case txn, ok := <-b.TxnChannel:
			if !ok {
				return // Channel was closed, stop processing
			}
			b.marshalTransaction(txn) // Process each transaction
		default:
			// Sleep for a short duration before checking again
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (b *AlgoBridge) marshalTransaction(txn models.Transaction) {

	// Check to see if it is an asset transfer and if it is process the NFT
	if txn.Type == "axfer" && txn.AssetTransferTransaction.Amount > 0 {
		assetID := txn.AssetTransferTransaction.AssetId
		// Check if the asset ID exists in the store and process it
		if nft, exists := b.nftStore[assetID]; exists {
			b.processNFT(&nft)
		} else {
			fmt.Printf("No NFT found for asset ID %d\n", assetID)
		}
		return
	}
	noteStr := string(txn.Note)
	var rawNote rawNoteObj
	if err := json.Unmarshal([]byte(noteStr), &rawNote); err != nil {
		fmt.Printf("Note is not a valid JSON object: %s\n", err)
		return
	}

	// Convert the To field from string to types.Address
	var note noteObj
	var err error
	note.To, err = types.DecodeAddress(rawNote.To)
	if err != nil {
		fmt.Printf("Invalid 'To' address: %s\n", err)
		return
	}
	note.AssetID = rawNote.AssetID
	note.Amount = rawNote.Amount

	// Assuming note.AssetID is used as the key for nftStore
	b.mu.Lock() // Ensure thread-safe access to nftStore
	if existingNFT, exists := b.nftStore[note.AssetID]; exists {
		// If an NFT with this AssetID already exists, report its state and error out
		// Assuming the list always has at least one element if it exists
		b.mu.Unlock()
		fmt.Printf("Error: The NFT with AssetID %d is already being processed and it is in %s state\n", note.AssetID, existingNFT.State)
		return
	}
	b.mu.Unlock()

	// Fetch the asset details from the Algorand network directly
	assetDetails, err := b.algodClient.Client.GetAssetByID(uint64(note.AssetID)).Do(context.Background())
	if err != nil {
		fmt.Printf("Failed to fetch asset details: %s\n", err)
		return
	}

	// Determine the spec based on asset details
	var spec SpecType
	assetURL := assetDetails.Params.Url
	assetName := assetDetails.Params.Name

	if isARC3(assetURL, assetName) {
		spec = ARC3
	} else {
		fmt.Printf("Error: The NFT with AssetID %d is not a valid spec", note.AssetID)
		return
	}

	// Create a BridgedNFT object with the parsed note information
	bridgedNFT, err := NewBridgedNFT(
		Algorand,     // Assuming the ChainOfOrigin is always Algorand for this use case
		Expect,       // Assuming the initial State is always Expect
		note.To,      // The 'To' address from the note
		note.AssetID, // Using AssetID as a unique identifier
		txn.Sender,
	)
	if err != nil {
		fmt.Printf("Failed to create BridgedNFT: %s\n", err)
		return
	}

	// Set the spec and asset URL
	bridgedNFT.Spec = spec
	bridgedNFT.AssetURL = assetDetails.Params.Url

	// Add the transaction to the BridgedNFT object for easy access
	bridgedNFT.Transaction = txn

	// Store or process the BridgedNFT object as needed
	// For example, add it to the nftStore (you need to handle concurrency and check for duplicates)

	b.mu.Lock()
	b.nftStore[note.AssetID] = *bridgedNFT
	b.mu.Unlock()

	b.processNFT(bridgedNFT)
}

// isARC3 determines if the asset adheres to the ARC3 spec based on the provided rules.
func isARC3(assetURL, assetName string) bool {

	// Check if the Asset Name is 'arc3' or ends with '@arc3'
	if assetName != "arc3" && !strings.HasSuffix(assetName, "@arc3") && !strings.HasSuffix(assetURL, "#arc3") {
		return false
	}

	// Check if the Asset URL ends with '#arc3'
	if strings.HasSuffix(assetURL, "#arc3") {
		// URL should be valid and point to the same resource with or without '#arc3'
		return true
	}

	// Add any other necessary checks based on the ARC3 specification

	return true
}

func (b *AlgoBridge) processNFT(nft *BridgedNFT) {
	// Fetch account information
	accountInfo, err := b.algodClient.GetAccountInfo(context.Background(), b.algoAccount.Address.String())
	if err != nil {
		fmt.Printf("Failed to fetch account information: %s\n", err)
		return
	}

	// Check if the account is opted into the asset
	var optedIn bool
	var assetBalance uint64
	for _, asset := range accountInfo.Assets {
		if asset.AssetId == nft.AssetID {
			optedIn = true
			assetBalance = asset.Amount
			break
		}
	}

	// If not opted in, opt into the asset and set state to Prepared
	if !optedIn {
		txID, err := b.algodClient.OptIntoAsset(context.Background(), nft.AssetID, b.algoAccount)
		if err != nil {
			fmt.Printf("Failed to opt into asset: %s\n", err)
			return
		}
		nft.State = Prepared
		fmt.Printf("Opted into asset: %v with transaction ID: %v\n", nft.AssetID, txID)

		// Update the NFT in the store
		b.updateNFTStore(nft)
		return
	} else {
		nft.State = Prepared
		b.updateNFTStore(nft)
	}

	// If already opted in and balance >= 1, set state to Received
	if assetBalance >= 1 {
		nft.State = Received
		// Update the NFT in the store
		b.updateNFTStore(nft)
	}
}