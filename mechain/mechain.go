package main

/**
 *  Copyright (C) 2021 HyperBench.
 *  SPDX-License-Identifier: Apache-2.0
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 * @brief Mechain the implementation of client.Blockchain based on mechain network
 * @file mechain.go
 * @author: caiyongtao
 * @date 2024-10-28
 */

import (
	"bufio"
	"context"
	"cosmossdk.io/math"
	"encoding/json"
	types2 "github.com/evmos/evmos/v12/sdk/types"
	"github.com/zkMeLabs/mechain-go-sdk/client"
	"math/rand"

	"os"
	"path/filepath"
	"strconv"
	"time"

	fcom "github.com/hyperbench/hyperbench-common/common"

	"github.com/hyperbench/hyperbench-common/base"
	"github.com/spf13/cast"
	"github.com/spf13/viper"

	"github.com/zkMeLabs/mechain-go-sdk/types"
)

const (
	// evm
	EVM = "evm"
	ABI = "abi"
	BIN = "bin"
	SOL = "sol"

	// go
	GO = "go"

	// keystore
	KEYSTORE = "keys"

	// config
	CONFIG = "mechain.toml"

	// default bcname
	DEFAULTBCNAME = "zkme"
)

// DirPath direction path
type DirPath string

// Mechain the implementation of client.Blockchain based on mechain network
type Mechain struct {
	*base.BlockchainBase
	nodeURL       string
	account       *types.Account
	accounts      map[string]*types.Account
	contractNames []string
	contractType  string
	instant       int

	mechainClient client.IClient
}

// Msg the message info of context
type Msg struct {
	ContractNames []string                  `json:"ContractNames,omitempty"`
	ContractType  string                    `json:"ContractType,omitempty"`
	Accounts      map[string]*types.Account `json:"Accounts,omitempty"`
}

func New(blockchainBase *base.BlockchainBase) (mechain interface{}, err error) {
	log := fcom.GetLogger("mechain")
	// read zkme chain rpc config
	chainConfig, err := os.Open(filepath.Join(blockchainBase.ConfigPath, CONFIG))
	if err != nil {
		log.Errorf("load zkme configuration fialed: %v", err)
		return nil, err
	}
	_ = viper.MergeConfig(chainConfig)
	nodeURL := viper.GetString("rpc.node") + ":" + viper.GetString("rpc.port")

	// get account from file
	Accounts := make(map[string]*types.Account)
	var Account *types.Account
	keystorePath := filepath.Join(blockchainBase.ConfigPath, KEYSTORE)
	file, err := os.Open(keystorePath)
	defer func() {
		_ = file.Close()
	}()
	if err != nil {
		log.Errorf("open keystore failed: %v", err)
	} else {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			account, err := types.NewAccountFromPrivateKey("", line)
			if err != nil {
				log.Errorf("get account failed : %v", err)
				return nil, err
			}
			if file.Name() == "main" {
				Account = account
			}
			Accounts[account.GetAddress().String()] = account
		}
	}

	c, err := client.New("mechain_5151-1", "", client.Option{})
	if err != nil {
		log.Error("init mechain client failed")
		return nil, err
	}
	mechain = &Mechain{
		BlockchainBase: blockchainBase,
		nodeURL:        nodeURL,
		account:        Account,
		accounts:       Accounts,
		instant:        cast.ToInt(blockchainBase.Options["instant"]),
		mechainClient:  c,
	}
	return
}

// DeployContract deploy contract to zkme chain network
func (x *Mechain) DeployContract() error {
	return nil
}

// Invoke invoke contract with funcName and args in zkme chain network
func (x *Mechain) Invoke(invoke fcom.Invoke, ops ...fcom.Option) *fcom.Result {
	return nil
}

// Confirm check the result of `Invoke` or `Transfer`
func (x *Mechain) Confirm(result *fcom.Result, ops ...fcom.Option) *fcom.Result {
	// if transaction id is invalid or state of result is failure, return
	if result.UID == "" ||
		result.UID == fcom.InvalidUID ||
		result.Status != fcom.Success ||
		result.Label == fcom.InvalidLabel {
		return result
	}
	// query transaction in mechain network by its txId
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	cancel()
	tx, err := x.mechainClient.WaitForTx(ctx, result.UID)
	// record invoke tx send time
	result.ConfirmTime = time.Now().UnixNano()
	if err != nil {
		// if query is failed, set state of result as unknown
		x.Logger.Errorf("query failed: %v", err)
		result.Status = fcom.Unknown
		return result
	}
	// set status, write time, ret of result based on query response
	result.Status = fcom.Confirm
	result.WriteTime = int64(tx.TxResult.Code) // todo
	result.Ret = []interface{}{tx.TxResult}
	return result
}

// Verify check the relative time of transaction
func (x *Mechain) Verify(result *fcom.Result, ops ...fcom.Option) *fcom.Result {
	// mechain verification is the same of confirm
	return x.Confirm(result)
}

