package exchange

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"tbapi/modals"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func FindAndSettleOrders(r *http.Request, initiatorEID primitive.ObjectID, amount string, fromCurrency string, toCurrency string, initAddress string) (string, float64) {
	db, err := modals.ConnectDB()
	if err != nil {
		return "pending", 0
	}
	buyerAMT, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return "pending", 0
	}
	buyerAMT = RoundToNDecimals(buyerAMT, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exOrders := db.Collection("exchangeOrders")
	accounts := db.Collection("tb_accounts")
	sellerOrders := findSellers(buyerAMT, fromCurrency, toCurrency, db)
	if sellerOrders == nil {
		return "pending", 0
	}
	if len(sellerOrders) == 0 {
		return "pending", 0
	}
	var totalAmountSettled float64
	var buyerAMTtoSettle float64
	for _, order := range sellerOrders {
		buyerAMTtoSettle = buyerAMT - totalAmountSettled

		isSettled, amountSettled := settleExchange(db, order, buyerAMTtoSettle)
		if !isSettled {
			return "pending", 0
		}
		totalAmountSettled += amountSettled

		if totalAmountSettled >= buyerAMT {
			break
		}
	}
	var buyerStatus string
	var buyerNewSAMT float64
	if totalAmountSettled == buyerAMT {
		buyerStatus = "done"
		buyerNewSAMT = buyerAMT
	} else if len(sellerOrders) == 0 {
		buyerStatus = "pending"
		buyerNewSAMT = 0
	} else {
		buyerStatus = "partial"
		buyerNewSAMT = buyerAMT - (buyerAMT - totalAmountSettled)
	}
	// redeem to buyer
	if buyerStatus == "done" {
		accountData, _ := modals.GetAccountData(initAddress, accounts)

		previousPOS, _ := strconv.ParseFloat(accountData.POS, 64)
		previousTBT, _ := strconv.ParseFloat(accountData.TBT, 64)
		var newPOS float64
		var newTBT float64
		if fromCurrency == "TBT" {
			newPOS = previousPOS + buyerAMT
			newTBT = previousTBT
		} else {
			newTBT = previousTBT + buyerAMT
			newPOS = previousPOS
		}

		newPOSString := fmt.Sprintf("%f", newPOS)
		newTBTString := fmt.Sprintf("%f", newTBT)

		update := bson.M{"$set": bson.M{
			"POS": newPOSString,
			"TBT": newTBTString,
		}}
		filter := bson.M{"ID": initAddress}
		_, err = accounts.UpdateOne(ctx, filter, update)
		if err != nil {
			return "pending", 0
		}
	}
	buyerNewSAMTStr := fmt.Sprintf("%f", buyerNewSAMT)
	update := bson.M{"$set": bson.M{
		"STAT": buyerStatus,
		"SAMT": buyerNewSAMTStr,
	}}
	filter := bson.M{"_id": initiatorEID}
	_, err = exOrders.UpdateOne(ctx, filter, update)
	if err != nil {
		return "pending", 0
	}
	return buyerStatus, totalAmountSettled
}

func findSellers(
	buyerAMT float64,
	fromCurrency string,
	toCurrency string,
	db *mongo.Database,
) []ExOrder {
	// Database collections
	exOrders := db.Collection("exchangeOrders")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	filter := bson.M{
		"$and": []bson.M{
			{"FROM": toCurrency},
			{"TO": fromCurrency},
			{
				"STAT": bson.M{
					"$in": []string{"pending", "partial"},
				},
			},
		},
	}

	opts := options.Find().SetSort(bson.M{"TMP": 1})

	cursor, err := exOrders.Find(ctx, filter, opts)
	if err != nil {
		return nil
	}

	var orders []ExOrder
	err = cursor.All(ctx, &orders)
	if err != nil {
		return nil
	}

	sellerOrdersAMT := 0.00
	var filteredOrders []ExOrder

	for i := range orders {
		orderAMT, err := strconv.ParseFloat(orders[i].SAMT, 64)
		if err != nil {
			return nil
		}
		sellerOrdersAMT += orderAMT
		filteredOrders = append(filteredOrders, orders[i])
		if sellerOrdersAMT >= buyerAMT {
			break
		}
	}

	return filteredOrders

}

