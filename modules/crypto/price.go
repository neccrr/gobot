package crypto

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// getCryptoPrice fetches the current price of a cryptocurrency
func getCryptoPrice(symbol string) (float64, error) {
	// Convert symbol to lowercase for API call
	symbolLower := strings.ToLower(symbol)

	// Using CoinGecko API - you might need to get an API key for higher rate limits
	// For demonstration, we'll use a simple mapping
	coinID := mapSymbolToCoinID(symbolLower)

	if coinID == "" {
		return 0, fmt.Errorf("unsupported cryptocurrency symbol: %s", symbol)
	}

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s", coinID)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var prices []struct {
		CurrentPrice float64 `json:"current_price"`
		Symbol       string  `json:"symbol"`
		Name         string  `json:"name"`
	}

	err = json.Unmarshal(body, &prices)
	if err != nil {
		return 0, err
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("cryptocurrency not found: %s", symbol)
	}

	return prices[0].CurrentPrice, nil
}

// mapSymbolToCoinID maps common symbols to CoinGecko coin IDs
func mapSymbolToCoinID(symbol string) string {
	symbolMap := map[string]string{
		"btc":       "bitcoin",
		"bitcoin":   "bitcoin",
		"eth":       "ethereum",
		"ethereum":  "ethereum",
		"ada":       "cardano",
		"cardano":   "cardano",
		"doge":      "dogecoin",
		"dogecoin":  "dogecoin",
		"dot":       "polkadot",
		"polkadot":  "polkadot",
		"sol":       "solana",
		"solana":    "solana",
		"matic":     "matic-network",
		"polygon":   "matic-network",
		"avax":      "avalanche-2",
		"avalanche": "avalanche-2",
		"link":      "chainlink",
		"chainlink": "chainlink",
		"xrp":       "ripple",
		"ripple":    "ripple",
		"ltc":       "litecoin",
		"litecoin":  "litecoin",
		"bnb":       "binancecoin",
		"binance":   "binancecoin",
	}

	return symbolMap[symbol]
}
