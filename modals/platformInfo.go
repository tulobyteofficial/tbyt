package modals

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Fees struct {
	Polygon string `bson:"polygon"`
	Eth     string `bson:"eth"`
	Gwei    string `bson:"gwei"`
}

func PlatformInfo(r *http.Request) (string, string) {

	_, err := io.ReadAll(r.Body)
	if err != nil {
		return "false", "API Database Error"
	}
	Cost, isGot := getFees()
	if !isGot {
		return "false", "Cannot Get Fees Data"
	}
	gweiCostFloat, err := strconv.ParseFloat(Cost.Gwei, 64)
	if err != nil {
		return "false", "Backend Error"
	}
	ethFloat, err := strconv.ParseFloat(Cost.Eth, 64)
	if err != nil {
		return "false", "Backend Error"
	}
	polFloat, err := strconv.ParseFloat(Cost.Polygon, 64)
	if err != nil {
		return "false", "Backend Error"
	}
	evmFees := 21000.0 * gweiCostFloat
	ethFeesUSD := evmFees * ethFloat

	polFees := evmFees * polFloat
	Info, isGot := GetPlatformInfo()
	if !isGot {
		return "false", "Cannot Get Info Data"
	}

	returnString := fmt.Sprintf("%f,%f,%s,%s,%s,%s", polFees, ethFeesUSD, Info.TotalSupply, Info.MaxSupply, Info.Mined, Info.Holder)
	return "true", returnString
}

func getFees() (Fees, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := ConnectDB()

	if err != nil {
		return Fees{}, false
	}
	// Database collections
	fees := db.Collection("platformInfo")

	filter := bson.M{"type": "updatedFee"}
	var result Fees
	err = fees.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, false
		}
	}

	return result, true

}

type Currency struct {
	TotalSupply string `bson:"totalSupply"`
	MaxSupply   string `bson:"maxSupply"`
	Mined       string `bson:"mined"`
	Holder      string `bson:"holders"`
}

func GetPlatformInfo() (Currency, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := ConnectDB()

	if err != nil {
		return Currency{}, false
	}
	// Database collections
	fees := db.Collection("platformInfo")

	filter := bson.M{"type": "currencyInfo"}
	var result Currency
	err = fees.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, false
		}
	}
	return result, true

}

type AppInfo struct {
	VERSION string `bson:"VERSION"`
}

func GetVersion() (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := ConnectDB()

	if err != nil {
		return "false", "Error Connecting to DB"
	}
	// Database collections
	fees := db.Collection("platformInfo")

	filter := bson.M{"type": "APPINFO"}
	var result AppInfo
	err = fees.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "false", "Database Error"
		}
	}
	var numbers []string
	for _, ch := range result.VERSION {
		numbers = append(numbers, string(ch))
	}

	return "true", result.VERSION

}
