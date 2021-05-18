package transaction

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	hezcommon "github.com/hermeznetwork/hermez-node/common"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/jeffprestes/hermez-go-sdk/account"
)

// NewHermezAPITxRequest convert L2 tx to Hermez API request model
func NewHermezAPITxRequest(poolTx *hezcommon.PoolL2Tx, token hezcommon.Token) APITx {
	toIdx := "hez:ETH:0"
	if poolTx.ToIdx > 0 {
		toIdx = IdxToHez(poolTx.ToIdx, token.Symbol)
	}
	toEth := ""
	if poolTx.ToEthAddr != hezcommon.EmptyAddr {
		toEth = ethAddrToHez(poolTx.ToEthAddr)
	}
	toBJJ := BjjToString(poolTx.ToBJJ)
	if poolTx.ToBJJ != hezcommon.EmptyBJJComp {
		toBJJ = BjjToString(poolTx.ToBJJ)
	}
	return APITx{
		TxID:      poolTx.TxID,
		Type:      string(poolTx.Type),
		TokenID:   uint32(poolTx.TokenID),
		FromIdx:   IdxToHez(poolTx.FromIdx, token.Symbol),
		ToIdx:     toIdx,
		ToEthAddr: toEth,
		ToBJJ:     toBJJ,
		Amount:    poolTx.Amount.String(),
		Fee:       uint64(poolTx.Fee),
		Nonce:     uint64(poolTx.Nonce),
		Signature: poolTx.Signature.String(),
	}
}

// IdxToHez convert idx to hez idx
func IdxToHez(idx hezcommon.Idx, tokenSymbol string) string {
	return "hez:" + tokenSymbol + ":" + strconv.Itoa(int(idx))
}

// EthAddrToHez convert eth address to hez address
func ethAddrToHez(addr common.Address) string {
	return "hez:" + addr.String()
}

// BjjToString convert the BJJ public key to string
func BjjToString(bjj babyjub.PublicKeyComp) string {
	pkComp := [32]byte(bjj)
	sum := pkComp[0]
	for i := 1; i < len(pkComp); i++ {
		sum += pkComp[i]
	}
	bjjSum := append(pkComp[:], sum)
	return "hez:" + base64.RawURLEncoding.EncodeToString(bjjSum)
}

// MarshalTransaction marshal transaction information into a Hermez transaction API request
func MarshalTransaction(itemToTransfer string,
	senderAcctDetails account.AccountAPIResponse,
	receipientAcctDetails account.AccountAPIResponse,
	bjjWallet account.BJJWallet,
	amount *big.Int,
	feeSelector int,
	chainID int) (apiTxRequest APITx, err error) {

	var token hezcommon.Token
	var nonce hezcommon.Nonce
	var fromIdx, toIdx hezcommon.Idx

	// Get from account Token and nonce details from sender account
	for _, account := range senderAcctDetails.Accounts {
		if strings.ToUpper(account.Token.Symbol) == itemToTransfer {
			token.TokenID = hezcommon.TokenID(account.Token.ID)
			token.Symbol = account.Token.Symbol
			nonce = hezcommon.Nonce(account.Nonce + 1)
			tempAccountsIdx := strings.Split(account.AccountIndex, ":")
			if len(tempAccountsIdx) == 3 {
				tempAccIdx, errAtoi := strconv.Atoi(tempAccountsIdx[2])
				if errAtoi != nil {
					err = fmt.Errorf("[MarshalTransaction] Error getting sender account index. Account: %+v - Error: %s\n", account, err.Error())
					return
				}
				fromIdx = hezcommon.Idx(tempAccIdx)
			}
		}
	}

	// Get from account Token and nonce details from receipient account
	for _, account := range receipientAcctDetails.Accounts {
		if strings.ToUpper(account.Token.Symbol) == itemToTransfer {
			tempAccountsIdx := strings.Split(account.AccountIndex, ":")
			if len(tempAccountsIdx) == 3 {
				tempAccIdx, errAtoi := strconv.Atoi(tempAccountsIdx[2])
				if errAtoi != nil {
					log.Printf("[MarshalTransaction] Error getting receipient account index. Account: %+v - Error: %s\n", account, err.Error())
					return
				}
				toIdx = hezcommon.Idx(tempAccIdx)
			}
		}
	}

	// If there is no account created to this specific token stop the code
	if len(fromIdx.String()) < 1 {
		err = fmt.Errorf("[MarshalTransaction] There is no account to this user %s for this Token %s", bjjWallet.HezBjjAddress, itemToTransfer)
		return
	}

	// fee := hezcommon.FeeSelector(100)
	fee := hezcommon.FeeSelector(uint8(feeSelector)) // 10.2%

	tx := new(hezcommon.PoolL2Tx)
	tx.FromIdx = fromIdx
	tx.ToEthAddr = hezcommon.EmptyAddr
	tx.ToBJJ = hezcommon.EmptyBJJComp
	tx.ToIdx = toIdx
	tx.Amount = amount
	tx.Fee = fee
	tx.TokenID = token.TokenID
	tx.Nonce = nonce
	tx.Type = hezcommon.TxTypeTransfer

	tx, err = hezcommon.NewPoolL2Tx(tx)
	if err != nil {
		err = fmt.Errorf("[MarshalTransaction] Error creating L2 TX Pool object. TX: %+v - Error: %s\n", tx, err.Error())
		return
	}

	txHash, err := tx.HashToSign(uint16(chainID))
	if err != nil {
		err = fmt.Errorf("[MarshalTransaction] Error generating tx hash. TX: %+v - Error: %s\n", tx, err.Error())
		return
	}

	signedTx := bjjWallet.PrivateKey.SignPoseidon(txHash)
	tx.Signature = signedTx.Compress()

	apiTxRequest = NewHermezAPITxRequest(tx, token)
	return
}
