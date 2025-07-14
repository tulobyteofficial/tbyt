package modals

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateAccount(r *http.Request) (string, string) {
	db, err := ConnectDB()

	if err != nil {
		return "false", "API Database Error"
	}
	// Database collections
	accounts := db.Collection("tb_accounts")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "false", "API Request Body Error"
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

	walletsDetail := make(map[string]string)
	walletsDetail["ID"] = walletsDetailList[0]
	walletsDetail["REF"] = walletsDetailList[1]
	// Check referral
	var addREF = false
	var refs []string
	var isReferal bool
	var message string
	if walletsDetail["REF"] != "NIL" {
		isReferal, message, refs = checkReferal(accounts, walletsDetail["REF"])
		if !isReferal {
			return "false", message
		}
		addREF = true
	} else {
		walletsDetail["REF"] = ""
	}

	// Insert a new account
	isCreated, backendWalletData, message := CreateWallet()
	if !isCreated {
		return "false", message
	}
	isInserted := insertAccount(walletsDetail, accounts, addREF, refs, backendWalletData)
	if !isInserted {
		return "false", "Server Database Error"
	}

	return "true", backendWalletData.Address
}

func checkReferal(tbWallets *mongo.Collection, referal string) (bool, string, []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"ID": referal}
	var result User
	err := tbWallets.FindOne(ctx, filter).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return false, "Referrals Address not found", []string{}
	} else if err != nil {
		return false, "Can't Get Referrals Address", []string{}
	} else if len(result.REFS) > 20 {
		return false, "Referral Limit Exceeded (max. 20), Try another", []string{}
	}
	return true, "", result.REFS
}

func insertAccount(walletsDetail map[string]string, accounts *mongo.Collection, isAddREF bool, REFS []string, backendWalletData CrWallet) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	upperCaseAddress := strings.ToLower(backendWalletData.Address)
	accountJson := bson.M{
		"ID":      walletsDetail["ID"],
		"EADD":    upperCaseAddress,
		"EKEY":    backendWalletData.Key,
		"TBT":     "0.00",
		"POS":     "0.00",
		"ERC":     "0.00",
		"NPT":     "0.00",
		"NPTP":    "0.00",
		"REFB":    walletsDetail["REF"],
		"REFS":    []string{},
		"REFRESH": "0,0",
	}

	_, err := accounts.InsertOne(ctx, accountJson)
	if err != nil {
		return false
	}
	refereeAdded := AddReferrals(accounts, walletsDetail["ID"], walletsDetail["REF"], REFS)
	if !refereeAdded {
		return false
	}
	return true
}

func AddReferrals(accounts *mongo.Collection, refree string, referer string, REFSCount []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// store refree in referer
	newRef := append(REFSCount, refree)
	filter := bson.M{"ID": referer}
	update := bson.M{"$set": bson.M{"REFS": newRef}}
	_, err := accounts.UpdateOne(ctx, filter, update)
	return err == nil
}

type CrWallet struct {
	Address string
	Key     string
}

func CreateWallet() (bool, CrWallet, string) {
	var CreatedWallet CrWallet
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return false, CrWallet{}, "Can't generate private key"
	}

	// Convert private key to bytes
	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex := fmt.Sprintf("0x%x", privateKeyBytes)

	// Get public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return false, CrWallet{}, "Can't generate backend wallets"
	}

	// Get wallet address
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	CreatedWallet.Address = address
	CreatedWallet.Key = privateKeyHex

	return true, CreatedWallet, ""
}
