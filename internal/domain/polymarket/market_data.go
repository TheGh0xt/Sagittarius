package polymarket

// Trade is one executed trade from the Data API.
type Trade struct {
	Wallet      string  `json:"proxyWallet"`
	Side        string  `json:"side"` // BUY / SELL
	Price       float64 `json:"price"`
	Size        float64 `json:"size"`
	ConditionID string  `json:"conditionId"`
	Timestamp   int64   `json:"timestamp"`
}

type OrderbookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type Orderbook struct {
	AssetID string           `json:"asset_id"`
	Bids    []OrderbookLevel `json:"bids"`
	Asks    []OrderbookLevel `json:"asks"`
}
