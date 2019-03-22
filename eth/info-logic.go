package eth

import (
	"context"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
)

// TODO: add this logic with load info from Database and send to this method network id
func (self *ETHConn) GetBlockHeigth() (int64, error) {
	return int64(self.blockHandler.GetBlockHight()), nil
}

func (self *ETHConn) GetSeviceInfo() (*store.ServiceInfo, error) {
	serviceVersion, err := self.GRPCClient.ServiceInfo(context.Background(), &pb.Empty{})
	log.Errorf("%v, %v", serviceVersion, err)

	return &store.ServiceInfo{
		Branch:    serviceVersion.GetBranch(),
		Commit:    serviceVersion.GetCommit(),
		Buildtime: serviceVersion.GetBuildtime(),
		Lasttag:   serviceVersion.GetLasttag(),
	}, nil
}
