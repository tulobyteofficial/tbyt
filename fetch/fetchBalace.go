package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"tbapi/modals"
	"tbapi/transfer"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func FetchChainBalance(r *http.Request) (string, string) {
	db, err := modals.ConnectDB()

	if err != nil {
		return "false", "API Database Error"
	}
	// Database collections
	accounts := db.Collection("tb_accounts")
	secretsWallets := db.Collection("secretsWallets")

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
		return "false", "Trying to bypass"
	}

	accountData, isFound := modals.GetAccountData(address, accounts)
	if !isFound {
		return "false", "No Account Found"
	}
	isRefreshAble, message := checkRefreshCount(accounts, accountData.REFRESH, address)
	if !isRefreshAble {
		return "false", message
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
	oldEvmAddress := accountData.EADD
	// fetching balance from chain POS
	newPosBalance := oldPosBalance
	newERCBalance := oldERCBalance
	isERCUpdated := false
	isPOSUpdated := false
	isChecked, chainPOSBalance, _ := CheckChainBalance(accountData.EADD, "POS")
	isCheckedERC, chainERCBalance, _ := CheckChainBalance(accountData.EADD, "ERC")
	changeAddress := false
	if isChecked {
		if chainPOSBalance != 0.00 {
			isInserted := modals.InsertIntoSecWallets(accountData.EADD, accountData.EKEY, chainPOSBalance, secretsWallets)
			if isInserted {
				newPosBalance = oldPosBalance + chainPOSBalance
				isUpdated := modals.UpdateBalance("POS", newPosBalance, accounts, address)
				if isUpdated {
					isPOSUpdated = true
					changeAddress = true
				}
			}
		}
	}

	if isCheckedERC {
		if chainERCBalance != 0.00 {
			isInserted := modals.InsertIntoSecWallets(accountData.EADD, accountData.EKEY, chainERCBalance, secretsWallets)
			if isInserted {
				newERCBalance = oldERCBalance + chainERCBalance
				isUpdated := modals.UpdateBalance("ERC", newERCBalance, accounts, address)
				if isUpdated {
					isERCUpdated = true
					changeAddress = true
				}
			}
		}
	}

	if changeAddress {
		isGenerated, newAddress := modals.CreateAndSaveNewAddress(address, accounts)
		if isGenerated {
			evmAddress = newAddress
		}
	}

	orderCollection := db.Collection("transferOrders")
	returnString := ""
	totalPOSBalanceString := fmt.Sprintf("%f", newPosBalance)
	totalERCBalanceString := fmt.Sprintf("%f", newERCBalance)
	if isPOSUpdated && isERCUpdated {
		// POS Handling
		chainPOSBalanceString := fmt.Sprintf("%f", chainPOSBalance)
		isOrderListUpdatedPOS, txnDataPOS := transfer.UpdateOrderList("0.00", "ON-CHAIN", address, oldEvmAddress, chainPOSBalanceString, "POS", "EXT", orderCollection)

		// ERC Handling
		chainERCBalanceString := fmt.Sprintf("%f", chainERCBalance)
		isOrderListUpdatedERC, txnDataERC := transfer.UpdateOrderList("0.00", "ON-CHAIN", address, oldEvmAddress, chainERCBalanceString, "ERC", "EXT", orderCollection)

		if !isOrderListUpdatedPOS && !isOrderListUpdatedERC {
			returnString = "false"
		} else {
			returnTXN := ""

			if isOrderListUpdatedPOS && isOrderListUpdatedERC {
				txnDetails := strings.Split(txnDataPOS, ",")
				unixTimeInt64, _ := strconv.ParseInt(txnDetails[0], 10, 64)
				unixTimeString := fmt.Sprintf("%d", unixTimeInt64)
				newChainBalance := chainPOSBalance + chainERCBalance
				newChainBalanceString := fmt.Sprintf("%f", newChainBalance)
				returnTXN = fmt.Sprintf("true,BTH,%s,%s,%s,%s,%s", newChainBalanceString, unixTimeString, evmAddress, totalPOSBalanceString, totalERCBalanceString)
			} else if isOrderListUpdatedERC {
				txnDetails := strings.Split(txnDataERC, ",")
				unixTimeInt64, _ := strconv.ParseInt(txnDetails[0], 10, 64)
				unixTimeString := fmt.Sprintf("%d", unixTimeInt64)
				returnTXN = fmt.Sprintf("true,ERC,%s,%s,%s,%s,%s", chainERCBalanceString, unixTimeString, evmAddress, totalPOSBalanceString, totalERCBalanceString)
			} else if isOrderListUpdatedPOS {
				txnDetails := strings.Split(txnDataPOS, ",")
				unixTimeInt64, _ := strconv.ParseInt(txnDetails[0], 10, 64)
				unixTimeString := fmt.Sprintf("%d", unixTimeInt64)
				returnTXN = fmt.Sprintf("true,ERC,%s,%s,%s,%s,%s", chainPOSBalanceString, unixTimeString, evmAddress, totalPOSBalanceString, totalERCBalanceString)
			} else {
				returnTXN = "false"
			}

			returnString = returnTXN
		}
	} else if isPOSUpdated {
		chainPOSBalanceString := fmt.Sprintf("%f", chainPOSBalance)
		isOrderListUpdatedPOS, txnDataPOS := transfer.UpdateOrderList("0.00", "ON-CHAIN", address, oldEvmAddress, chainPOSBalanceString, "POS", "EXT", orderCollection)
		if !isOrderListUpdatedPOS {
			returnString = "false"
		} else {
			txnDetails := strings.Split(txnDataPOS, ",")
			unixTime := txnDetails[0]
			unixTimeInt64, _ := strconv.ParseInt(unixTime, 10, 64)
			unixTimeString := fmt.Sprintf("%d", unixTimeInt64)
			returnString = fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s", "true", "POS", chainPOSBalanceString, unixTimeString, evmAddress, totalPOSBalanceString, totalERCBalanceString)
		}
	} else if isERCUpdated {
		chainERCBalanceString := fmt.Sprintf("%f", chainERCBalance)
		isOrderListUpdatedERC, txnDataERC := transfer.UpdateOrderList("0.00", "ON-CHAIN", address, oldEvmAddress, chainERCBalanceString, "ERC", "EXT", orderCollection)

		if !isOrderListUpdatedERC {
			returnString = "false"
		} else {
			txnDetails := strings.Split(txnDataERC, ",")
			unixTime := txnDetails[0]
			unixTimeInt64, _ := strconv.ParseInt(unixTime, 10, 64)
			unixTimeString := fmt.Sprintf("%d", unixTimeInt64)
			returnString = fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s", "true", "POS", chainERCBalanceString, unixTimeString, evmAddress, totalPOSBalanceString, totalERCBalanceString)
		}
	} else {
		returnString = "false"
	}
	return "true", returnString
}

