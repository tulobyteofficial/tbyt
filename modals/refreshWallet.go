package modals

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Stake struct {
	EID  primitive.ObjectID `bson:"_id,omitempty"`
	ADD  string             `bson:"ADD"`
	AMT  string             `bson:"AMT"`
	STMP string             `bson:"STMP"`
	MTMP string             `bson:"MTMP"`
	OPT  string             `bson:"OPT"`
	STAT string             `bson:"STAT"`
}
type User struct {
	ID      string   `bson:"ID"`      // Unique ID
	TBT     string   `bson:"TBT"`     // Tulobyte Balance
	POS     string   `bson:"POS"`     // Polygon USDT Bal
	ERC     string   `bson:"ERC"`     // ERC Usdt Bal
	NPT     string   `bson:"NPT"`     // Net Profit
	NPTP    string   `bson:"NPTP"`    // Net Profit Percentage
	EADD    string   `bson:"EADD"`    // Ethereium Address
	REFB    string   `bson:"REF"`     // Referred By Address
	REFS    []string `bson:"REFS"`    // Refererals
	EKEY    string   `bson:"EKEY"`    // EVM private key
	REFRESH string   `bson:"REFRESH"` // EVM private key
}

func RefreshAccount(r *http.Request) (string, string) {
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
		return "false", "Trying to bypass"
	}

	accountData, isFound := GetAccountData(address, accounts)
	if !isFound {
		return "false", "No Account Found"
	}

	oldPosBalance, err := strconv.ParseFloat(accountData.POS, 64)
	if err != nil {
		return "false", "Problem at backend"
	}

	oldERCBalance, err := strconv.ParseFloat(accountData.ERC, 64)
	if err != nil {
		return "false", "Problem at backend"
	}

	evmAddress := accountData.EADD
	// fetching balance from chain POS
	newPosBalance := oldPosBalance
	newERCBalance := oldERCBalance

	stakeCollection := db.Collection("stakesCollection")
	var REFStatus = []int{}
	for i := range accountData.REFS {
		refID := accountData.REFS[i]
		isActive := CheckStake(stakeCollection, refID)
		if isActive {
			REFStatus = append(REFStatus, 1)
		} else {
			REFStatus = append(REFStatus, 0)
		}
	}

	csvData := userToCSV(accountData, REFStatus, evmAddress, newERCBalance, newPosBalance)
	return "true", csvData

}

func UpdateBalance(assetChoice string, newBalance float64, accounts *mongo.Collection, address string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	newBalanceString := fmt.Sprintf("%f", newBalance)
	update := bson.M{"$set": bson.M{
		assetChoice: newBalanceString,
	}}
	filter := bson.M{"ID": address}
	_, err := accounts.UpdateOne(ctx, filter, update)
	return err == nil
}

func userToCSV(accountData User, REFStatus []int, newAddress string, erc float64, pos float64) string {
	ercString := fmt.Sprintf("%f", erc)
	posString := fmt.Sprintf("%f", pos)
	return fmt.Sprintf("%s,%s,%s,%s,%s,%d,%s,%s", accountData.TBT, posString, ercString, accountData.NPT, accountData.REFS, REFStatus, accountData.NPTP, newAddress)
}
func GetAccountData(walletAddress string, accounts *mongo.Collection) (User, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	filter := bson.M{"ID": walletAddress}
	var user User
	err := accounts.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return user, false
		}
	}
	return user, true
}

func CheckStake(stakesCollection *mongo.Collection, stakerID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	filter := bson.M{
		"ADD":  stakerID,
		"STAT": "active",
	}
	var stake Stake
	err := stakesCollection.FindOne(ctx, filter).Decode(&stake)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false
		}
	}
	return true
}
