/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package client

import (
	"github.com/pkg/errors"

	"github.com/Multy-io/Multy-back/exchanger"

	"net/http"
	"strconv"
	"strings"
	"time"

	ethcommon "github.com/Multy-io/Multy-back/common/eth"
	"github.com/Multy-io/Multy-back/currencies"
	"github.com/Multy-io/Multy-back/eth"
	"github.com/Multy-io/Multy-back/store"
	"github.com/jekabolt/slf"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

const (
	msgErrMissingRequestParams  = "missing request parameters"
	msgErrServerError           = "internal server error"
	msgErrNoWallet              = "no such wallet"
	msgErrWalletNonZeroBalance  = "can't delete non zero balance wallet"
	msgErrWalletIndex           = "already existing wallet index"
	msgErrKnownAddres           = "already existing wallet address"
	msgErrWrongBadAddress       = "bad address"
	msgErrTxHistory             = "not found any transaction history"
	msgErrAddressIndex          = "already existing address index"
	msgErrMethodNotImplennted   = "method is not implemented"
	msgErrHeaderError           = "wrong authorization headers"
	msgErrRequestBodyError      = "missing request body params"
	msgErrUserNotFound          = "user not found in db"
	msgErrNoTransactionAddress  = "zero balance"
	msgErrNoSpendableOutputs    = "no spendable outputs"
	msgErrRatesError            = "internal server error rates"
	msgErrDecodeWalletIndexErr  = "wrong wallet index"
	msgErrDecodeNetworkIDErr    = "wrong network id"
	msgErrDecodeTypeErr         = "wrong type"
	msgErrNoSpendableOuts       = "no spendable outputs"
	msgErrDecodeCurIndexErr     = "wrong currency index"
	msgErrDecodenetworkidErr    = "wrong networkid index"
	msgErrAdressBalance         = "empty address or 3-rd party server error"
	msgErrChainIsNotImplemented = "current chain is not implemented"
	msgErrUserHaveNoTxs         = "user have no transactions"
	msgErrWrongSeedPhraseType   = "seed phrase type is not supported"
)

type RestClient struct {
	middlewareJWT *GinJWTMiddleware
	userStore     store.UserStore

	log slf.StructuredLogger

	// donationAddresses []store.DonationInfo
	mobileVersions store.MobileVersions
	ERC20TokenList store.VerifiedTokenList

	ETH              *eth.EthController
	MultyVersion     store.ServerConfig
	BrowserDefault   store.BrowserDefault
	Secretkey        string
	ExchangerFactory *exchanger.FactoryExchanger
}

func SetRestHandlers(
	// TODO: reduce properties amount and get desired services directly from multy container
	userDB store.UserStore,
	r *gin.Engine,
	// donationAddresses []store.DonationInfo,
	eth *eth.EthController,
	mv store.ServerConfig,
	secretkey string,
	mobileVer store.MobileVersions,
	tl store.VerifiedTokenList,
	bd store.BrowserDefault,
	exchangerFactory *exchanger.FactoryExchanger,
) (*RestClient, error) {
	restClient := &RestClient{
		userStore: userDB,
		log:       slf.WithContext("rest-client").WithCaller(slf.CallerShort),
		// donationAddresses: donationAddresses,
		ETH:              eth,
		MultyVersion:     mv,
		Secretkey:        secretkey,
		mobileVersions:   mobileVer,
		ERC20TokenList:   tl,
		BrowserDefault:   bd,
		ExchangerFactory: exchangerFactory,
	}
	initMiddlewareJWT(restClient)

	r.POST("/auth", restClient.LoginHandler())
	r.GET("/server/config", restClient.getServerConfig())

	v1 := r.Group("/api/v1")
	v1.Use(restClient.middlewareJWT.MiddlewareFunc())
	{
		v1.POST("/wallet", restClient.addWallet())
		v1.GET("/transaction/feerate/:currencyid/:networkid", restClient.getFeeRate())
		v1.GET("/transaction/feerate/:currencyid/:networkid/*address", restClient.getFeeRate())
		v1.POST("/transaction/send", restClient.sendTransaction())
		v1.GET("/wallet/:walletindex/verbose/:currencyid/:networkid", restClient.getWalletVerbose())
		v1.GET("/wallets/verbose", restClient.getAllWalletsVerbose())
		v1.GET("/wallets/transactions/:currencyid/:networkid/:walletindex", restClient.getWalletTransactionsHistory())
		v1.GET("/wallets/transactions/:currencyid/:networkid/:walletindex/:token", restClient.getWalletTokenTransactionsHistory())
		v1.POST("/resync/wallet/:currencyid/:networkid/:walletindex/*type", restClient.resyncWallet())

		v1.GET("/exchanger/supported_currencies", restClient.GetExchangerSupportedCurrencies())
		v1.POST("/exchanger/exchange_amount", restClient.GetExchangerAmountExchange())
		v1.POST("/exchanger/transaction", restClient.CreateExchangerTransaction())
		v1.POST("/exchanger/minimum_amount", restClient.GetExchangerTransactionMinimumAmount())
	}
	return restClient, nil
}

func initMiddlewareJWT(restClient *RestClient) {
	restClient.middlewareJWT = &GinJWTMiddleware{
		Realm:      "test zone",
		Key:        []byte(restClient.Secretkey), // config
		Timeout:    time.Hour,
		MaxRefresh: time.Hour,
		Authenticator: func(userId, deviceId, pushToken string, deviceType int, seedPhraseType int, c *gin.Context) (store.User, bool) {
			query := bson.M{"userID": userId}

			user := store.User{}

			err := restClient.userStore.FindUser(query, &user)

			if err != nil || len(user.UserID) == 0 {
				return user, false
			}
			return user, true
		},
		Unauthorized: nil,
		TokenLookup:  "header:Authorization",

		TokenHeadName: "Bearer",

		TimeFunc: time.Now,
	}
}

type SelectWallet struct {
	CurrencyID   int    `json:"currencyID"`
	NetworkID    int    `json:"networkID"`
	WalletIndex  int    `json:"walletIndex"`
	Address      string `json:"address"`
	AddressIndex int    `json:"addressIndex"`
}

type Tx struct {
	Transaction   string `json:"transaction"`
	AllowHighFees bool   `json:"allowHighFees"`
}

type DisplayWallet struct {
	Chain    string          `json:"chain"`
	Adresses []store.Address `json:"addresses"`
}

type WalletExtended struct {
	CuurencyID  int         `bson:"chain"`       //cuurencyID
	WalletIndex int         `bson:"walletIndex"` //walletIndex
	Addresses   []AddressEx `bson:"addresses"`
}

type AddressEx struct { // extended
	AddressID int    `bson:"addressID"` //addressIndex
	Address   string `bson:"address"`
	Amount    int    `bson:"amount"` //float64
}

func getToken(c *gin.Context) (string, error) {
	authHeader := strings.Split(c.GetHeader("Authorization"), " ")
	if len(authHeader) < 2 {
		return "", errors.New(msgErrHeaderError)
	}
	return authHeader[1], nil
}

func createCustomWallet(wp store.WalletParams, token string, restClient *RestClient, isEmpty bool, c *gin.Context) error {

	user := store.User{}
	query := bson.M{"devices.JWT": token}
	err := restClient.userStore.FindUser(query, &user)
	if err != nil {
		restClient.log.Errorf("createCustomWallet: restClient.userStore.FindUser: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
		return errors.New(msgErrUserNotFound)
	}

	wallet := store.Wallet{}
	//internal
	if !wp.IsImported {
		for _, checkWallet := range user.Wallets {
			if checkWallet.CurrencyID == wp.CurrencyID && checkWallet.NetworkID == wp.NetworkID && checkWallet.WalletIndex == wp.WalletIndex {
				err = errors.New(msgErrWalletIndex)
				return err
			}
		}
		if wp.CurrencyID == currencies.Ether {
			wp.Address = strings.ToLower(wp.Address)
		}

		walletStatus := store.WalletStatusOK
		if isEmpty {
			walletStatus = store.WalletStatusDeleted
		}
		wallet = createWallet(wp.CurrencyID, wp.NetworkID, wp.Address, wp.AddressIndex, wp.WalletIndex, wp.WalletName, wp.IsImported, walletStatus)
		go NewAddressNode(wp.Address, user.UserID, wp.CurrencyID, wp.NetworkID, wp.WalletIndex, wp.AddressIndex, restClient)
	}

	sel := bson.M{"devices.JWT": token}
	update := bson.M{"$push": bson.M{"wallets": wallet}}

	err = restClient.userStore.Update(sel, update)
	if err != nil {
		restClient.log.Errorf("createCustomWallet: restClient.userStore.Update: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
		return errors.New(msgErrServerError)
	}

	return nil
}

func changeName(cn ChangeName, token string, restClient *RestClient, c *gin.Context) error {
	user := store.User{}
	query := bson.M{"devices.JWT": token}

	if err := restClient.userStore.FindUser(query, &user); err != nil {
		restClient.log.Errorf("changeName: restClient.userStore.FindUser: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
		err := errors.New(msgErrUserNotFound)
		return err
	}
	var position int

	switch cn.Type {
	case store.AssetTypeMultyAddress:
		for i, wallet := range user.Wallets {
			if wallet.NetworkID == cn.NetworkID && wallet.WalletIndex == cn.WalletIndex && wallet.CurrencyID == cn.CurrencyID {
				position = i
				break
			}
		}
		sel := bson.M{"userID": user.UserID, "wallets.walletIndex": cn.WalletIndex, "wallets.networkID": cn.NetworkID}
		update := bson.M{
			"$set": bson.M{
				"wallets." + strconv.Itoa(position) + ".walletName": cn.WalletName,
			},
		}
		err := restClient.userStore.Update(sel, update)
		if err != nil {
			return errors.New("changeName:restClient.userStore.Update:AssetTypeMultyAddress " + err.Error())
		}
		return err

	case store.AssetTypeImportedAddress:
		query := bson.M{"userID": user.UserID, "wallets.addresses.address": cn.Address}
		update := bson.M{
			"$set": bson.M{
				"wallets.$.walletName": cn.WalletName,
			},
		}
		err := restClient.userStore.Update(query, update)
		if err != nil {
			return errors.New("changeName:restClient.userStore.Update:AssetTypeImportedAddress " + err.Error())
		}
		return err

	}

	return errors.New(msgErrNoWallet)

}

func addAddressToWallet(address, token string, currencyID, networkid, walletIndex, addressIndex int, restClient *RestClient, c *gin.Context) error {
	user := store.User{}
	query := bson.M{"devices.JWT": token}

	if err := restClient.userStore.FindUser(query, &user); err != nil {
		restClient.log.Errorf("addAddressToWallet: restClient.userStore.FindUser: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
		return errors.New(msgErrUserNotFound)
	}

	var position int
	for i, wallet := range user.Wallets {
		if wallet.NetworkID == networkid && wallet.CurrencyID == currencyID && wallet.WalletIndex == walletIndex {
			position = i
			for _, walletAddress := range wallet.Adresses {
				if walletAddress.AddressIndex == addressIndex {
					return errors.New(msgErrAddressIndex)
				}
			}
		}
	}

	addr := store.Address{
		Address:        address,
		AddressIndex:   addressIndex,
		LastActionTime: time.Now().Unix(),
	}

	//TODO: make no possibility to add eth address
	sel := bson.M{"devices.JWT": token, "wallets.currencyID": currencyID, "wallets.networkID": networkid, "wallets.walletIndex": walletIndex}
	update := bson.M{"$push": bson.M{"wallets." + strconv.Itoa(position) + ".addresses": addr}}
	if err := restClient.userStore.Update(sel, update); err != nil {
		restClient.log.Errorf("addAddressToWallet: restClient.userStore.Update: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
		return errors.New(msgErrServerError)
	}

	return nil

}

func NewAddressNode(address, userid string, currencyID, networkID, walletIndex, addressIndex int, restClient *RestClient) {
	switch currencyID {
	case currencies.Ether:
		if networkID == currencies.ETHMain {
			restClient.ETH.WatchAddress <- eth.UserAddress{
				Address:      address,
				UserID:       userid,
				WalletIndex:  int32(walletIndex),
				AddressIndex: int32(addressIndex),
			}
		}

	}
}

func (restClient *RestClient) addWallet() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c)
		if err != nil {
			restClient.log.Errorf("addWallet: getToken: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrHeaderError,
			})
			return
		}

		var (
			code    = http.StatusOK
			message = http.StatusText(http.StatusOK)
		)

		var wp store.WalletParams

		err = decodeBody(c, &wp)
		if err != nil {
			restClient.log.Errorf("addWallet: decodeBody: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrRequestBodyError,
			})
			return
		}

		err = restClient.userStore.CheckAddWallet(&wp, token)
		if err != nil {
			restClient.log.Errorf("addWallet: CheckAddWallet: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusNotAcceptable, gin.H{
				"code":    http.StatusNotAcceptable,
				"message": err.Error(),
			})
			return
		}

		// Create wallet
		err = createCustomWallet(wp, token, restClient, false, c)
		if err != nil {
			restClient.log.Errorf("addWallet:createCustomWallet %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"code":    code,
			"time":    time.Now().Unix(),
			"message": message,
		})
		return
	}
}