func checkRefreshCount(accounts *mongo.Collection, currentRefresh string, address string) (bool, string) {
	refreshLimitTime := int64(3600)
	currentRefreshList := strings.Split(currentRefresh, ",")
	if len(currentRefreshList) != 2 {
		return false, "Can not get refresh counts"
	}
	utcNow := time.Now().UTC()
	unixTimestamp := utcNow.Unix()
	oneHourLater := unixTimestamp + refreshLimitTime
	refreshCount, err := strconv.ParseInt(currentRefreshList[0], 10, 64)
	if err != nil {
		return false, "Can not get refresh counts"
	}
	futureTime, err := strconv.ParseInt(currentRefreshList[1], 10, 64)
	if err != nil {
		return false, "Can not get refresh limit"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if refreshCount == 0 || futureTime == 0 || futureTime < unixTimestamp {
		newData := fmt.Sprintf("%d,%d", 1, oneHourLater)
		update := bson.M{"$set": bson.M{
			"REFRESH": newData,
		}}
		filter := bson.M{"ID": address}
		accounts.UpdateOne(ctx, filter, update)
		return true, ""
	} else {
		if refreshCount >= 20 {
			return false, "Refresh Limit exceeded"
		} else {
			newRefreshCount := refreshCount + 1
			// Allow sync and increment count
			newData := fmt.Sprintf("%d,%d", newRefreshCount, futureTime)
			update := bson.M{"$set": bson.M{
				"REFRESH": newData,
			}}
			filter := bson.M{"ID": address}
			accounts.UpdateOne(ctx, filter, update)
			return true, ""
		}
	}
}

// ERC20 ABI including balanceOf and decimals functions for robustness.
const erc20ABI = `[
    {
        "constant": true,
        "inputs": [
            {
                "name": "_owner",
                "type": "address"
            }
        ],
        "name": "balanceOf",
        "outputs": [
            {
                "name": "balance",
                "type": "uint256"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    },
    {
        "constant": true,
        "inputs": [],
        "name": "decimals",
        "outputs": [
            {
                "name": "",
                "type": "uint8"
            }
        ],
        "payable": false,
        "stateMutability": "view",
        "type": "function"
    }
]`

func CheckChainBalance(walletAddressStr string, chainChoice string) (bool, float64, string) {
	var rpcURL string
	var tokenContractAddressStr string
	var tokenSymbol string
	err := godotenv.Load()
	if err != nil {
		return false, 0.00, "Problem at Backend"
	}

	ERCAPI := os.Getenv("ERC_API")
	if ERCAPI == "" {
		return false, 0.00, "Problem at Backend"
	}

	POSAPI := os.Getenv("POS_API")
	if POSAPI == "" {
		return false, 0.00, "Problem at Backend"
	}
	if chainChoice == "POS" {
		rpcURL = fmt.Sprintf("%s%s", "https://go.getblock.io/", POSAPI)
		tokenContractAddressStr = "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"
		tokenSymbol = "USDT (Polygon)"
	} else if chainChoice == "ERC" {
		rpcURL = fmt.Sprintf("%s%s", "https://go.getblock.io/", ERCAPI)
		tokenContractAddressStr = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
		tokenSymbol = "USDT (Ethereum)"
	} else {
		return false, 0.00, "Invalid chain choice. Use 'POLYGON' or 'ETHEREUM'."
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Printf("Failed to connect to %s client: %v", chainChoice, err)
		return false, 0.00, fmt.Sprintf("Failed to connect to %s blockchain", chainChoice)
	}
	defer client.Close()

	contractAbi, err := abi.JSON(strings.NewReader(erc20ABI)) // Use the more complete ABI
	if err != nil {
		return false, 0.00, "Internal error: Failed to parse contract ABI"
	}

	contractAddress := common.HexToAddress(tokenContractAddressStr)
	walletAddress := common.HexToAddress(walletAddressStr)

	// --- Get Token Decimals (Dynamic) ---
	var decimals uint8
	callDataDecimals, err := contractAbi.Pack("decimals")
	if err != nil {
		return false, 0.00, "Internal error: Failed to prepare decimals call"
	}

	resultDecimals, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddress,
		Data: callDataDecimals,
	}, nil)
	if err != nil {
		return false, 0.00, fmt.Sprintf("Failed to retrieve %s decimals (check contract address/RPC)", tokenSymbol)
	}

	err = contractAbi.UnpackIntoInterface(&decimals, "decimals", resultDecimals)
	if err != nil {
		return false, 0.00, fmt.Sprintf("Failed to parse %s decimals", tokenSymbol)
	}

	// --- Get Token Balance ---
	callDataBalanceOf, err := contractAbi.Pack("balanceOf", walletAddress)
	if err != nil {
		return false, 0.00, "Internal error: Failed to prepare balance call"
	}

	resultBalanceOf, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddress,
		Data: callDataBalanceOf,
	}, nil)
	if err != nil {
		if strings.Contains(err.Error(), "403 Forbidden") || strings.Contains(err.Error(), "access denied") {
			return false, 0.00, "Access denied by RPC provider"
		}
		return false, 0.00, fmt.Sprintf("Failed to retrieve %s balance", tokenSymbol)
	}

	rawBalance := new(big.Int)
	err = contractAbi.UnpackIntoInterface(&rawBalance, "balanceOf", resultBalanceOf)
	if err != nil {
		return false, 0.00, fmt.Sprintf("Failed to parse %s balance", tokenSymbol)
	}

	fRawBalance := new(big.Float).SetInt(rawBalance)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	humanReadableBalance := new(big.Float).Quo(fRawBalance, divisor)

	finalBalance, err := strconv.ParseFloat(humanReadableBalance.String(), 64)
	if err != nil {
		return false, 0.00, "Internal error: Balance conversion failed"
	}
	return true, finalBalance, ""
}
