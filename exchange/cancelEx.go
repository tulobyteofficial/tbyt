package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"tbapi/modals"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func CencelExchange(r *http.Request) (string, string, string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "false", "API Database Error", ""
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return "false", "Request Empty", ""
	}
	// Convert JSON body to map[string]string
	var data map[string]string
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "false", "API Database Error", ""
	}

	walletsDetailString, exists := data["data"]
	if !exists {
		return "false", "Request Malformed No Data", ""
	}
	walletsDetailList := strings.Split(walletsDetailString, ",")
	if len(walletsDetailList) != 3 {
		return "false", "Request Malformed", ""
	}
	address := walletsDetailList[0]
	walletKey := walletsDetailList[1]
	orderID := walletsDetailList[2]

	validKey := modals.CheckKey(walletKey, address)
	if !validKey {
		return "false", "Trying to bypass", ""
	}
	db, err := modals.ConnectDB()

	if err != nil {
		return "false", "API Database Error", ""
	}
	isSettled, message, purpose := settleOrder(db, orderID, address)
	if !isSettled {
		return "false", message, ""
	}
	return "true", message, purpose
}

func settleOrder(db *mongo.Database, orderID string, address string) (bool, string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exchangeCollection := db.Collection("exchangeOrders")
	accountCollection := db.Collection("tb_accounts")
	// for accounts
	accountData, isFound := modals.GetAccountData(address, accountCollection)
	if !isFound {
		return false, "Can't fetch account details", ""
	}

	// for exchnage orders
	objectID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		log.Fatalf("Invalid ObjectID hex: %v", err)
	}
	exOrderData, isFound := getExOrderData(objectID, exchangeCollection)
	if !isFound {
		return false, "Can't fetch Swap details", ""
	}

	orderSAMT, err := strconv.ParseFloat(exOrderData.SAMT, 64)
	if err != nil {
		return false, "Settled Amount Conversion Error", ""
	}
	orderAMT, err := strconv.ParseFloat(exOrderData.AMT, 64)
	if err != nil {
		return false, "Exchange Amount Conversion Error", ""
	}
	pendingExchange := orderAMT - orderSAMT
	fromCurrency := exOrderData.FROM
	toCurrency := exOrderData.TO

	tbtBalance, err := strconv.ParseFloat(accountData.TBT, 64)
	if err != nil {
		return false, "TBT Balance Conversion Error", ""
	}
	posBalance, err := strconv.ParseFloat(accountData.POS, 64)
	if err != nil {
		return false, "POS Balance Conversion Error", ""
	}
	ercBalance, err := strconv.ParseFloat(accountData.ERC, 64)
	if err != nil {
		return false, "POS Balance Conversion Error", ""
	}

	newTBTBal := tbtBalance
	newPOSBal := posBalance
	newERCBal := ercBalance
	newAMT := orderAMT

	if fromCurrency == "USDT-POS" && toCurrency == "TBYT" {
		newPOSBal = posBalance + orderSAMT
		newTBTBal = tbtBalance + pendingExchange
		newAMT = orderSAMT
	} else if fromCurrency == "USDT-POS" && toCurrency == "USDT-ERC" {
		newPOSBal = posBalance + orderSAMT
		newERCBal = ercBalance + pendingExchange
		newAMT = orderSAMT
	} else if fromCurrency == "USDT-ERC" && toCurrency == "TBYT" {
		newERCBal = ercBalance + orderSAMT
		newTBTBal = tbtBalance + pendingExchange
		newAMT = orderSAMT
	} else if fromCurrency == "USDT-ERC" && toCurrency == "USDT-POS" {
		newERCBal = ercBalance + orderSAMT
		newPOSBal = posBalance + pendingExchange
		newAMT = orderSAMT
	} else if fromCurrency == "TBYT" && toCurrency == "USDT-POS" {
		newPOSBal = posBalance + orderSAMT
		newTBTBal = tbtBalance + pendingExchange
		newAMT = orderSAMT
	} else if fromCurrency == "TBYT" && toCurrency == "USDT-ERC" {
		newERCBal = ercBalance + orderSAMT
		newTBTBal = tbtBalance + pendingExchange
		newAMT = orderSAMT
	}

	newAMTString := fmt.Sprintf("%f", newAMT)
	newERCString := fmt.Sprintf("%f", newERCBal)
	newPOSBalString := fmt.Sprintf("%f", newPOSBal)
	newTBTBalString := fmt.Sprintf("%f", newTBTBal)

	updateAccount := bson.M{"$set": bson.M{
		"TBT": newTBTBalString,
		"POS": newPOSBalString,
		"ERC": newERCString,
	}}
	filter := bson.M{"ID": address}
	_, err = accountCollection.UpdateOne(ctx, filter, updateAccount)
	if err != nil {
		return false, "Could not update balance info", ""
	}
	purpose := ""
	if orderSAMT == 0.00 {
		filterEx := bson.M{"_id": objectID}
		_, err = exchangeCollection.DeleteOne(ctx, filterEx)
		if err != nil {
			return false, "Could not delete order", ""
		}
		purpose = "deleted"
	} else {
		updateExOrder := bson.M{"$set": bson.M{
			"STAT": "done",
			"AMT":  newAMTString,
		}}
		filterEx := bson.M{"_id": objectID}
		_, err = exchangeCollection.UpdateOne(ctx, filterEx, updateExOrder)
		if err != nil {
			return false, "Could not update swap info", ""
		}
		purpose = "settled"
	}

	return true, "Successfully Settled", purpose

}

func getExOrderData(orderID primitive.ObjectID, collection *mongo.Collection) (ExOrder, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	filter := bson.M{"_id": orderID}
	var user ExOrder
	err := collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return user, false
		}
	}
	return user, true
}
