/*
Copyright 2019 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package eth

import (
	"context"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/types/eth"
)

func (ethcli *ETHConn) setGRPCHandlers(networkID int, accuracyRange int) {

	// TODO: Write method  transaction handler with NSQ from ns-eth

	// TODO: update blockHeight on database and check logic parce and send notify from ns-eth
	// TODO: move logic rejected tx  to ns-eth look on old commit with it logic

	// watch for channel and push to node
	go func() {
		for {
			select {
			case addr := <-ethcli.WatchAddress:
				// TODO: split chan and move emit new address to nsq
				err := ethcli.NSQClient.EmitNewAddressEvent(eth.HexToAddress(addr.Address))
				if err != nil {
					log.Errorf("NewAddressNode: cli.EventAddNewAddress %s\n", err.Error())
				}
				// log.Debugf("EventAddNewррррнAddress Reply %s", rp)

				rp, err := ethcli.GRPCClient.ResyncAddress(context.Background(), &pb.Address{
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
