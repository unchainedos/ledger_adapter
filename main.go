package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rpcUrl     = "http://127.0.0.1:9654/ext/bc/2v7DxDguPLY8fqFL7Jxry82MWomtkPiUSFnhXvxQJfA4T2BHmn/rpc"
	privateKey = "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"
)

var addr common.Address = common.HexToAddress("0x4Ac1d98D9cEF99EC6546dEd4Bd550b0b287aaD6D")

const contractABI = `[
  {
    "inputs": [
      {
        "internalType": "string",
        "name": "_addition",
        "type": "string"
      }
    ],
    "name": "addMessage",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "getMessage",
    "outputs": [
      {
        "internalType": "string",
        "name": "",
        "type": "string"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "k",
    "outputs": [
      {
        "internalType": "int256",
        "name": "",
        "type": "int256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "message",
    "outputs": [
      {
        "internalType": "string",
        "name": "",
        "type": "string"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "string",
        "name": "_newMessage",
        "type": "string"
      }
    ],
    "name": "setMessage",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

func getHelloWorld(client *ethclient.Client) (string, error) {
	parsedABI, err := abi.JSON(strings.NewReader(contractABI))
	if err != nil {
		return "", fmt.Errorf("failed to parse ABI: %v", err)
	}

	data, err := parsedABI.Pack("getMessage")
	if err != nil {
		return "", fmt.Errorf("failed to pack data: %v", err)
	}

	res, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &addr,
		Data: data,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("contract call failed: %v", err)
	}

	var message string
	err = parsedABI.UnpackIntoInterface(&message, "getMessage", res)
	if err != nil {
		return "", fmt.Errorf("failed to unpack with ABI: %v", err)
	}

	return message, nil
}

func setMessage(client *ethclient.Client, x string) error {
	parsedABI, err := abi.JSON(strings.NewReader(contractABI))
	if err != nil {
		return err
	}
	data, err := parsedABI.Pack("setMessage", x)
	if err != nil {
		return err
	}
	privateKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %v", err)
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get gas price: %v", err)
	}
	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From: fromAddress,
		To:   &addr,
		Data: data,
	})
	if err != nil {
		gasLimit = 100000
	}
	tx := types.NewTransaction(
		nonce,
		addr,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		data,
	)
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %v", err)
	}
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %v", err)
	}
	return nil
}

func main() {
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	if err != nil {
		fmt.Print(err.Error())
	}

	x := func(w http.ResponseWriter, r *http.Request) {
		res, err := getHelloWorld(client)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(res))
		return
	}

	y := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{error:"page not found"}`, http.StatusNotFound)
			return
		}
		x := map[string]string{
			"NewWord": "",
		}
		m, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(m, &x)
		if err != nil {
			http.Error(w, `{error:"expected json"}`, http.StatusBadRequest)
			return
		}
		err = setMessage(client, x["NewWord"])
		if err != nil {
			http.Error(w, `{error:"unexpected behaviour of subnet"}`, http.StatusBadGateway)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("created"))
		return
	}

	http.HandleFunc("/", x)
	http.HandleFunc("/setstring", y)
	http.ListenAndServe(":8080", nil)
}
