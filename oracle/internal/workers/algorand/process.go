package workers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/abi"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
)

var appId uint64 = 26169081

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
			time.Sleep(1000 * time.Millisecond)
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
	assetDetails, err := b.algoAlgodClient.Client.GetAssetByID(uint64(note.AssetID)).Do(context.Background())
	if err != nil {
		fmt.Printf("Failed to fetch asset details: %s\n", err)
		return
	}

	// Determine the spec based on asset details
	var spec SpecType
	assetURL := assetDetails.Params.Url
	assetName := assetDetails.Params.Name
	assetReserveStr := assetDetails.Params.Reserve

	assetReserve, err := types.DecodeAddress(assetReserveStr)
	if err != nil {
		fmt.Printf("Failed to decode address: %s\n", err)
		return
	}

	//TODO: Figure out ARC19/ARC3/ARC69 differences
	isArc19, parsedURL := isARC19(assetURL, assetReserve)
	if isArc19 {
		spec = ARC19
	} else if isARC3(assetURL, assetName) {
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
	bridgedNFT.AssetURL = parsedURL

	// Add the transaction to the BridgedNFT object for easy access
	bridgedNFT.Transaction = txn

	// Store or process the BridgedNFT object as needed
	// For example, add it to the nftStore (you need to handle concurrency and check for duplicates)

	b.mu.Lock()
	b.nftStore[note.AssetID] = *bridgedNFT
	b.mu.Unlock()

	fmt.Printf("Successfully created and processed BridgedNFT with AssetID: %d, Spec: %s, and State: %s\n", bridgedNFT.AssetID, bridgedNFT.Spec, bridgedNFT.State)

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

func isARC19(assetURL string, assetReserve types.Address) (bool, string) {
	parsedURL, err := ParseASAUrl(assetURL, assetReserve)
	if err != nil || parsedURL == "" {
		// It's not ARC-19 or there was an error in parsing
		return false, ""
	}
	// It's ARC-19 and here's the parsed URL
	return true, parsedURL
}

func (b *AlgoBridge) processNFT(nft *BridgedNFT) {
	// Fetch account information
	accountInfo, err := b.algoAlgodClient.GetAccountInfo(context.Background(), b.algoAccount.Address.String())
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
		txID, err := b.algoAlgodClient.OptIntoAsset(context.Background(), nft.AssetID, b.algoAccount)
		if err != nil {
			fmt.Printf("Failed to opt into asset: %s\n", err)
			return
		}
		//TODO: Need to implement that State can not be skipped. You MUST go in order.
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

	if nft.State == Received {
		if b.nftExists(strconv.FormatUint(nft.AssetID, 10)) {
			nft.State = Sent
			b.updateNFTStore(nft)
			return
		}
		err = b.mintNFT(nft.To.String(), []byte(nft.AssetURL), strconv.FormatUint(nft.AssetID, 10))
		if err != nil {
			fmt.Printf("Failed to mint asset: %s\n", err)
			return
		}
		nft.State = Sent

		b.updateNFTStore(nft)
	}

}

var (
	ErrUnknownSpec      = errors.New("unsupported template-ipfs spec")
	ErrUnsupportedField = errors.New("unsupported ipfscid field, only reserve is currently supported")
	ErrUnsupportedCodec = errors.New("unknown multicodec type in ipfscid spec")
	ErrUnsupportedHash  = errors.New("unknown hash type in ipfscid spec")
	ErrInvalidV0        = errors.New("cid v0 must always be dag-pb and sha2-256 codec/hash type")
	ErrHashEncoding     = errors.New("error encoding new hash")
	templateIPFSRegexp  = regexp.MustCompile(`template-ipfs://{ipfscid:(?P<version>[01]):(?P<codec>[a-z0-9\-]+):(?P<field>[a-z0-9\-]+):(?P<hash>[a-z0-9\-]+)}`)
)

func ParseASAUrl(asaUrl string, reserveAddress types.Address) (string, error) {
	matches := templateIPFSRegexp.FindStringSubmatch(asaUrl)
	if matches == nil {
		if strings.HasPrefix(asaUrl, "template-ipfs://") {
			return "", ErrUnknownSpec
		}
		return asaUrl, nil
	}
	if matches[templateIPFSRegexp.SubexpIndex("field")] != "reserve" {
		return "", ErrUnsupportedField
	}
	var (
		codec         multicodec.Code
		multihashType uint64
		hash          []byte
		err           error
		cidResult     cid.Cid
	)
	if err = codec.Set(matches[templateIPFSRegexp.SubexpIndex("codec")]); err != nil {
		return "", ErrUnsupportedCodec
	}
	multihashType = multihash.Names[matches[templateIPFSRegexp.SubexpIndex("hash")]]
	if multihashType == 0 {
		return "", ErrUnsupportedHash
	}

	hash, err = multihash.Encode(reserveAddress[:], multihashType)
	if err != nil {
		return "", ErrHashEncoding
	}
	if matches[templateIPFSRegexp.SubexpIndex("version")] == "0" {
		if codec != multicodec.DagPb {
			return "", ErrInvalidV0
		}
		if multihashType != multihash.SHA2_256 {
			return "", ErrInvalidV0
		}
		cidResult = cid.NewCidV0(hash)
	} else {
		cidResult = cid.NewCidV1(uint64(codec), hash)
	}
	return fmt.Sprintf("ipfs://%s", strings.ReplaceAll(asaUrl, matches[0], cidResult.String())), nil
}

func (b *AlgoBridge) nftExists(assetID string) bool {
	// Load the ABI method signature for mintTo
	method, _ := abi.MethodFromSignature("arc72_ownerOf(uint256)address")

	convertedAssetID, _ := strconv.ParseUint(assetID, 10, 64)
	assetIDTypeObj, _ := method.Args[0].GetTypeObject()
	encodedAssetIDUINT256, _ := assetIDTypeObj.Encode(convertedAssetID)

	args := [][]byte{method.GetSelector(), encodedAssetIDUINT256}

	sp, _ := b.voiAlgodClient.Client.SuggestedParams().Do(context.Background())

	// Construct the transaction
	simtx, _ := transaction.MakeApplicationCallTxWithBoxes(
		appId,                // appID
		args,                 // appArgs
		nil,                  // accounts
		nil,                  // foreignApps
		nil,                  // foreignAssets
		nil,                  // appBoxReference
		types.NoOpOC,         // onCompletion
		nil,                  // note
		nil,                  // lease
		types.StateSchema{},  // localStateSchema
		types.StateSchema{},  // globalStateSchema
		0,                    // fee
		sp,                   // suggestedParams
		b.voiAccount.Address, // sender
		nil,                  // rekeyTo
		types.Digest{},       // approvalProgram
		[32]byte{},           // clearProgram
		b.voiAccount.Address, // rekeyTo (again)
	)

	// Create a SimulateRequest
	simulateRequest := models.SimulateRequest{
		AllowEmptySignatures:  true,
		AllowUnnamedResources: true,
		TxnGroups: []models.SimulateRequestTransactionGroup{
			{
				Txns: []types.SignedTxn{{Txn: simtx}}, // Include your unsigned transaction here
			},
		},
	}

	// Simulate the transaction
	simulateResponse, _ := b.voiAlgodClient.Client.SimulateTransaction(simulateRequest).Do(context.Background())

	owner := hex.EncodeToString([]byte(simulateResponse.TxnGroups[0].TxnResults[0].TxnResult.Logs[0]))

	if owner != "151f7c750000000000000000000000000000000000000000000000000000000000000000" {
		fmt.Printf("NFT with AssetID %s exists on Voi. Owner: %s\n", assetID, owner)
		return true
	} else {
		fmt.Printf("NFT with AssetID %s does not exist on Voi.\n", assetID)
		return false
	}

}

func (b *AlgoBridge) mintNFT(receiverAddress string, metadata []byte, assetID string) error {

	const microAlgosToSend = 249300 * 2 // 0.2 Algo in microAlgos

	// Load the ABI method signature for mintTo
	method, err := abi.MethodFromSignature("mintTo(address,byte[256],uint256,byte[256],uint64)uint256")
	if err != nil {
		return fmt.Errorf("failed to load method signature: %v", err)
	}

	// Receiver Address
	addr, _ := types.DecodeAddress(receiverAddress)
	receiverAddressTypeObj, _ := method.Args[0].GetTypeObject()
	encodedReceiverAddress, _ := receiverAddressTypeObj.Encode(addr)

	// Metadata URI
	metadataTypeObj, _ := method.Args[1].GetTypeObject()
	paddedMetadata := make([]byte, 256)
	copy(paddedMetadata, metadata)
	encodedMetadata, _ := metadataTypeObj.Encode(paddedMetadata)

	// uint256 AssetID
	convertedAssetID, _ := strconv.ParseUint(assetID, 10, 64)
	assetIDTypeObj, _ := method.Args[2].GetTypeObject()
	encodedAssetIDUINT256, _ := assetIDTypeObj.Encode(convertedAssetID)

	// string AssetID
	assetIDStringTypeObj, _ := method.Args[3].GetTypeObject()
	paddedAssetID := make([]byte, 256)
	copy(paddedAssetID, assetID)
	encodedAssetIDString, _ := assetIDStringTypeObj.Encode(paddedAssetID)

	// originChainID
	originChainID := 1
	originChainIDTypeObj, _ := method.Args[4].GetTypeObject()
	encodedOriginChainID, _ := originChainIDTypeObj.Encode(originChainID) // Ensure originChainID is of type uint64

	// Now include all encoded args
	args := [][]byte{method.GetSelector(), encodedReceiverAddress, encodedMetadata, encodedAssetIDUINT256, encodedAssetIDString, encodedOriginChainID}

	// Fetch suggested parameters from the client
	sp, err := b.voiAlgodClient.Client.SuggestedParams().Do(context.Background())
	if err != nil {
		return fmt.Errorf("unable to suggest params: %v", err)
	}

	// Construct the transaction
	simtx, err := transaction.MakeApplicationCallTxWithBoxes(
		appId,                // appID
		args,                 // appArgs
		nil,                  // accounts
		nil,                  // foreignApps
		nil,                  // foreignAssets
		nil,                  // appBoxReference
		types.NoOpOC,         // onCompletion
		nil,                  // approvalProgram
		nil,                  // clearProgram
		types.StateSchema{},  // globalStateSchema
		types.StateSchema{},  // localStateSchema
		0,                    // extraPages
		sp,                   // suggestedParams
		b.voiAccount.Address, // sender
		nil,                  // note
		types.Digest{},       // group
		[32]byte{},           // lease
		types.Address{},      // rekeyTo (again)
	)

	paymentTx, _ := transaction.MakePaymentTxn(
		b.voiAccount.Address.String(),
		crypto.GetApplicationAddress(appId).String(),
		microAlgosToSend,
		nil,
		"",
		sp,
	)

	gid, _ := crypto.ComputeGroupID([]types.Transaction{paymentTx, simtx})

	paymentTx.Group = gid
	simtx.Group = gid

	// Create a SimulateRequest
	simulateRequest := models.SimulateRequest{
		AllowEmptySignatures:  true,
		AllowUnnamedResources: true,
		TxnGroups: []models.SimulateRequestTransactionGroup{
			{
				Txns: []types.SignedTxn{
					{Txn: paymentTx},
					{Txn: simtx},
				},
			},
		},
	}

	// Simulate the transaction
	simulateResponse, _ := b.voiAlgodClient.Client.SimulateTransaction(simulateRequest).Do(context.Background())
	if simulateResponse.TxnGroups[0].FailureMessage != "" {
		return fmt.Errorf("failed to simulate transaction: %v", simulateResponse.TxnGroups[0].FailureMessage)
	}

	boxes := make([]types.AppBoxReference, len(simulateResponse.TxnGroups[0].UnnamedResourcesAccessed.Boxes))
	for i, box := range simulateResponse.TxnGroups[0].UnnamedResourcesAccessed.Boxes {
		boxes[i] = types.AppBoxReference{
			AppID: box.App,
			Name:  box.Name,
		}
	}

	// Construct the transaction
	tx, err := transaction.MakeApplicationCallTxWithBoxes(
		appId,                // appID
		args,                 // appArgs
		nil,                  // accounts
		nil,                  // foreignApps
		nil,                  // foreignAssets
		boxes,                // appBoxReference
		types.NoOpOC,         // onCompletion
		nil,                  // approvalProgram
		nil,                  // clearProgram
		types.StateSchema{},  // globalStateSchema
		types.StateSchema{},  // localStateSchema
		0,                    // extraPages
		sp,                   // suggestedParams
		b.voiAccount.Address, // sender
		nil,                  // note
		types.Digest{},       // group
		[32]byte{},           // lease
		types.Address{},      // rekeyTo (again)
	)

	paymentTx, _ = transaction.MakePaymentTxn(
		b.voiAccount.Address.String(),
		crypto.GetApplicationAddress(appId).String(),
		microAlgosToSend,
		nil,
		"",
		sp,
	)

	gid, _ = crypto.ComputeGroupID([]types.Transaction{paymentTx, tx})

	paymentTx.Group = gid
	tx.Group = gid

	// Sign the transaction
	_, signedTxBytes, err := crypto.SignTransaction(b.voiAccount.PrivateKey, tx)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	_, signedTxBytes2, err := crypto.SignTransaction(b.voiAccount.PrivateKey, paymentTx)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	signedGroup := append(signedTxBytes2, signedTxBytes...)

	// Send the transaction
	sendResponse, err := b.voiAlgodClient.Client.SendRawTransaction(signedGroup).Do(context.Background())
	fmt.Printf("Error", err)
	if err != nil {

		return fmt.Errorf("failed to send transaction: %v", err)
	}

	fmt.Printf("Success! Minted NFT to: %s, AssetID: %s, Transaction ID: %s\n", receiverAddress, assetID, sendResponse)

	return nil
}