func settleExchange(
	db *mongo.Database,
	sellerOrder ExOrder,
	buyerAMTtoSettle float64,
) (bool, float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exOrders := db.Collection("exchangeOrders")
	accounts := db.Collection("tb_accounts")
	sellerEID := sellerOrder.EID
	sellerAddress := sellerOrder.ID
	sellerSAMT, err := strconv.ParseFloat(sellerOrder.SAMT, 64)
	if err != nil {
		return false, 0
	}
	sellerAMT, err := strconv.ParseFloat(sellerOrder.AMT, 64)
	if err != nil {
		return false, 0
	}
	sellerPendingAMT := sellerAMT - sellerSAMT
	var sellerNewSAMT float64
	var amountSettled float64
	var sellerStatus string
	if buyerAMTtoSettle == sellerPendingAMT {
		sellerNewSAMT = sellerSAMT + sellerPendingAMT
		sellerStatus = "done"
		amountSettled = sellerPendingAMT
	} else if buyerAMTtoSettle > sellerPendingAMT {
		sellerNewSAMT = sellerSAMT + sellerPendingAMT
		sellerStatus = "done"
		amountSettled = sellerPendingAMT
	} else {
		sellerNewSAMT = sellerSAMT + buyerAMTtoSettle
		sellerStatus = "partial"
		amountSettled = buyerAMTtoSettle
	}
	// redeem to wallet
	if sellerStatus == "done" {
		accountData, _ := modals.GetAccountData(sellerAddress, accounts)
		previousPOS, _ := strconv.ParseFloat(accountData.POS, 64)
		previousTBT, _ := strconv.ParseFloat(accountData.TBT, 64)
		previousERC, _ := strconv.ParseFloat(accountData.ERC, 64)
		var newPOS float64
		var newTBT float64
		var newERC float64

		if sellerOrder.FROM == "TBYT" {
			newTBT = previousTBT
			if sellerOrder.TO == "USDT-POS" {
				newPOS = previousPOS + sellerAMT
				newERC = previousERC
			} else if sellerOrder.TO == "USDT-ERC" {
				newERC = previousERC + sellerAMT
				newPOS = previousPOS
			} else {
				newPOS = previousPOS
				newERC = previousERC
			}
		} else if sellerOrder.FROM == "USDT-POS" {
			newPOS = previousPOS
			if sellerOrder.TO == "TBYT" {
				newTBT = previousTBT + sellerAMT
				newERC = previousERC
			} else if sellerOrder.TO == "USDT-ERC" {
				newERC = previousERC + sellerAMT
				newPOS = previousPOS
			} else {
				newTBT = previousTBT
				newERC = previousERC
			}
		} else if sellerOrder.FROM == "USDT-ERC" {
			newERC = previousERC
			if sellerOrder.TO == "TBYT" {
				newPOS = previousPOS
				newTBT = previousTBT + sellerAMT
			} else if sellerOrder.TO == "USDT-POS" {
				newTBT = previousTBT
				newPOS = previousPOS + sellerAMT
			} else {
				newTBT = previousTBT
				newPOS = previousPOS
			}
		} else {
			return false, 0
		}

		newPOSString := fmt.Sprintf("%f", newPOS)
		newTBTString := fmt.Sprintf("%f", newTBT)
		newERCString := fmt.Sprintf("%f", newERC)

		update := bson.M{"$set": bson.M{
			"POS": newPOSString,
			"TBT": newTBTString,
			"ERC": newERCString,
		}}
		filter := bson.M{"ID": sellerAddress}
		accounts.UpdateOne(ctx, filter, update)
	}
	// redeem to seller wallet
	sellerNewSAMTStr := fmt.Sprintf("%f", sellerNewSAMT)
	update := bson.M{"$set": bson.M{
		"STAT": sellerStatus,
		"SAMT": sellerNewSAMTStr,
	}}
	filter := bson.M{"_id": sellerEID}
	_, err = exOrders.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, 0
	}

	return true, amountSettled
}

func RoundToNDecimals(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}
