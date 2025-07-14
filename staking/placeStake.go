package staking

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

type Stake struct {
	EID  primitive.ObjectID `bson:"_id,omitempty"`
	ADD  string             `bson:"ADD"`
	AMT  string             `bson:"AMT"`
	STMP string             `bson:"STMP"`
	MTMP string             `bson:"MTMP"`
	OPT  string             `bson:"OPT"`
	STAT string             `bson:"STAT"`
	STKP string             `bson:"STKP"`
}

func PlaceStake(r *http.Request) (string, string) {
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
	if len(walletsDetailList) != 4 {
		return "false", "Request Malformed"
	}
	address := walletsDetailList[0]
	walletKey := walletsDetailList[1]
	stakeAmount := walletsDetailList[2]
	stakeOption := walletsDetailList[3]
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
	accountData, isAccountFound := modals.GetAccountData(address, accounts)
	if !isAccountFound {
		return "false", "Problem with account"
	}

	stakeAmountFloat, err := strconv.ParseFloat(stakeAmount, 64)
	if err != nil {
		return "false", "Can't Convert Stake Amount to Integer "
	}

	tbtBalance, err := strconv.ParseFloat(accountData.TBT, 64)
	if err != nil {
		return "false", "Can't Convert Tulobyte Balance to Integer "
	}

	if tbtBalance < stakeAmountFloat {
		return "false", "Insufficient Tulobyte Balance"
	}
	stakeDuration, err := strconv.ParseFloat(stakeOption, 64)
	if err != nil {
		return "false", "Problem With Stake Option"
	}
	stakesCollection := db.Collection("stakesCollection")
	stakerReferrals := accountData.REFS
	bonusStakes := 0

	for i := 0; i < len(stakerReferrals); i++ {
		isStaked := CheckStake(stakesCollection, stakerReferrals[i])
		if isStaked {
			bonusStakes += 1
		}
	}
	totalStakers := float64(bonusStakes)

	var durationProfit = 0.00
	if stakeDuration == 7 {
		durationProfit = 5.6
	} else if stakeDuration == 14 {
		durationProfit = 14
	} else if stakeDuration == 21 {
		durationProfit = 25.2
	} else if stakeDuration == 29 {
		durationProfit = 43.5
	} else {
		return "false", "Update App to Stake"
	}

	var stakesProfitPercent = 0.00
	referralStakePercent := totalStakers * 0.5
	if totalStakers > 0 {
		stakesProfitPercent = referralStakePercent + durationProfit
	} else {
		stakesProfitPercent = durationProfit
	}
	var stakeProfit = (stakesProfitPercent * stakeAmountFloat) / 100

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	utcNow := time.Now().UTC()
	unixTimestamp := utcNow.Unix()
	timeString := fmt.Sprintf("%d", unixTimestamp)

	futureTime := utcNow.Add(time.Duration(stakeDuration) * 24 * time.Hour) // 10 days added
	futureTMP := futureTime.Unix()
	futureTMPString := fmt.Sprintf("%d", futureTMP)

	stakeProfitString := fmt.Sprintf("%f", stakeProfit)
	stakeData := bson.M{
		"ADD":  address,
		"AMT":  stakeAmount,
		"STKP": stakeProfitString,
		"STMP": timeString,
		"MTMP": futureTMPString,
		"OPT":  stakeOption,
		"STAT": "active",
	}

	amountOnMaturity := stakeAmountFloat + stakeProfit

	result, err := stakesCollection.InsertOne(ctx, stakeData)
	if err != nil {
		return "false", "Can't Place Stake"
	}

	isTBTBalUpdated := UpdateTBTBal(accounts, tbtBalance, stakeAmountFloat, address)
	if !isTBTBalUpdated {
		filter := bson.M{"_id": result.InsertedID.(primitive.ObjectID)}
		stakesCollection.DeleteOne(ctx, filter)
		return "true", "Can not fetch"
	}
	returnData := fmt.Sprintf("%s,%s,%f,%s,%f", result.InsertedID.(primitive.ObjectID).Hex(), stakeAmount, amountOnMaturity, futureTMPString, referralStakePercent)
	return "true", returnData
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

func UpdateTBTBal(account *mongo.Collection, tbtBalance float64, stakeAmountFloat float64, address string) bool {
	newTBTBalance := tbtBalance - stakeAmountFloat
	newTBTBalanceString := fmt.Sprintf("%f", newTBTBalance)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// update stake collection
	update := bson.M{"$set": bson.M{
		"TBT": newTBTBalanceString,
	}}
	filter := bson.M{"ID": address}
	_, err := account.UpdateOne(ctx, filter, update)
	return err == nil
}
