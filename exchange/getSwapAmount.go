package exchange

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

func GetSwapAmounts(r *http.Request) (string, string) {
	db, err := modals.ConnectDB()

	if err != nil {
		return "false", "API Database Error"
	}
	// Database collections
	exchangeCollection := db.Collection("exchangeOrders")

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
	validKey := modals.CheckKey(walletKey, address)
	if !validKey {
		return "false", "Invalid Account Key"
	}
	orderAmountData := getExchangeAmount(exchangeCollection)
	if orderAmountData == "nil" {
		return "true", "0.00,0.00,0.00,0.00,0.00,0.00"
	}
	// csvData := exOrderToCSV(accountData)
	return "true", orderAmountData
}

func getExchangeAmount(exchangeOrders *mongo.Collection) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	filter := bson.M{
		"STAT": bson.M{
			"$in": []string{"pending", "partial"},
		},
	}
	cursor, err := exchangeOrders.Find(ctx, filter)
	if err != nil {
		return "nil"
	}
	var orders []ExOrder
	err = cursor.All(ctx, &orders)
	if err != nil {
		return "nil"
	}

	TBYTtoPOS := 0.00
	POStoTBYT := 0.00
	ERCtoPOS := 0.00
	POStoERC := 0.00
	ERCtoTBYT := 0.00
	TBYTtoERC := 0.00

	for i := range orders {
		orderAMT, err := strconv.ParseFloat(orders[i].AMT, 64)
		if err != nil {
			break
		}
		orderSAMT, err := strconv.ParseFloat(orders[i].SAMT, 64)
		if err != nil {
			break
		}
		pendingAMT := orderAMT - orderSAMT

		if orders[i].FROM == "TBYT" && orders[i].TO == "USDT-POS" {
			TBYTtoPOS = TBYTtoPOS + pendingAMT
		} else if orders[i].FROM == "USDT-POS" && orders[i].TO == "TBYT" {
			POStoTBYT = POStoTBYT + pendingAMT
		} else if orders[i].FROM == "TBYT" && orders[i].TO == "USDT-ERC" {
			TBYTtoERC = TBYTtoERC + pendingAMT
		} else if orders[i].FROM == "USDT-ERC" && orders[i].TO == "TBYT" {
			ERCtoTBYT = ERCtoTBYT + pendingAMT
		} else if orders[i].FROM == "USDT-ERC" && orders[i].TO == "USDT-POS" {
			ERCtoPOS = ERCtoPOS + pendingAMT
		} else if orders[i].FROM == "USDT-POS" && orders[i].TO == "USDT-ERC" {
			POStoERC = POStoERC + pendingAMT
		}
	}

	return fmt.Sprintf("%.2f,%.2f,%.2f,%.2f,%.2f,%.2f",
		TBYTtoPOS, TBYTtoERC, POStoERC, POStoTBYT, ERCtoPOS, ERCtoTBYT)
}
