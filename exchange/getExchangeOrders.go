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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetExOrderData(r *http.Request) (string, string) {
	db, err := modals.ConnectDB()

	if err != nil {
		return "false", "API Database Error"
	}
	// Database collections
	orderCollection := db.Collection("exchangeOrders")

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
	if len(walletsDetailList) != 3 {
		return "false", "Request Malformed"
	}
	address := walletsDetailList[0]
	walletKey := walletsDetailList[1]
	orderNeeded := walletsDetailList[2]
	validKey := modals.CheckKey(walletKey, address)
	if !validKey {
		return "false", "Trying to bypass"
	}

	orderData, isFound := getExLastTenOrders(address, orderCollection, orderNeeded)
	if !isFound {
		return "false", "No Account Found"
	}

	csvData := exOrderToCSV(orderData)
	return "true", csvData

}

func getExLastTenOrders(walletAddress string, orderCollection *mongo.Collection, orderNeeded string) ([]ExOrder, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Convert orderNeeded to page number
	pageNum, err := strconv.Atoi(orderNeeded)
	if err != nil || pageNum <= 0 {
		log.Println("Invalid page number:", orderNeeded)
		return nil, false
	}

	pageSize := int64(10)
	skip := int64((pageNum - 1)) * pageSize

	filter := bson.M{"ID": walletAddress}
	opts := options.Find().
		SetSort(bson.M{"TMP": -1}). // Newest first
		SetSkip(skip).
		SetLimit(pageSize)

	cursor, err := orderCollection.Find(ctx, filter, opts)
	if err != nil {
		log.Println("Error finding orders:", err)
		return nil, false
	}

	var orders []ExOrder
	if err = cursor.All(ctx, &orders); err != nil {
		log.Println("Error decoding orders:", err)
		return nil, true
	}

	if len(orders) == 0 {
		return nil, true
	}

	return orders, true
}
func exOrderToCSV(orders []ExOrder) string {
	var builder strings.Builder
	for i, order := range orders {
		builder.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s",
			order.EID.Hex(), order.FROM, order.TO, order.AMT, order.SAMT, order.AMT, order.TMP, order.STAT))

		if i < len(orders)-1 {
			builder.WriteString("#") // Separate orders with slash, but not after the last one
		}
	}
	return builder.String()
}
