package modals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Wallet struct {
	EADD string `"bson":  "EADD"` // Tulobyte Balance
	TADD string `"bson":  "TADD"` // Polygon USDT Bal
}

func RecoverAccount(r *http.Request) (string, string) {
	db, err := ConnectDB()

	if err != nil {
		return "false", "API Database Error"
	}
	// Database collections
	accounts := db.Collection("tb_accounts")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "false", "API Database Error"
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return "false", "Request Empty"
	}
	// Convert JSON body to map[string]string
	var data map[string]string
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "false", "API Database Error"
	}

	walletsDetailString, exists := data["data"]
	if !exists {
		return "false", "Request Malformed No Data"
	}
	walletsDetailList := strings.Split(walletsDetailString, ",")
	if len(walletsDetailList) != 2 {
		return "false", "Request Malformed"
	}
	address := walletsDetailList[0]
	walletKey := walletsDetailList[1]
	validKey := CheckKey(walletKey, address)
	if !validKey {
		return "false", "You're trying to bypass"
	}
	isReferal, _, _ := checkReferal(accounts, address)
	if !isReferal {
		return "false", "No Account Found"
	}
	walletData, isWallet := getWalletDetail(address, accounts)
	if !isWallet {
		return "false", "No Account Found"
	}
	return "true", fmt.Sprintf("%s,%s", walletData.EADD, walletData.TADD)
}

func getWalletDetail(address string, tbWallets *mongo.Collection) (Wallet, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"ID": address}
	var result Wallet
	err := tbWallets.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, false
		}
	}
	return result, true
}
func CheckKey(walletKey string, caddress string) bool {
	// Convert hex string to bytes
	privateKeyBytes, err := hex.DecodeString(walletKey)
	if err != nil {
		return false
	}

	// Get ECDSA private key
	privKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)

	// Get uncompressed public key
	pubKey := privKey.PubKey().SerializeUncompressed()

	// Hash the public key (skip the first byte - 0x04)
	hash := crypto.Keccak256(pubKey[1:])

	// Take last 20 bytes
	address := hash[12:]

	// Prepend Tron address prefix 0x41
	tronAddress := append([]byte{0x41}, address...)
	// Base58Check encode
	base58CheckAddr := base58.Encode(addCheckSum(tronAddress))
	if base58CheckAddr == caddress {
		return true
	} else {
		return false
	}
}

func addCheckSum(input []byte) []byte {
	first := sha256.Sum256(input)
	second := sha256.Sum256(first[:])
	return append(input, second[:4]...)
}