// Transfer transfer a amount of money from a account to  another
func (x *Mechain) Transfer(args fcom.Transfer, ops ...fcom.Option) (result *fcom.Result) {
	// record transfer tx build time
	buildTime := time.Now().UnixNano()
	// get account by its address and convert amount from int64 type to string
	from, to, amount := x.accounts[args.From], x.accounts[args.To], uint64(args.Amount)
	// if from account is nil, use the main account
	if from == nil {
		from = x.account
	}
	// if to account is nil
	if to == nil {
		// if accounts initiated before running, use accounts randomly, or create a new one
		if x.Options["initAccount"] != nil {
			to = x.accounts[strconv.Itoa(rand.Intn(int(x.Options["initAccount"].(float64))))]
		} else {
			to, _, _ = types.NewAccount("")
			x.accounts[args.To] = to
		}
	}
	// send the transfer tx
	x.mechainClient.SetDefaultAccount(from)
	txHash, err := x.mechainClient.Transfer(context.TODO(), to.GetAddress().String(), math.NewIntFromUint64(amount), types2.TxOption{})
	// record transfer tx send time
	sendTime := time.Now().UnixNano()
	if err != nil {
		// if tx is failed, return failure result
		x.Logger.Errorf("transfer error: %v", err)
		return &fcom.Result{
			Label:     fcom.BuiltinTransferLabel,
			UID:       fcom.InvalidUID,
			Ret:       []interface{}{},
			Status:    fcom.Failure,
			BuildTime: buildTime,
		}
	}
	// if tx is successful, return success result
	return &fcom.Result{
		Label:     fcom.BuiltinTransferLabel,
		UID:       txHash,
		Ret:       []interface{}{},
		Status:    fcom.Success,
		BuildTime: buildTime,
		SendTime:  sendTime,
	}
}

// SetContext set test group context in go client
func (x *Mechain) SetContext(context string) error {
	x.Logger.Debugf("prepare msg: %v", context)
	msg := &Msg{}

	if context == "" {
		x.Logger.Infof("Prepare nothing")
		return nil
	}

	err := json.Unmarshal([]byte(context), msg)
	if err != nil {
		x.Logger.Errorf("can not unmarshal msg: %v \n err: %v", context, err)
		return err
	}
	x.contractNames, x.contractType, x.accounts = msg.ContractNames, msg.ContractType, msg.Accounts
	return nil
}

// GetContext generate TxContext
func (x *Mechain) GetContext() (string, error) {
	err := x.initAccount(x.instant)
	if err != nil {
		x.Logger.Error(err)
	}
	msg := &Msg{
		ContractNames: x.contractNames,
		ContractType:  x.contractType,
		Accounts:      x.accounts,
	}

	bytes, err := json.Marshal(msg)

	return string(bytes), err
}

// Statistic statistic remote node performance
func (x *Mechain) Statistic(statistic fcom.Statistic) (*fcom.RemoteStatistic, error) {
	// initiate from,to timestamp, txNum, blockNum and duration
	from, to := statistic.From.TimeStamp, statistic.To.TimeStamp
	txNum, blockNum := 0, 0
	duration := float64(to - from)
	// query each block to count txNum from start block to end block
	for i := statistic.From.BlockHeight; i <= statistic.To.BlockHeight; i++ {
		blockResult, err := x.mechainClient.GetBlockByHeight(context.TODO(), i)
		if err != nil {
			return nil, err
		}
		blockNum++
		txNum += len(blockResult.Txs)
	}
	// return result
	ret := &fcom.RemoteStatistic{
		Start:    from,
		End:      to,
		BlockNum: blockNum,
		TxNum:    txNum,
		CTps:     float64(txNum) * float64(time.Second) / duration,
		Bps:      float64(blockNum) * float64(time.Second) / duration,
	}
	return ret, nil
}

// LogStatus records block height and time
func (x *Mechain) LogStatus() (chainInfo *fcom.ChainInfo, err error) {
	bk, err := x.mechainClient.GetStatus(context.TODO())
	if err != nil {
		return nil, err
	}
	return &fcom.ChainInfo{BlockHeight: bk.SyncInfo.LatestBlockHeight, TimeStamp: time.Now().UnixNano()}, err
}

// ResetContext reset test group context in go client
func (x *Mechain) ResetContext() error {
	return nil
}

func (x *Mechain) Option(options fcom.Option) error {
	return nil
}

// initAccount init the number of account
func (x *Mechain) initAccount(count int) (err error) {
	if count <= 0 {
		return nil
	}
	for i := 0; i < count; i++ {
		acc, _, _ := types.NewAccount("")
		x.mechainClient.SetDefaultAccount(x.account)
		_, err := x.mechainClient.Transfer(context.TODO(), acc.GetAddress().String(), math.NewInt(10000000), types2.TxOption{})
		if err != nil {
			x.Logger.Error(err)
		}
		x.accounts[strconv.Itoa(i)] = acc
	}
	return err
}
