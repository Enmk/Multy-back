package eth

import (
	"context"

	"github.com/pkg/errors"

	"github.com/Multy-io/Multy-back/common"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
)

// TODO: add this logic with load info from Database and send to this method network id
func (self *EthController) GetBlockHeigth() (int64, error) {
	return 0, nil
	// return int64(self.blockHandler.GetBlockHeight()), nil
}

func (self *EthController) GetSeviceInfo() (*common.ServiceInfo, error) {
	serviceVersion, err := self.GRPCClient.GetServiceVersion(context.Background(), &pb.Empty{})
	log.Infof("GetSeviceInfo: %v, %v", serviceVersion, err)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch service info from NS")
	}

	return &common.ServiceInfo{
		Branch:    serviceVersion.GetBranch(),
		Commit:    serviceVersion.GetCommit(),
		Buildtime: serviceVersion.GetBuildtime(),
		Lasttag:   serviceVersion.GetLasttag(),
	}, nil
}
