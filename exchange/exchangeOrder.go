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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ExOrder struct {
	EID  primitive.ObjectID `bson:"_id,omitempty"`
	ID   string             `bson:"ID"`
	FROM string             `bson:"FROM"`
	TO   string             `bson:"TO"`
	AMT  string             `bson:"AMT"`
	SAMT string             `bson:"SAMT"`
	TMP  string             `bson:"TMP"`
	STAT string             `bson:"STAT"`
}

func PlaceExchangeOrder(r *http.Request) (string, string, string, primitive.ObjectID, string, string, string, string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "false", "API Database Error", "", primitive.NilObjectID, "", "", "", ""
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return "false", "Request Empty", "", primitive.NilObjectID, "", "", "", ""
	}
	// Convert JSON body to map[string]string
	var data map[string]string
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "false", "API Database Error", "", primitive.NilObjectID, "", "", "", ""
	}

	walletsDetailString, exists := data["data"]
	if !exists {
		return "false", "Request Malformed No Data", "", primitive.NilObjectID, "", "", "", ""
	}
	walletsDetailList := strings.Split(walletsDetailString, ",")
	if len(walletsDetailList) != 5 {
		return "false", "Request Malformed", "", primitive.NilObjectID, "", "", "", ""
	}

	db, err := modals.ConnectDB()

	if err != nil {
		return "false", "API Database Error", "", primitive.NilObjectID, "", "", "", ""
	}
	// Database collections
	accounts := db.Collection("tb_accounts")

	address := walletsDetailList[0]
	walletKey := walletsDetailList[1]
	fromCurrency := walletsDetailList[2]
	fromAmount := walletsDetailList[3]
	toCurrency := walletsDetailList[4]
	validKey := modals.CheckKey(walletKey, address)
	if !validKey {
		return "false", "Trying to bypass", "", primitive.NilObjectID, "", "", "", ""
	}

	accountData, isFound := modals.GetAccountData(address, accounts)
	if !isFound {
		return "false", "No Account Found", "", primitive.NilObjectID, "", "", "", ""
	}

	isCreated, orderStatus, exStatus, EID := createExOrder(accountData, fromCurrency, toCurrency, fromAmount, db)

	return isCreated, orderStatus, exStatus, EID, fromAmount, fromCurrency, toCurrency, address
}

func createExOrder(accountData modals.User, fromCurrency string, toCurrency string, fromAmount string, db *mongo.Database) (string, string, string, primitive.ObjectID) {
	tbtBalance, err := strconv.ParseFloat(accountData.TBT, 64)
	if err != nil {
		return "false", "Can't convert Recipient Balance to Integer ", "", primitive.NilObjectID
	}

	posBalance, err := strconv.ParseFloat(accountData.POS, 64)
	if err != nil {
		return "false", "Can't convert Recipient Balance to Integer ", "", primitive.NilObjectID
	}

	ercBalance, err := strconv.ParseFloat(accountData.ERC, 64)
	if err != nil {
		return "false", "Can't convert Recipient Balance to Integer ", "", primitive.NilObjectID
	}

	fromAmountFloat, err := strconv.ParseFloat(fromAmount, 64)
	if err != nil {
		return "false", "Can't convert Recipient Balance to Integer ", "", primitive.NilObjectID
	}

	fromAmountFloat = RoundToNDecimals(fromAmountFloat, 4)
	isOrderCreated := false
	errorMessage := ""
	EID := primitive.NilObjectID
	if fromCurrency == "USDT-ERC" {
		if ercBalance < fromAmountFloat {
			return "false", "Insufficient USDT-ERC Amount ", "", primitive.NilObjectID
		}
		isOrderCreated, errorMessage, EID = makeOrder(accountData, fromAmount, fromCurrency, toCurrency, db, tbtBalance, fromAmountFloat, posBalance, ercBalance)
	} else if fromCurrency == "USDT-POS" {
		if posBalance < fromAmountFloat {
			return "false", "Insufficient USDT-POS Amount ", "", primitive.NilObjectID
		}
		isOrderCreated, errorMessage, EID = makeOrder(accountData, fromAmount, fromCurrency, toCurrency, db, tbtBalance, fromAmountFloat, posBalance, ercBalance)
	} else if fromCurrency == "TBYT" {
		if tbtBalance < fromAmountFloat {
			return "false", "Insufficient TBYT Amount ", "", primitive.NilObjectID
		}
		isOrderCreated, errorMessage, EID = makeOrder(accountData, fromAmount, fromCurrency, toCurrency, db, tbtBalance, fromAmountFloat, posBalance, ercBalance)
	} else {
		return "false", "Invalid Asset Conversion ", "", primitive.NilObjectID
	}

	if !isOrderCreated {
		return "false", errorMessage, "", primitive.NilObjectID
	}
	return "true", "Placed Successfully", "pending", EID

}

func makeOrder(
	accountData modals.User,
	fromAmount string,
	fromCurrency string,
	toCurrency string,
	db *mongo.Database,
	tbtBalance float64,
	fromAmountFloat float64,
	posBalance float64,
	ercBalance float64,
) (bool, string, primitive.ObjectID) {
	// Database collections
	exOrders := db.Collection("exchangeOrders")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Calculating deductions
	posNewBalance := posBalance
	ercNewBalance := ercBalance
	tbtNewBalance := tbtBalance
	if fromCurrency == "USDT-POS" {
		posNewBalance = posBalance - fromAmountFloat
	} else if fromCurrency == "USDT-ERC" {
		ercNewBalance = ercBalance - fromAmountFloat
	} else if fromCurrency == "TBYT" {
		tbtNewBalance = tbtBalance - fromAmountFloat
	} else {
		return false, "Can't Update Order List", primitive.NilObjectID
	}

	utcNow := time.Now().UTC()
	unixTimestamp := utcNow.Unix()
	timeString := fmt.Sprintf("%d", unixTimestamp)
	orderData := bson.M{
		"ID":   accountData.ID,
		"FROM": fromCurrency,
		"TO":   toCurrency,
		"SAMT": "0.00",
		"AMT":  fromAmount,
		"TMP":  timeString,
		"STAT": "pending",
	}

	result, err := exOrders.InsertOne(ctx, orderData)
	if err != nil {
		return false, "Can't Update Order List", primitive.NilObjectID
	}

	accountsColl := db.Collection("tb_accounts")

	// Deduct Balance from account

	tbtNewBalString := fmt.Sprintf("%f", tbtNewBalance)
	posNewBalString := fmt.Sprintf("%f", posNewBalance)
	ercNewBalString := fmt.Sprintf("%f", ercNewBalance)

	update2 := bson.M{"$set": bson.M{
		"TBT": tbtNewBalString,
		"POS": posNewBalString,
		"ERC": ercNewBalString,
	}}
	filter2 := bson.M{"ID": accountData.ID}
	_, err = accountsColl.UpdateOne(ctx, filter2, update2)
	if err != nil {
		// revert if not added
		_, err := exOrders.DeleteOne(context.TODO(), bson.M{"_id": result.InsertedID.(primitive.ObjectID)})
		if err != nil {
			return false, "Can not delete redundent order", primitive.NilObjectID
		}
	}

	return true, "Exchange Order Created", result.InsertedID.(primitive.ObjectID)
}