type DiscoverWallets struct {
	CurrencyID int               `json:"currencyID"`
	NetworkID  int               `json:"networkId"`
	Addresses  []DiscoverAddress `json:"addresses"`
}

type DiscoverAddress struct {
	WalletIndex  int    `json:"walletIndex"`
	WalletName   string `json:"walletName"`
	AddressIndex int    `json:"addressIndex"`
	Address      string `json:"address"`
}

type ChangeName struct {
	WalletName  string `json:"walletname"`
	CurrencyID  int    `json:"currencyID"`
	WalletIndex int    `json:"walletIndex"`
	NetworkID   int    `json:"networkId"`
	Address     string `json:"address"`
	Type        int    `json:"type"`
}

func (restClient *RestClient) getServerConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := map[string]interface{}{
			"stockexchanges": map[string][]string{
				"poloniex": []string{"usd_btc", "eth_btc", "eth_usd", "btc_usd"},
				"gdax":     []string{"eur_btc", "usd_btc", "eth_btc", "eth_usd", "eth_eur", "btc_usd"},
			},
			"servertime":     time.Now().UTC().Unix(),
			"api":            "1.2",
			"version":        restClient.MultyVersion,
			"erc20tokenlist": restClient.ERC20TokenList,
		}
		resp["android"] = map[string]int{
			"soft": restClient.mobileVersions.Android.Soft,
			"hard": restClient.mobileVersions.Android.Hard,
		}
		resp["ios"] = map[string]int{
			"soft": restClient.mobileVersions.Ios.Soft,
			"hard": restClient.mobileVersions.Ios.Hard,
		}
		resp["browserdefault"] = store.BrowserDefault{
			URL:        restClient.BrowserDefault.URL,
			CurrencyID: restClient.BrowserDefault.CurrencyID,
			NetworkID:  restClient.BrowserDefault.NetworkID,
		}

		c.JSON(http.StatusOK, resp)
	}
}

