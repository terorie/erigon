package commands

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/gasprice"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/log/v3"
)

// BlockNumber implements eth_blockNumber. Returns the block number of most recent block.
func (api *APIImpl) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	execution, err := stages.GetStageProgress(tx, stages.Finish)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(execution), nil
}

// Syncing implements eth_syncing. Returns a data object detaling the status of the sync process or false if not syncing.
func (api *APIImpl) Syncing(ctx context.Context) (interface{}, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	highestBlock, err := stages.GetStageProgress(tx, stages.Headers)
	if err != nil {
		return false, err
	}

	currentBlock, err := stages.GetStageProgress(tx, stages.Finish)
	if err != nil {
		return false, err
	}

	if currentBlock > 0 && currentBlock >= highestBlock { // Return not syncing if the synchronisation already completed
		return false, nil
	}

	// Otherwise gather the block sync stats
	type S struct {
		StageName   string         `json:"stage_name"`
		BlockNumber hexutil.Uint64 `json:"block_number"`
	}
	stagesMap := make([]S, len(stages.AllStages))
	for i, stage := range stages.AllStages {
		progress, err := stages.GetStageProgress(tx, stage)
		if err != nil {
			return nil, err
		}
		stagesMap[i].StageName = string(stage)
		stagesMap[i].BlockNumber = hexutil.Uint64(progress)
	}

	return map[string]interface{}{
		"currentBlock": hexutil.Uint64(currentBlock),
		"highestBlock": hexutil.Uint64(highestBlock),
		"stages":       stagesMap,
	}, nil
}

// ChainId implements eth_chainId. Returns the current ethereum chainId.
func (api *APIImpl) ChainId(ctx context.Context) (hexutil.Uint64, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(chainConfig.ChainID.Uint64()), nil
}

// ChainID alias of ChainId - just for convenience
func (api *APIImpl) ChainID(ctx context.Context) (hexutil.Uint64, error) {
	return api.ChainId(ctx)
}

// ProtocolVersion implements eth_protocolVersion. Returns the current ethereum protocol version.
func (api *APIImpl) ProtocolVersion(ctx context.Context) (hexutil.Uint, error) {
	ver, err := api.ethBackend.ProtocolVersion(ctx)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint(ver), nil
}

// GasPrice implements eth_gasPrice. Returns the current price per gas in wei.
func (api *APIImpl) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	oracle := gasprice.NewOracle(api, ethconfig.Defaults.GPO)
	price, err := oracle.SuggestPrice(ctx)
	return (*hexutil.Big)(price), err
}

// HeaderByNumber is necessary for gasprice.OracleBackend implementation
func (api *APIImpl) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	blockNum, err := getBlockNumber(number, tx)
	if err != nil {
		return nil, err
	}

	header := rawdb.ReadHeaderByNumber(tx, blockNum)
	if header == nil {
		return nil, fmt.Errorf("header not found: %d", blockNum)
	}
	return header, nil
}

// BlockByNumber is necessary for gasprice.OracleBackend implementation
func (api *APIImpl) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	blockNum, err := getBlockNumber(number, tx)
	if err != nil {
		return nil, err
	}

	block, _, err := rawdb.ReadBlockByNumberWithSenders(tx, blockNum)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// ChainConfig is necessary for gasprice.OracleBackend implementation
func (api *APIImpl) ChainConfig() *params.ChainConfig {
	tx, err := api.db.BeginRo(context.TODO())
	if err != nil {
		log.Warn("Could not read chain config from the db, defaulting to MainnetChainConfig", "err", err)
		return params.MainnetChainConfig
	}
	defer tx.Rollback()

	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		log.Warn("Could not read chain config from the db, defaulting to MainnetChainConfig", "err", err)
		return params.MainnetChainConfig
	}
	return chainConfig
}
