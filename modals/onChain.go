package modals

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func InsertIntoSecWallets(walletAddress string, walletKey string, usdtAmount float64, secWallets *mongo.Collection) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	usdtAmountString := fmt.Sprintf("%f", usdtAmount)
	accountJson := bson.M{
		"ADD":  walletAddress,
		"EKEY": walletKey,
		"AMT":  usdtAmountString,
	}

	_, err := secWallets.InsertOne(ctx, accountJson)
	return err == nil
}

func CreateAndSaveNewAddress(walletAddress string, accountCollection *mongo.Collection) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	isCreated, WalletData, message := CreateWallet()
	if !isCreated {
		return false, message
	}
	newAddress := WalletData.Address
	newKey := WalletData.Key
	update := bson.M{"$set": bson.M{
		"EADD": strings.ToLower(newAddress),
		"EKEY": newKey,
	}}
	filter := bson.M{"ID": walletAddress}
	_, err := accountCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, ""
	}
	return true, strings.ToLower(newAddress)
}
