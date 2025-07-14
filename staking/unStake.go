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

func Unstake(r *http.Request) (string, string) {
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
	stakeID := walletsDetailList[2]
	validKey := modals.CheckKey(walletKey, address)
	if !validKey {
		return "false", "Trying to bypass"
	}
	db, err := modals.ConnectDB()
	stakesCollection := db.Collection("stakesCollection")
	if err != nil {
		return "false", "API Database Error"
	}
	isStakeFound, stakeData := GetStakeByID(stakesCollection, stakeID)
	if !isStakeFound {
		return "false", "Problem in fecthing stake data"
	}
	if stakeData.STAT == "completed" {
		return "false", "Stake is already completed"
	}
	stakeMatureTime, err := strconv.ParseInt(stakeData.MTMP, 10, 64)
	if err != nil {
		return "false", "Problem at Backend UNSTK63 "
	}

	stakeAmount, err := strconv.ParseFloat(stakeData.AMT, 64)
	if err != nil {
		return "false", "Problem at Backend UNSTK63 "
	}

	stakeProfit, err := strconv.ParseFloat(stakeData.STKP, 64)
	if err != nil {
		return "false", "Problem at Backend UNSTK63 "
	}
	utcNow := time.Now().UTC()
	unixTimestamp := utcNow.Unix()
	isStakeMatures := unixTimestamp >= stakeMatureTime
	if !isStakeMatures {
		return "false", "Stake not matured yet"
	}

	isUnstaked, message := UnstakeAmount(db, stakeID, stakeData.ADD, stakeAmount, stakeProfit)
	if !isUnstaked {
		return "false", message
	}

	return "true", message
}

func GetStakeByID(stakesCollection *mongo.Collection, stakeID string) (bool, Stake) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stake Stake
	stakeIDObj, err := primitive.ObjectIDFromHex(stakeID)
	if err != nil {
		return false, stake
	}
	filter := bson.M{"_id": stakeIDObj}
	err = stakesCollection.FindOne(ctx, filter).Decode(&stake)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, stake
		}
	}
	return true, stake
}

func UnstakeAmount(db *mongo.Database, stakeID string, stakerID string, stakeAmount float64, stakeProfit float64) (bool, string) {
	stakesCollection := db.Collection("stakesCollection")
	accounts := db.Collection("tb_accounts")
	patformInfoCollection := db.Collection("platformInfo")
	accountData, _ := modals.GetAccountData(stakerID, accounts)
	tbtBalance, err := strconv.ParseFloat(accountData.TBT, 64)
	if err != nil {
		return false, "Backend Error UNSTK117"
	}

	totalProfit, err := strconv.ParseFloat(accountData.NPT, 64)
	if err != nil {
		return false, "Backend Error UNSTK122"
	}
	totalProfitPercentage, err := strconv.ParseFloat(accountData.NPTP, 64)
	if err != nil {
		return false, "Backend Error UNSTK126"
	}

	stakeIDObj, err := primitive.ObjectIDFromHex(stakeID)
	if err != nil {
		return false, "Backend Error UNSTK131"
	}
	profitPercentage := stakeProfit / stakeAmount * 100
	newProfitPercent := fmt.Sprintf("%f", totalProfitPercentage+profitPercentage)

	newProfit := fmt.Sprintf("%f", stakeProfit+totalProfit)

	stakeAmountWithProfit := stakeAmount + stakeProfit
	newTBTBalance := stakeAmountWithProfit + tbtBalance
	newTBTBalanceString := fmt.Sprintf("%f", newTBTBalance)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// update stake collection
	update := bson.M{"$set": bson.M{
		"STAT": "completed",
	}}
	filter := bson.M{"_id": stakeIDObj}
	_, err = stakesCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, "Unstake failed Try again"
	}

	// update accounts Data
	update = bson.M{"$set": bson.M{
		"TBT":  newTBTBalanceString,
		"NPT":  newProfit,
		"NPTP": newProfitPercent,
	}}
	filter = bson.M{"ID": stakerID}
	_, err = accounts.UpdateOne(ctx, filter, update)
	if err != nil {
		update := bson.M{"$set": bson.M{
			"STAT": "active",
		}}
		filter := bson.M{"_id": stakeIDObj}
		stakesCollection.UpdateOne(ctx, filter, update)
		return false, "Unstake failed Try again"
	}

	// update supply
	platformInfo, isFound := modals.GetPlatformInfo()
	if !isFound {
		return true, "Unstake failed Try again"
	}
	minedTBT, err := strconv.ParseFloat(platformInfo.Mined, 64)
	if err != nil {
		return true, "Unstake failed Try again"
	}
	newMined := stakeProfit + minedTBT
	newMinedStr := fmt.Sprintf("%f", newMined)
	update = bson.M{"$set": bson.M{
		"mined": newMinedStr,
	}}
	filter = bson.M{"type": "currencyInfo"}
	patformInfoCollection.UpdateOne(ctx, filter, update)
	return true, "Unstaked Successfully"
}
