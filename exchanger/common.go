/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/

package exchanger

type Exchanger struct {
	Name string
}

type CurrencyExchanger struct {
	Name string
}

type ExchangeTransaction struct {
	Id            string      `json:"id"`
	PayInAddress  string      `json:"payinAddress"`
	PayOutAddress string      `json:"payoutAddress"`
	Error         interface{} `json:"error"`
}

type BasicExchangeConfiguration struct {
	Name     string      `json:"name"`
	IsActive bool        `json:"isActive"`
	Config   interface{} `json:"config"`
}