func (restClient *RestClient) getFeeRate() gin.HandlerFunc {
	return func(c *gin.Context) {
		currencyID, err := strconv.Atoi(c.Param("currencyid"))
		if err != nil {
			restClient.log.Errorf("getFeeRate: non int currency id: %s \t[addr=%s]", err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrDecodeCurIndexErr,
			})
			return
		}

		networkid, err := strconv.Atoi(c.Param("networkid"))
		restClient.log.Debugf("getFeeRate [%d] \t[networkid=%s]", networkid, c.Request.RemoteAddr)
		if err != nil {
			restClient.log.Errorf("getFeeRate: non int networkid:[%d] %s \t[addr=%s]", networkid, err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrDecodenetworkidErr,
			})
			return
		}
		var address ethcommon.Address
		if len(c.Param("address")) > 0 {
			address = ethcommon.HexToAddress(c.Param("address")[1:])
			restClient.log.Debugf("getFeeRate [%d] \t[networkID=%s]", address, c.Request.RemoteAddr)
			if err != nil {
				restClient.log.Errorf("getFeeRate: non int networkid:[%d] %s \t[addr=%s]", address, err.Error(), c.Request.RemoteAddr)
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": msgErrDecodenetworkidErr,
				})
				return
			}
		}

		switch currencyID {
		case currencies.Ether:
			// var feeRateEstimation types.TransactionFeeRateEstimation
			gasPriceEstimate, gasLimitEstimate := restClient.ETH.GetFeeRate(address)
			c.JSON(http.StatusOK, gin.H{
				"gaslimit": gasLimitEstimate,
				"speeds":   gasPriceEstimate,
				"code":     http.StatusOK,
				"message":  http.StatusText(http.StatusOK),
			})
			return
		default:
		}
	}
}

