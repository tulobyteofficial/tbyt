package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"tbapi/exchange"
	"tbapi/fetch"
	"tbapi/modals"
	"tbapi/staking"
	"tbapi/transfer"
)

func createAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isCreated, message := modals.CreateAccount(r)

	response := map[string]interface{}{
		"status":  isCreated,
		"message": message,
	}
	json.NewEncoder(w).Encode(response)
}

func recoverAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isRecoverd, data := modals.RecoverAccount(r)

	response := map[string]interface{}{
		"status":  isRecoverd,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func refreshAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isRecoverd, data := modals.RefreshAccount(r)

	response := map[string]interface{}{
		"status":  isRecoverd,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func getPlatfromInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := modals.PlatformInfo(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func getVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := modals.GetVersion()

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func transferAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := transfer.TransferAssets(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func getTransferOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
	}

	isFetched, data := transfer.GetOrderData(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func placeExchangeOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isPlaced, settledMessage, status, EID, amount, fromCurrency, toCurrency, initAddress := exchange.PlaceExchangeOrder(r)
	settledStatus := "pending"
	var amountSettled float64
	// var postResponseLock sync.Mutexsss
	if isPlaced == "true" && status == "pending" {
		settledStatus, amountSettled = exchange.FindAndSettleOrders(r, EID, amount, fromCurrency, toCurrency, initAddress)
	}
	newData := fmt.Sprintf("%s,%s,%f,%s", settledStatus, settledMessage, amountSettled, EID.Hex())
	response := map[string]interface{}{
		"status":  isPlaced,
		"message": newData,
	}
	json.NewEncoder(w).Encode(response)
}

func cancelExchangeOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
	}

	isFetched, data, purpose := exchange.CencelExchange(r)
	newData := fmt.Sprintf("%s,%s", data, purpose)
	response := map[string]interface{}{
		"status":  isFetched,
		"message": newData,
	}
	json.NewEncoder(w).Encode(response)

}
func getExchangeOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := exchange.GetExOrderData(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func getSwapAmounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := exchange.GetSwapAmounts(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func placeStakeOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := staking.PlaceStake(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func getStakesOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := staking.GetStakeOrderData(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}

func unstakeNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := staking.Unstake(r)
	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)

}

func fetchChainBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // Ensure only POST is allowed
		response := map[string]interface{}{
			"status":  "false",
			"message": "Invalid Request Method",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isFetched, data := fetch.FetchChainBalance(r)

	response := map[string]interface{}{
		"status":  isFetched,
		"message": data,
	}
	json.NewEncoder(w).Encode(response)
}
func main() {
	http.HandleFunc("/getVersion", getVersion)
	http.HandleFunc("/create", createAccountHandler)

	http.HandleFunc("/recover", recoverAccountHandler)
	http.HandleFunc("/platformInfo", getPlatfromInfo)

	http.HandleFunc("/refreshWallet", refreshAccountHandler)

	// Transfer
	http.HandleFunc("/makeTransfer", transferAssets)
	http.HandleFunc("/getTransferOrders", getTransferOrders)

	// Exchanges/Swap
	http.HandleFunc("/placeExOrder", placeExchangeOrder)
	http.HandleFunc("/getExchangeOrders", getExchangeOrders)
	http.HandleFunc("/completeExchangeOrder", cancelExchangeOrder)
	http.HandleFunc("/swapAvailAmount", getSwapAmounts)

	// Stakes
	http.HandleFunc("/placeStake", placeStakeOrder)
	http.HandleFunc("/getStakesOrder", getStakesOrder)
	http.HandleFunc("/unstakeNow", unstakeNow)
	// v1.0.2
	// fetching onchain balance
	http.HandleFunc("/fetchChainBalance", fetchChainBalance)
	certFile := "/root/TBYT/cert.pem"
	keyFile := "/root/TBYT/key.pem"
	port := ":2021"
	log.Printf("Server running on https://localhost%s\n", port)
	err := http.ListenAndServeTLS(port, certFile, keyFile, nil)
	if err != nil {
		log.Fatal(err)
	}
}
