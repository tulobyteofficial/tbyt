package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"tbapi/modals"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TransferAssets(r *http.Request) (string, string) {
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
	if len(walletsDetailList) != 5 {
		return "false", "Request Malformed"
	}
	address := walletsDetailList[0]
	walletKey := walletsDetailList[1]
	recipientAddress := strings.ToLower(walletsDetailList[2])

	debitValue := walletsDetailList[3]
	assetChoice := walletsDetailList[4]
	validKey := modals.CheckKey(walletKey, address)
	if !validKey {
		return "false", "You're trying to bypass"
	}

	db, err := modals.ConnectDB()

	if err != nil {
		return "false", "API Database Error"
	}
	// Database collections
	accounts := db.Collection("tb_accounts")

	accountData, isFound := modals.GetAccountData(address, accounts)
	if !isFound {
		return "false", "No Account Found"
	}

	isInternal, Bal, rID := isInternalAddress(recipientAddress, assetChoice, accounts)
	if isInternal {
		isTransfered, insertedID := SendCurrencyInternal(assetChoice, accountData, recipientAddress, debitValue, db, address, Bal, rID)
		if isTransfered {
			return "true", insertedID
		} else {
			return "false", insertedID
		}
	} else {
		return "false", Bal
	}
}

func SendCurrencyInternal(
	assetType string,
	accountData modals.User,
	recipientAddress string,
	debitValue string,
	db *mongo.Database,
	senderAddress string,
	recipientBal string,
	rID string,
) (bool, string) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cType := ""
	cAdd := "EADD"
	cBal := ""
	if assetType == "USDT-PoS" {
		cType = "POS"
		cBal = accountData.POS
	} else if assetType == "TBYT-PoS" {
		cType = "TBT"
		cBal = accountData.TBT
	} else if assetType == "USDT-ERC" {
		cType = "ERC"
		cBal = accountData.ERC
	}

	if recipientAddress == accountData.EADD {
		return false, "You cannot transfer funds to your own wallet address. Please enter a different recipient address"
	}

	recipientCurrentBalance, err := strconv.ParseFloat(recipientBal, 64)
	if err != nil {
		return false, "Can't convert Recipient Balance to Integer "
	}

	debitValueFloat, err := strconv.ParseFloat(debitValue, 64)
	if err != nil {
		return false, "Can't convert Debit Balance to Integer"
	}

	senderCurrentBalance, err := strconv.ParseFloat(cBal, 64)
	if err != nil {
		return false, "Can't convert Sender Balance to Integer"
	}

	if senderCurrentBalance < debitValueFloat {
		return false, "Insufficient Balance"
	}

	senderRemainingBalance := senderCurrentBalance - debitValueFloat
	recipientNewBalance := debitValueFloat + recipientCurrentBalance

	accounts := db.Collection("tb_accounts")
	// Deducted from sender
	filter := bson.M{"ID": senderAddress}
	senderRemainingBalanceString := fmt.Sprintf("%.5f", senderRemainingBalance)
	update := bson.M{"$set": bson.M{cType: senderRemainingBalanceString}}
	_, err = accounts.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, "Can't Withdraw Balance"
	}
	transferOrders := db.Collection("transferOrders")
	accountTxnOrders, isFound := getLastTenOrders(rID, transferOrders, "1")
	if isFound {
		if accountTxnOrders == nil {
			patformInfoCollection := db.Collection("platformInfo")
			UpdateHolders(patformInfoCollection)
		}
	}
	// Transfer to other account

	filter2 := bson.M{cAdd: recipientAddress}
	recipientNewBalanceString := fmt.Sprintf("%.5f", recipientNewBalance)

	update2 := bson.M{"$set": bson.M{cType: recipientNewBalanceString}}
	_, err = accounts.UpdateOne(ctx, filter2, update2)
	if err != nil {
		// revert if not added

		update := bson.M{"$set": bson.M{cType: fmt.Sprintf("%.5f", senderCurrentBalance)}}
		_, err = accounts.UpdateOne(ctx, filter, update)
		if err != nil {
			return false, "Can't send to recipient account"
		}
	}
	debitValueFloatString := fmt.Sprintf("%.5f", debitValueFloat)

	result, message := UpdateOrderList("0.00", senderAddress, rID, recipientAddress, debitValueFloatString, cType, "INT", transferOrders)
	if !result {
		// revert if not added
		update := bson.M{"$set": bson.M{cType: fmt.Sprintf("%.5f", senderCurrentBalance)}}
		accounts.UpdateOne(ctx, filter, update)

		update2 := bson.M{"$set": bson.M{cType: fmt.Sprintf("%.5f", recipientCurrentBalance)}}
		accounts.UpdateOne(ctx, filter2, update2)

		return false, message
	}

	return true, message

}

func UpdateHolders(patformInfoCollection *mongo.Collection) {
	platformInfo, isFound := modals.GetPlatformInfo()
	if !isFound {
		fmt.Print("Not found")
		return
	}
	holders, err := strconv.ParseFloat(platformInfo.Holder, 64)
	if err != nil {
		fmt.Print(err)
		return
	}
	newHolder := holders + 1
	newHolderStr := fmt.Sprintf("%f", newHolder)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	update := bson.M{"$set": bson.M{
		"holders": newHolderStr,
	}}
	filter := bson.M{"type": "currencyInfo"}
	_, err = patformInfoCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		fmt.Print(err)
	}
}

func isInternalAddress(address string, assetChoice string, accounts *mongo.Collection) (bool, string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var filter bson.M
	var user modals.User
	filter = bson.M{
		"EADD": bson.M{
			"$regex":   address,
			"$options": "i", // i => case-insensitive
		},
	}
	err := accounts.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, "Transfer to external address are blocked", ""
		}
	}
	switch assetChoice {
	case "USDT-PoS":
		return true, user.POS, user.ID
	case "USDT-ERC":
		return true, user.ERC, user.ID
	case "TBYT-PoS":
		return true, user.TBT, user.ID
	}
	return false, "Invalid Asset Choice", ""
}

func UpdateOrderList(fee string, senderID string, receiverID string, recipientAddress string, debitValue string, cType string, TYPE string, orderCollection *mongo.Collection) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	utcNow := time.Now().UTC()
	unixTimestamp := utcNow.Unix()

	tmpString := strconv.FormatInt(unixTimestamp, 10)
	orderData := bson.M{
		"SADD": senderID,
		"CADD": recipientAddress,
		"RADD": receiverID,
		"AMT":  debitValue,
		"CTP":  cType,
		"TYP":  "INT",
		"TMP":  tmpString,
		"STAT": "done",
		"FEE":  fee,
	}

	_, err := orderCollection.InsertOne(ctx, orderData)
	if err != nil {
		return false, "Can't Update Order List"
	}

	returnData := fmt.Sprintf("%d,%s,%s,%s", unixTimestamp, TYPE, debitValue, fee)
	return true, returnData
}