type RawHDTx struct {
	CurrencyID int `json:"currencyid"`
	NetworkID  int `json:"networkID"`
	Payload    `json:"payload"`
}

type Payload struct {
	Address      string `json:"address"`
	AddressIndex int    `json:"addressindex"`
	WalletIndex  int    `json:"walletindex"`
	Transaction  string `json:"transaction"`
	IsHD         bool   `json:"ishd"`
	//  MultisigFactory     bool     `json:"multisigfactory"`
	WalletName          string   `json:"walletname"`
	Owners              []string `json:"owners"`
	ConfirmationsNeeded int      `json:"confirmationsneeded"`
}

func (restClient *RestClient) sendTransaction() gin.HandlerFunc {
	return func(c *gin.Context) {

		var rawTx RawHDTx
		if err := decodeBody(c, &rawTx); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrRequestBodyError,
			})
			return
		}

		token, err := getToken(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrHeaderError,
			})
			return
		}

		user := store.User{}
		query := bson.M{"devices.JWT": token}
		if err := restClient.userStore.FindUser(query, &user); err != nil {
			restClient.log.Errorf("sendRawHDTransaction: restClient.userStore.FindUser: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)

			return
		}
		// code := http.StatusOK
		switch rawTx.CurrencyID {
		case currencies.Ether:
			if rawTx.NetworkID == currencies.ETHMain {
				hash, err := restClient.ETH.SendRawTransaction(ethcommon.RawTransaction(rawTx.Transaction))
				if err != nil {
					restClient.log.Errorf("sendRawHDTransaction:eth.SendRawTransaction %s \n raw tx = %v ", err.Error(), rawTx.Transaction)
					c.JSON(http.StatusNotAcceptable, gin.H{
						"code":    http.StatusNotAcceptable,
						"message": err.Error(),
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code": http.StatusOK,
					// Client don't use this parametr
					"message": hash,
				})

				return
			}

		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrChainIsNotImplemented,
			})
		}
	}
}

