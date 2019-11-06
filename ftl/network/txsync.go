// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package p2p_handler contains the implementation of p2p handler for fractal.
package network

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/packer/pksvc"
	"github.com/fractal-platform/fractal/transaction"
	"github.com/fractal-platform/fractal/utils/log"
)

func (pm *ProtocolManager) txBroadcastLoop() {
	pm.wg.Add(1)
	defer pm.wg.Done()

	for {
		select {
		case event := <-pm.txsCh:
			txs := append(types.Transactions{}, transaction.ElemsToTxs(event.Ems)...)
			pm.BroadcastTxs(txs)

			// Err() channel will be closed when unsubscribing.
		case <-pm.txsSub.Err():
			return
		}
	}
}

func (pm *ProtocolManager) txPackageListenLoop() {
	pm.wg.Add(1)
	defer pm.wg.Done()

	for {
		select {
		case event := <-pm.txPkgCh:
			txPkgHashes := append([]common.Hash{}, pksvc.ElemsToTxPkgHashes(event.Ems)...)
			for i := range txPkgHashes {
				for _, block := range pm.blockchain.FutureTxPackageBlocks(txPkgHashes[i]) {
					log.Info("Try to insert future txPackage block", "txPkgHash", txPkgHashes[i], "blockHash", block.FullHash())
					pm.BlockProcessCh <- &BlockWithVerifyFlag{block, true}
				}
				pm.blockchain.RemoveFutureTxPackageBlocks(txPkgHashes[i])
			}
		case <-pm.txPkgSub.Err():
			return
		}
	}
}

// BroadcastTxs will propagate a batch of transactions to all peers which are not known to
// already have the given transaction.
func (pm *ProtocolManager) BroadcastTxs(txs types.Transactions) {
	var txset = make(map[*Peer]types.Transactions)

	// Broadcast transactions to a batch of peers not knowing about it
	for _, tx := range txs {
		peers := pm.peers.PeersWithoutTx(tx.Hash())
		for _, peer := range peers {
			txset[peer] = append(txset[peer], tx)
		}
		log.Debug("Broadcast transaction", "Hash", tx.Hash(), "recipients", len(peers))
	}
	// FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	for peer, txs := range txset {
		peer.AsyncSendTransactions(txs)
	}
}
