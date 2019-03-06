/*
Copyright 2019 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package eth

import (
	"context"
	"io"

	"github.com/Multy-io/Multy-back/currencies"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func (ethcli *ETHConn) setGRPCHandlers(networkID int, accuracyRange int) {
	var client pb.NodeCommunicationsClient
	var wa chan pb.WatchAddress

	nsqProducer := ethcli.NsqProducer

	switch networkID {
	case currencies.ETHMain:
		client = ethcli.CliMain
		wa = ethcli.WatchAddressMain
	case currencies.ETHTest:
		client = ethcli.CliTest
		wa = ethcli.WatchAddressTest

	}

	// add to transaction history record and send ws notification on tx
	go func() {
		stream, err := client.NewTx(context.Background(), &pb.Empty{})
		if err != nil {
			log.Errorf("setGRPCHandlers: cli.NewTx: %s", err.Error())
		}

		for {
			gTx, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf("initGrpcClient: cli.NewTx:stream.Recv: %s", err.Error())
			}

			log.Warnf("new tx for uid %v ", gTx.GetUserID())

			tx := generatedTxDataToStore(gTx)
			setExchangeRates(&tx, gTx.Resync, tx.BlockTime)

			err = saveTransaction(tx, networkID, gTx.Resync)
			updateWalletAndAddressDate(tx, networkID)
			if err != nil {
				log.Errorf("initGrpcClient: saveMultyTransaction: %s", err)
			}

			if !gTx.GetResync() {
				sendNotifyToClients(tx, nsqProducer, networkID)
			}
		}
	}()

	go func() {
		stream, err := client.EventNewBlock(context.Background(), &pb.Empty{})
		if err != nil {
			log.Errorf("setGRPCHandlers: cli.EventNewBlock: %s", err.Error())
			// return nil, err
		}
		for {
			h, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf("setGRPCHandlers: client.EventNewBlock:stream.Recv: %s", err.Error())
			}

			height := h.GetHeight()
			sel := bson.M{"currencyid": currencies.Ether, "networkid": networkID}
			update := bson.M{
				"$set": bson.M{
					"blockheight": height,
				},
			}
			_, err = restoreState.Upsert(sel, update)
			if err != nil {
				log.Errorf("restoreState.Upsert: %v", err.Error())
			}

			// check for rejected transactions
			var txStore *mgo.Collection
			var nsCli pb.NodeCommunicationsClient
			switch networkID {
			case currencies.ETHMain:
				txStore = txsData
				nsCli = ethcli.CliMain
			case currencies.ETHTest:
				txStore = txsDataTest
				nsCli = ethcli.CliTest
			}

			query := bson.M{
				"$or": []bson.M{
					bson.M{"$and": []bson.M{
						bson.M{"blockheight": 0},
						bson.M{"txstatus": bson.M{"$nin": []int{store.TxStatusTxRejectedOutgoing, store.TxStatusTxRejectedIncoming}}},
					}},

					bson.M{"$and": []bson.M{
						bson.M{"blockheight": bson.M{"$lt": h.GetHeight()}},
						bson.M{"blockheight": bson.M{"$gt": h.GetHeight() - int64(accuracyRange)}},
					}},
				},
			}

			txs := []store.TransactionETH{}
			txStore.Find(query).All(&txs)

			hashes := &pb.TxsToCheck{}

			for _, tx := range txs {
				hashes.Hash = append(hashes.Hash, tx.Hash)
			}

			txToReject, err := nsCli.CheckRejectTxs(context.Background(), hashes)
			if err != nil {
				log.Errorf("setGRPCHandlers: CheckRejectTxs: %s", err.Error())
			}

			// TODO: Pasha remove this if and rewrite for below
			// Set status to rejected in db
			if len(txToReject.GetRejectedTxs()) > 0 {

				for _, hash := range txToReject.GetRejectedTxs() {
					// reject incoming
					query := bson.M{"$and": []bson.M{
						bson.M{"hash": hash},
						bson.M{"txstatus": bson.M{"$in": []int{store.TxStatusAppearedInMempoolIncoming,
							store.TxStatusAppearedInBlockIncoming,
							store.TxStatusInBlockConfirmedIncoming}}},
					}}
					update := bson.M{
						"$set": bson.M{
							"txstatus": store.TxStatusTxRejectedIncoming,
						},
					}
					_, err := txStore.UpdateAll(query, update)
					if err != nil {
						log.Errorf("setGRPCHandlers: cli.EventNewBlock:txStore.UpdateAll:Incoming: %s", err.Error())
					}

					// reject outcoming
					query = bson.M{"$and": []bson.M{
						bson.M{"hash": hash},
						bson.M{"txstatus": bson.M{"$in": []int{store.TxStatusAppearedInMempoolOutcoming,
							store.TxStatusAppearedInBlockOutcoming,
							store.TxStatusInBlockConfirmedOutcoming}}},
					}}
					update = bson.M{
						"$set": bson.M{
							"txstatus": store.TxStatusTxRejectedOutgoing,
						},
					}
					_, err = txStore.UpdateAll(query, update)
					if err != nil {
						log.Errorf("setGRPCHandlers: cli.EventNewBlock:txStore.UpdateAll:Outcoming: %s", err.Error())
					}
				}
				// TODO: Pasha remove this check error
				if err != nil {
					log.Errorf("initGrpcClient: restoreState.Update: %s", err.Error())
				}
			}

		}
	}()

	// watch for channel and push to node
	go func() {
		for {
			select {
			case addr := <-wa:
				a := addr
				rp, err := client.EventAddNewAddress(context.Background(), &a)
				if err != nil {
					log.Errorf("NewAddressNode: cli.EventAddNewAddress %s\n", err.Error())
				}
				log.Debugf("EventAddNewAddress Reply %s", rp)

				rp, err = client.EventResyncAddress(context.Background(), &pb.AddressToResync{
					Address: addr.Address,
				})
				if err != nil {
					log.Errorf("EventResyncAddress: cli.EventResyncAddress %s\n", err.Error())
				}
				log.Debugf("EventResyncAddress Reply %s", rp)

			}
		}
	}()

}