func errHandler(resp string) bool {
	return strings.Contains(resp, "err:")
}

type RawTx struct { // remane RawClientTransaction
	Transaction string `json:"transaction"` //HexTransaction
}

func (restClient *RestClient) getWalletVerbose() gin.HandlerFunc {
	return func(c *gin.Context) {
		var wv []interface{}
		//var wv []WalletVerbose
		token, err := getToken(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrHeaderError,
			})
			return
		}

		// TODO: find out what the magic with this variable "derivationPath"
		var derivationPath string
		walletIndex, err := strconv.Atoi(c.Param("walletindex"))
		restClient.log.Debugf("getWalletVerbose [%d] \t[walletindexr=%s]", walletIndex, c.Request.RemoteAddr)
		if err != nil {
			derivationPath = strings.ToLower(c.Param("walletindex"))
			restClient.log.Errorf("Error where parse walletindex derivationPath: %v, error: %v", derivationPath, err)
		}
		currencyId, err := strconv.Atoi(c.Param("currencyid"))
		restClient.log.Debugf("getWalletVerbose [%d] \t[currencyId=%s]", walletIndex, c.Request.RemoteAddr)
		if err != nil {
			restClient.log.Errorf("getWalletVerbose: non int currency id:[%d] %s \t[addr=%s]", currencyId, err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrDecodeCurIndexErr,
			})
			return
		}

		networkId, err := strconv.Atoi(c.Param("networkid"))
		restClient.log.Debugf("getWalletVerbose [%d] \t[networkID=%s]", networkId, c.Request.RemoteAddr)
		if err != nil {
			restClient.log.Errorf("getWalletVerbose: non int networkid:[%d] %s \t[addr=%s]", networkId, err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrDecodeNetworkIDErr,
				"wallet":  wv,
			})
			return
		}

		//multy address
		assetType := store.AssetTypeMultyAddress
		if len(c.Param("type")) > 0 {
			assetType, err = strconv.Atoi(c.Param("type")[1:])
			restClient.log.Debugf("getWalletVerbose [%d] \t[networkID=%s]", networkId, c.Request.RemoteAddr)
			if err != nil {
				restClient.log.Errorf("getWalletVerbose: non int networkid:[%d] %s \t[addr=%s]", networkId, err.Error(), c.Request.RemoteAddr)
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": msgErrDecodeTypeErr,
					"wallet":  wv,
				})
				return
			}
		}

		var (
			code    int
			message string
		)

		user := store.User{}

		switch currencyId {

		case currencies.Ether:
			code = http.StatusOK
			message = http.StatusText(http.StatusOK)

			var av []ETHAddressVerbose

			query := bson.M{"devices.JWT": token}
			if err := restClient.userStore.FindUser(query, &user); err != nil {
				restClient.log.Errorf("getWalletsVerbose: restClient.userStore.FindUser: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
			}

			// fetch wallet with concrete networkid currencyid and wallet index
			var pending bool
			var totalBalance string
			var pendingBalance string
			var waletNonce uint64
			wallet := store.Wallet{}

			// wallet verbose
			// TODO: rewrite with another select from DB witch alredy have wallets with this netwjrkID and currencyID
			if assetType == store.AssetTypeMultyAddress {
				for _, w := range user.Wallets {
					if w.NetworkID == networkId && w.CurrencyID == currencyId && w.WalletIndex == walletIndex {
						wallet = w
						break
					}
				}

				if len(wallet.Adresses) == 0 {
					restClient.log.Errorf("getWalletsVerbose: len(wallet.Adresses) == 0:\t[addr=%s]", c.Request.RemoteAddr)
					c.JSON(code, gin.H{
						"code":    http.StatusBadRequest,
						"message": msgErrNoWallet,
						"wallet":  wv,
					})
					return
				}

				for _, address := range wallet.Adresses {

					var err error
					addr := ethcommon.HexToAddress(address.Address)
					addressInfo := ethcommon.AddressInfo{}
					// ercAddres := &ethpb.ERC20Address{
					// 	Address:      address.Address,
					// 	OnlyBalances: true,
					// }

					switch networkId {
					case currencies.ETHMain:
						addressInfo, err = restClient.ETH.GetAddressInfo(addr)
						// erc20Info, err = restClient.ETH.CliMain.GetERC20Info(context.Background(), ercAddres)
					default:
						c.JSON(code, gin.H{
							"code":    http.StatusBadRequest,
							"message": msgErrMethodNotImplennted,
							"wallet":  wv,
						})
						return
					}

					if err != nil {
						restClient.log.Errorf("EventGetAdressNonce || EventGetAdressBalance: %v", err.Error())
					}

					totalBalance = addressInfo.TotalBalance.String()
					pendingBalance = addressInfo.PendingBalance.String()

					if addressInfo.PendingBalance.Uint64() != addressInfo.TotalBalance.Uint64() {
						pending = true
						address.LastActionTime = time.Now().Unix()
					}

					if addressInfo.PendingBalance.Uint64() == addressInfo.TotalBalance.Uint64() {
						pendingBalance = "0"
					}

					waletNonce = uint64(addressInfo.Nonce) //.GetNonce()

					av = append(av, ETHAddressVerbose{
						LastActionTime: address.LastActionTime,
						Address:        address.Address,
						AddressIndex:   address.AddressIndex,
						Amount:         totalBalance,
						Nonce:          waletNonce,
						// ERC20Balances:  erc20Info.Balances,
					})

				}

				wv = append(wv, WalletVerboseETH{
					WalletIndex:    wallet.WalletIndex,
					CurrencyID:     wallet.CurrencyID,
					NetworkID:      wallet.NetworkID,
					WalletName:     wallet.WalletName,
					LastActionTime: wallet.LastActionTime,
					DateOfCreation: wallet.DateOfCreation,
					Nonce:          waletNonce,
					Balance:        totalBalance,
					PendingBalance: pendingBalance,
					VerboseAddress: av,
					Pending:        pending,
					Broken:         wallet.BrokenStatus,
				})
				av = []ETHAddressVerbose{}
			}

		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrChainIsNotImplemented,
			})
			return
		}

		c.JSON(code, gin.H{
			"code":    code,
			"message": message,
			"wallet":  wv,
		})
	}
}

