package nseth

import (
	"strconv"
	"sync"
	"testing"

	"github.com/Multy-io/Multy-back/common"
)

func TestEstimateGasPrice(t *testing.T) {
	// test empty map
	mempoolTransactionGasPrices := &sync.Map{}
	actualEstimation := estimateTransactionGasPriceFromTxpool(mempoolTransactionGasPrices, 1)
	expectedEstimation := common.TransactionFeeRateEstimation{
		VerySlow: 9 * 1000000000,
		Slow:     10 * 1000000000,
		Medium:   14 * 1000000000,
		Fast:     20 * 1000000000,
		VeryFast: 25 * 1000000000,
	}
	if expectedEstimation != actualEstimation {
		t.Errorf("Wrong answer on emptyEstimate \nexpected: %v\nactual: %v",
			expectedEstimation, actualEstimation)
	}

	// test 100 record
	mempoolTransactionGasPrices = &sync.Map{}
	for i := uint64(0); i < 100; i++ {
		mempoolTransactionGasPrices.Store(strconv.FormatUint(i, 10)+"1", i)
	}

	actualEstimation = estimateTransactionGasPriceFromTxpool(mempoolTransactionGasPrices, 1)
	if expectedEstimation != actualEstimation {
		t.Errorf("Wrong answer on  100 elements \nexpected: %v\nactual: %v",
			expectedEstimation, actualEstimation)
	}

	// test 600 record
	mempoolTransactionGasPrices = &sync.Map{}
	for i := uint64(0); i < 600; i++ {
		mempoolTransactionGasPrices.Store(strconv.FormatUint(i, 10)+"1", i)
	}

	expectedEstimation = common.TransactionFeeRateEstimation{
		VerySlow: 67,
		Slow:     202,
		Medium:   337,
		Fast:     472,
		VeryFast: 569,
	}
	actualEstimation = estimateTransactionGasPriceFromTxpool(mempoolTransactionGasPrices, 1)
	if expectedEstimation != actualEstimation {
		t.Errorf("Wrong answer on  600 elements \nexpected: %v\nactual: %v",
			expectedEstimation, actualEstimation)
	}

	// test 2000 record
	mempoolTransactionGasPrices = &sync.Map{}
	for i := uint64(0); i < 2000; i++ {
		mempoolTransactionGasPrices.Store(strconv.FormatUint(i, 10)+"1", i)
	}

	expectedEstimation = common.TransactionFeeRateEstimation{
		VerySlow: 846,
		Slow:     1139,
		Medium:   1431,
		Fast:     1723,
		VeryFast: 1934,
	}
	actualEstimation = estimateTransactionGasPriceFromTxpool(mempoolTransactionGasPrices, 1)
	if expectedEstimation != actualEstimation {
		t.Errorf("Wrong answer on  2000 elements \nexpected: %v\nactual: %v",
			expectedEstimation, actualEstimation)
	}

}
