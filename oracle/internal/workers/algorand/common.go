package workers

import (
	"errors"
	"fmt"
	"nft-bridge/internal/algodapi"
	"nft-bridge/internal/indexerapi"
	"sync"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/types"
)

type AlgoBridge struct {
	algoAlgodClient   *algodapi.AlgodAPI
	algoIndexerClient *indexerapi.IndexerAPI
	voiAlgodClient    *algodapi.AlgodAPI
	voiIndexerClient  *indexerapi.IndexerAPI
	algoAccount       crypto.Account
	voiAccount        crypto.Account
	lastKnownRound    uint64
	nftStore          map[uint64]BridgedNFT
	TxnChannel        chan models.Transaction
	mu                sync.Mutex // Mutex to protect lastKnownRound
}

func NewAlgoBridge(algoAlgodClient *algodapi.AlgodAPI, algoIndexerClient *indexerapi.IndexerAPI, voiAlgodClient *algodapi.AlgodAPI, voiIndexerClient *indexerapi.IndexerAPI, algoAccount crypto.Account, voiAccount crypto.Account) *AlgoBridge {
	return &AlgoBridge{
		algoAlgodClient:   algoAlgodClient,
		algoIndexerClient: algoIndexerClient,
		voiAlgodClient:    voiAlgodClient,
		voiIndexerClient:  voiIndexerClient,
		algoAccount:       algoAccount,
		voiAccount:        voiAccount,
		lastKnownRound:    0,
		TxnChannel:        make(chan models.Transaction, 1000),
		nftStore:          make(map[uint64]BridgedNFT),
	}
}

func (b *AlgoBridge) updateNFTStore(nft *BridgedNFT) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nftStore[nft.AssetID] = *nft
	fmt.Printf("NFT with AssetID %d has been updated to state: %s\n", nft.AssetID, nft.State)
}

type ChainType string
type StateType string
type SpecType string

// Define your chain types
const (
	Algorand ChainType = "Algorand"
	Polygon  ChainType = "Polygon"
	Solana   ChainType = "Solana"
)

// Define your state types
const (
	Expect   StateType = "Expect"
	Prepared StateType = "Prepared"
	Received StateType = "Received"
	Minted   StateType = "Minted"
	Sent     StateType = "Sent"
)

const (
	ARC3  SpecType = "ARC3"
	ARC69 SpecType = "ARC69"
	ARC19 SpecType = "ARC19"
	ARC72 SpecType = "ARC72"
	None  SpecType = "None" // To use when the asset doesn't adhere to any known spec
)

// BridgedNFT represents an NFT being bridged between blockchains.
type BridgedNFT struct {
	ChainOfOrigin ChainType          // Chain of origin for the NFT
	State         StateType          // Current state of the NFT in the bridge process
	To            types.Address      // Algorand address to which the NFT will be sent
	AssetID       uint64             // Unique identifier for the NFT
	Sender        string             // String representing from where the NFT is coming
	Transaction   models.Transaction // Original transaction
	Spec          SpecType           // The specification of the NFT (ARC3, ARC69, etc.)
	AssetURL      string             // URL of the asset
}

// NewBridgedNFT creates a new BridgedNFT instance with validation.
func NewBridgedNFT(chain ChainType, state StateType, to types.Address, id uint64, sender string) (*BridgedNFT, error) {
	if !chain.IsValid() {
		return nil, errors.New("invalid chain type")
	}
	if !state.IsValid() {
		return nil, errors.New("invalid state type")
	}
	return &BridgedNFT{
		ChainOfOrigin: chain,
		State:         state,
		To:            to,
		AssetID:       id,
		Sender:        sender,
	}, nil
}

// IsValid checks if the ChainType is one of the predefined constants.
func (c ChainType) IsValid() bool {
	switch c {
	case Algorand, Polygon, Solana:
		return true
	}
	return false
}

// IsValid checks if the StateType is one of the predefined constants.
func (s StateType) IsValid() bool {
	switch s {
	case Expect, Prepared, Received, Minted, Sent:
		return true
	}
	return false
}

// IsValid checks if the SpecType is one of the predefined constants.
func (s SpecType) IsValid() bool {
	switch s {
	case ARC3, ARC69, ARC19, ARC72:
		return true
	}
	return false
}