type WalletVerboseETH struct {
	CurrencyID     int                 `json:"currencyid"`
	NetworkID      int                 `json:"networkid"`
	WalletIndex    int                 `json:"walletindex"`
	WalletName     string              `json:"walletname"`
	LastActionTime int64               `json:"lastactiontime"`
	DateOfCreation int64               `json:"dateofcreation"`
	Nonce          uint64              `json:"nonce"`
	PendingBalance string              `json:"pendingbalance"`
	Balance        string              `json:"balance"`
	VerboseAddress []ETHAddressVerbose `json:"addresses"`
	Pending        bool                `json:"pending"`
	Syncing        bool                `json:"issyncing"`
	Broken         int                 `json:"brokenStatus"`
}

type ETHAddressVerbose struct {
	LastActionTime int64  `json:"lastactiontime"`
	Address        string `json:"address"`
	AddressIndex   int    `json:"addressindex"`
	Amount         string `json:"amount"`
	Nonce          uint64 `json:"nonce,omitempty"`
	// ERC20Balances  []*ethpb.ERC20Balances `json:"erc20balances"`
}

type StockExchangeRate struct {
	ExchangeName   string `json:"exchangename"`
	FiatEquivalent int    `json:"fiatequivalent"`
	TotalAmount    int    `json:"totalamount"`
}

type TopIndex struct {
	CurrencyID int `json:"currencyid"`
	NetworkID  int `json:"networkid"`
	TopIndex   int `json:"topindex"`
}

func findTopIndexes(wallets []store.Wallet) []TopIndex {
	top := map[TopIndex]int{} // currency id -> topindex
	topIndex := []TopIndex{}
	for _, wallet := range wallets {
		top[TopIndex{wallet.CurrencyID, wallet.NetworkID, 0}]++
	}
	for value, maxIndex := range top {
		topIndex = append(topIndex, TopIndex{
			CurrencyID: value.CurrencyID,
			NetworkID:  value.NetworkID,
			TopIndex:   maxIndex,
		})
	}
	return topIndex
}

func (restClient *RestClient) GetUserByContext(c *gin.Context) (store.User, error) {
	var user store.User

	token, err := getToken(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": msgErrHeaderError,
		})
		return user, err
	}

	query := bson.M{"devices.JWT": token}
	if err := restClient.userStore.FindUser(query, &user); err != nil {
		restClient.log.Errorf("getAllWalletsVerbose: restClient.userStore.FindUser: %s\t[addr=%s]", err.Error(), c.Request.RemoteAddr)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": msgErrUserNotFound,
			"wallets": []interface{}{},
		})
	}

	return user, err
}

func (restClient *RestClient) getAllWalletsVerbose() gin.HandlerFunc {
	return func(c *gin.Context) {
		var wv []interface{}
		user, err := restClient.GetUserByContext(c)
		if err != nil {
			restClient.log.Errorf("Failed to get user from context creds, [%s]", err.Error())
			return
		}

		topIndexes := findTopIndexes(user.Wallets)

		for _, wallet := range user.Wallets {
			switch wallet.CurrencyID {
			case currencies.Ether:
				var av []ETHAddressVerbose
				var pending bool
				var walletNonce uint64
				var totalBalance string
				var pendingBalance string
				// var erc20Info *ethpb.ERC20Info
				// TODO: rewrite this method and refactor
				for _, address := range wallet.Adresses {
					var err error
					addr := ethcommon.HexToAddress(address.Address)
					addressInfo := ethcommon.AddressInfo{}

					switch wallet.NetworkID {
					case currencies.ETHMain:
						addressInfo, err = restClient.ETH.GetAddressInfo(addr)
						// erc20Info, err = restClient.ETH.CliMain.GetERC20Info(context.Background(), ercAddres)
					default:
						c.JSON(http.StatusBadRequest, gin.H{
							"code":    http.StatusBadRequest,
							"message": msgErrMethodNotImplennted,
							"wallet":  wv,
						})
						return
					}

					if err != nil {
						restClient.log.Errorf("EventGetAdressNonce || EventGetAdressBalance: %v", err.Error())
					}

					totalBalance = addressInfo.TotalBalance.String()
					pendingBalance = addressInfo.PendingBalance.String()

					if addressInfo.PendingBalance.Uint64() != addressInfo.TotalBalance.Uint64() {
						pending = true
					}

					if addressInfo.PendingBalance.Uint64() == addressInfo.TotalBalance.Uint64() {
						pendingBalance = "0"
					}
					walletNonce = uint64(addressInfo.Nonce)

					av = append(av, ETHAddressVerbose{
						LastActionTime: address.LastActionTime,
						Address:        address.Address,
						AddressIndex:   address.AddressIndex,
						Amount:         totalBalance,
						Nonce:          walletNonce,
						// ERC20Balances:  erc20Info.GetBalances(),
					})

				}

				wv = append(wv, WalletVerboseETH{
					WalletIndex:    wallet.WalletIndex,
					CurrencyID:     wallet.CurrencyID,
					NetworkID:      wallet.NetworkID,
					Balance:        totalBalance,
					PendingBalance: pendingBalance,
					Nonce:          walletNonce,
					WalletName:     wallet.WalletName,
					LastActionTime: wallet.LastActionTime,
					DateOfCreation: wallet.DateOfCreation,
					VerboseAddress: av,
					Pending:        pending,
					Broken:         wallet.BrokenStatus,
				})
				av = []ETHAddressVerbose{}
			default:

			}

		}

		c.JSON(http.StatusOK, gin.H{
			"code":       http.StatusOK,
			"message":    http.StatusText(http.StatusOK),
			"wallets":    wv,
			"topindexes": topIndexes,
		})

	}
}

func (restClient *RestClient) getWalletTokenTransactionsHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		restClient.handleGetTransactionHistory(c, true)
	}
}

func (restClient *RestClient) handleGetTransactionHistory(c *gin.Context, useTokens bool) {
	user, err := restClient.GetUserByContext(c)

	if err != nil {
		restClient.requestFailed(c, http.StatusNotFound, "Unknown user", err)
		return
	}

	params := struct {
		walletIndex int    `json:"walletindex"`
		currencyId  int    `json:"currencyid"`
		networkId   int    `json:"networkid"`
		assetType   int    `json:"type,omitempty"`
		token       string `json:"token,omitempty"`
	}{
		assetType: store.AssetTypeMultyAddress,
	}

	err = BindParams(c.Params, &params)
	if err != nil {
		restClient.requestFailed(c, http.StatusBadRequest, "InvalidParameters", err)
		return
	}

	var token *ethcommon.Address
	if useTokens {
		if ethcommon.IsValidHexAddress(params.token) {
			t := ethcommon.HexToAddress(params.token)
			token = &t
		}
	}
	if useTokens && token == nil {
		restClient.requestFailed(c, http.StatusBadRequest, "InvalidParameters", errors.New("token is not set or invalid"))
		return
	}

	history, err := restClient.ETH.GetUserTransactions(user, params.walletIndex, params.currencyId, params.networkId, token)
	if err != nil {
		restClient.requestFailed(c, http.StatusBadRequest, "Failed to get user transactions.", err)
		return
	}

	if params.assetType == store.AssetTypeMultyAddress {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": http.StatusText(http.StatusOK),
			"history": history,
		})
		return
	}

	restClient.requestFailed(c, http.StatusBadRequest, "InvalidParameters", errors.Errorf("Invalid type"))
}

func (restClient *RestClient) getWalletTransactionsHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		restClient.handleGetTransactionHistory(c, false)
	}
}

type TxHistory struct {
	TxID        string               `json:"txid"`
	TxHash      string               `json:"txhash"`
	TxOutScript string               `json:"txoutscript"`
	TxAddress   string               `json:"address"`
	TxStatus    int                  `json:"txstatus"`
	TxOutAmount int64                `json:"txoutamount"`
	TxOutID     int                  `json:"txoutid"`
	WalletIndex int                  `json:"walletindex"`
	BlockTime   int64                `json:"blocktime"`
	BlockHeight int64                `json:"blockheight"`
	TxFee       int64                `json:"txfee"`
	MempoolTime int64                `json:"mempooltime"`
	BtcToUsd    float64              `json:"btctousd"`
	TxInputs    []store.AddresAmount `json:"txinputs"`
	TxOutputs   []store.AddresAmount `json:"txoutputs"`
}

func (restClient *RestClient) resyncWallet() gin.HandlerFunc {
	return func(c *gin.Context) {
		// :currencyid/:networkid/:walletindex
		token, err := getToken(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrHeaderError,
			})
			return
		}

		var derivationPath string
		walletIndex, err := strconv.Atoi(c.Param("walletindex"))
		restClient.log.Debugf("resyncWallet [%d] \t[walletindexr=%s]", walletIndex, c.Request.RemoteAddr)
		if err != nil {
			derivationPath = strings.ToLower(c.Param("walletindex"))
		}

		assetType := store.AssetTypeMultyAddress
		if len(c.Param("type")) > 0 {
			assetType, err = strconv.Atoi(c.Param("type")[1:])
			restClient.log.Debugf("resyncWallet [%d] \t[networkID=%s]", assetType, c.Request.RemoteAddr)
			if err != nil {
				restClient.log.Errorf("resyncWallet: non int asset type:[%d] %s \t[addr=%s]", assetType, err.Error(), c.Request.RemoteAddr)
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    http.StatusBadRequest,
					"message": msgErrDecodeTypeErr,
				})
				return
			}
		}

		currencyID, err := strconv.Atoi(c.Param("currencyid"))
		restClient.log.Debugf("resyncWallet [%d] \t[currencyId=%s]", currencyID, c.Request.RemoteAddr)
		if err != nil {
			restClient.log.Errorf("resyncWallet: non int currency id:[%d] %s \t[addr=%s]", currencyID, err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrDecodeCurIndexErr,
			})
			return
		}

		networkID, err := strconv.Atoi(c.Param("networkid"))
		restClient.log.Debugf("resyncWallet [%d] \t[networkid=%s]", networkID, c.Request.RemoteAddr)
		if err != nil {
			restClient.log.Errorf("resyncWallet: non int networkid index:[%d] %s \t[addr=%s]", networkID, err.Error(), c.Request.RemoteAddr)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrDecodenetworkidErr,
			})
			return
		}

		user := store.User{}
		sel := bson.M{"devices.JWT": token}
		err = restClient.userStore.FindUser(sel, &user)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": msgErrUserNotFound,
			})
			return
		}

		walletToResync := store.Wallet{}
		for _, wallet := range user.Wallets {
			if wallet.CurrencyID == currencyID && wallet.NetworkID == networkID && wallet.WalletIndex == walletIndex {
				walletToResync = wallet
			}
		}

		if len(walletToResync.Adresses) == 0 && len(derivationPath) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": msgErrUserHaveNoTxs,
			})
			return
		}

		switch currencyID {

		case currencies.Ether:
			// var resync ethpb.NodeCommunicationsClient
			if networkID == currencies.ETHMain {
				// resync = restClient.ETH.CliMain
				go func() {
					for _, address := range walletToResync.Adresses {
						err = restClient.ETH.ResyncAddress(ethcommon.HexToAddress(address.Address))
						if err != nil {
							restClient.log.Errorf("resyncWallet case currencies.Ether:ETHMain: %v", err.Error())
						}
					}
				}()
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": http.StatusText(http.StatusOK),
		})

	}
}

func (restClient *RestClient) requestFailed(c *gin.Context, responseCode int, responseMessage string, err error) {
	r := c.Request
	restClient.log.Errorf("Request '%s %s' from %s failed with error: %+v", r.Method, r.RequestURI, r.RemoteAddr, err)

	c.JSON(responseCode, gin.H{
		"code":     responseCode,
		"message":  responseMessage,
		"error":    err.Error(),
	})
}
