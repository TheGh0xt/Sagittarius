package signal

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/polymarket"
)

// Engine computes deterministic signals from Polymarket data without invoking an LLM.
type Engine struct {
	client *polymarket.Client
}

// NewEngine creates a signal engine backed by the given Polymarket client.
func NewEngine(client *polymarket.Client) *Engine {
	return &Engine{client: client}
}

// WhaleEvent represents a single trade whose USD notional exceeds the whale threshold.
type WhaleEvent struct {
	TradeID   string    `json:"trade_id"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	ValueUSD  float64   `json:"value_usd"`
	Side      string    `json:"side"`
	Timestamp time.Time `json:"timestamp"`
}

// VolumeSignal captures recent vs historical volume and flags spikes.
type VolumeSignal struct {
	RecentVolumeUSD   float64 `json:"recent_volume_usd"`
	AverageVolumeUSD  float64 `json:"average_volume_usd"`
	VelocityChangePct float64 `json:"velocity_change_pct"`
	IsSpike           bool    `json:"is_spike"`
}

// OrderbookSkew captures the imbalance between bid and ask dollar-weighted depth.
type OrderbookSkew struct {
	BidVolume float64 `json:"bid_volume"`
	AskVolume float64 `json:"ask_volume"`
	Skew      float64 `json:"skew"`   // (Bid - Ask) / (Bid + Ask) -> range [-1, 1]
	Spread    float64 `json:"spread"` // bestAsk - bestBid
}

// MarketSnapshot is a unified state vector for a single market.
type MarketSnapshot struct {
	MarketID       string        `json:"market_id"`
	Question       string        `json:"question"`
	Active         bool          `json:"active"`
	Closed         bool          `json:"closed"`
	CurrentPrice   float64       `json:"current_price"`
	ClobTokenID    string        `json:"clob_token_id"`
	SkewInfo       OrderbookSkew `json:"skew_info"`
	RecentWhales   []WhaleEvent  `json:"recent_whales"`
	VolumeAnalysis VolumeSignal  `json:"volume_analysis"`
	Timestamp      time.Time     `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Pure computation helpers (no I/O – safe to unit-test without mocking)
// ---------------------------------------------------------------------------

// ComputeWhaleEvents filters trades whose notional (price × size) >= thresholdUSD.
func ComputeWhaleEvents(trades []polymarket.Trade, thresholdUSD float64) []WhaleEvent {
	var events []WhaleEvent
	for _, t := range trades {
		val := t.Price * t.Size
		if val >= thresholdUSD {
			events = append(events, WhaleEvent{
				TradeID:   t.ID,
				Price:     t.Price,
				Size:      t.Size,
				ValueUSD:  val,
				Side:      t.Side,
				Timestamp: time.Unix(t.Timestamp, 0),
			})
		}
	}
	return events
}

// ComputeOrderbookSkew calculates bid/ask dollar-weighted imbalance and spread
// from a raw orderbook snapshot.
func ComputeOrderbookSkew(ob *polymarket.Orderbook) OrderbookSkew {
	var bidVol, askVol float64

	for _, bid := range ob.Bids {
		p, _ := strconv.ParseFloat(bid.Price, 64)
		s, _ := strconv.ParseFloat(bid.Size, 64)
		bidVol += p * s
	}
	for _, ask := range ob.Asks {
		p, _ := strconv.ParseFloat(ask.Price, 64)
		s, _ := strconv.ParseFloat(ask.Size, 64)
		askVol += p * s
	}

	skew := 0.0
	if total := bidVol + askVol; total > 0 {
		skew = (bidVol - askVol) / total
	}

	spread := 0.0
	if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
		bestAsk, _ := strconv.ParseFloat(ob.Asks[0].Price, 64)
		bestBid, _ := strconv.ParseFloat(ob.Bids[0].Price, 64)
		spread = math.Abs(bestAsk - bestBid)
	}

	return OrderbookSkew{
		BidVolume: bidVol,
		AskVolume: askVol,
		Skew:      skew,
		Spread:    spread,
	}
}

// ComputeVolumeSignal compares recent trade volume against a baseline average
// and flags whether the velocity change constitutes a spike (>200%).
func ComputeVolumeSignal(trades []polymarket.Trade, baselineVolumeUSD float64) VolumeSignal {
	recentVol := 0.0
	for _, t := range trades {
		recentVol += t.Price * t.Size
	}

	velocityChange := 0.0
	if baselineVolumeUSD > 0 {
		velocityChange = ((recentVol - baselineVolumeUSD) / baselineVolumeUSD) * 100
	}

	return VolumeSignal{
		RecentVolumeUSD:   recentVol,
		AverageVolumeUSD:  baselineVolumeUSD,
		VelocityChangePct: velocityChange,
		IsSpike:           velocityChange > 200.0,
	}
}

// ---------------------------------------------------------------------------
// Orchestration methods (require live client)
// ---------------------------------------------------------------------------

// DetectWhaleActivity fetches recent trades and filters for whale-sized events.
func (e *Engine) DetectWhaleActivity(ctx context.Context, tokenID string, thresholdUSD float64, limit int) ([]WhaleEvent, error) {
	trades, err := e.client.GetTrades(ctx, tokenID, limit)
	if err != nil {
		return nil, err
	}
	return ComputeWhaleEvents(trades, thresholdUSD), nil
}

// AnalyzeOrderbookSkew fetches the live orderbook and computes skew metrics.
func (e *Engine) AnalyzeOrderbookSkew(ctx context.Context, tokenID string) (*OrderbookSkew, error) {
	ob, err := e.client.GetOrderbook(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	result := ComputeOrderbookSkew(ob)
	return &result, nil
}

// AnalyzeVolumeSpikes fetches recent trades and compares against market-level
// volume from Gamma metadata to detect anomalous spikes.
func (e *Engine) AnalyzeVolumeSpikes(ctx context.Context, market *polymarket.ParsedMarket, tokenID string) (*VolumeSignal, error) {
	recentTrades, err := e.client.GetTrades(ctx, tokenID, 100)
	if err != nil {
		return &VolumeSignal{}, nil
	}

	// Derive baseline from Gamma's all-time volume normalised to a 24h window.
	// Gamma reports total lifetime volume; we estimate daily average.
	// If market has been active for N days, daily average ≈ Volume/N.
	// As a reasonable default, assume the market has been live ~30 days.
	baselineVol := 10000.0 // fallback default
	if market != nil && market.Volume > 0 {
		baselineVol = market.Volume / 30.0 // rough daily average
	}

	result := ComputeVolumeSignal(recentTrades, baselineVol)
	return &result, nil
}

// GetMarketSnapshot aggregates price, orderbook skew, whale events, and volume
// analysis into a single unified state vector for a market.
func (e *Engine) GetMarketSnapshot(ctx context.Context, marketID string) (*MarketSnapshot, error) {
	m, err := e.client.GetMarket(ctx, marketID)
	if err != nil {
		return nil, err
	}

	tokenID := ""
	if len(m.ClobTokenIDs) > 0 {
		tokenID = m.ClobTokenIDs[0] // Primary token (typically YES outcome)
	}

	var currentPrice float64
	if len(m.OutcomePrices) > 0 {
		currentPrice = m.OutcomePrices[0]
	}

	var skew OrderbookSkew
	if tokenID != "" {
		if sk, err := e.AnalyzeOrderbookSkew(ctx, tokenID); err == nil {
			skew = *sk
		}
		if p, err := e.client.GetPrice(ctx, tokenID); err == nil {
			currentPrice = p
		}
	}

	var whales []WhaleEvent
	if tokenID != "" {
		if wh, err := e.DetectWhaleActivity(ctx, tokenID, 25000.0, 50); err == nil {
			whales = wh
		}
	}

	var volSignal VolumeSignal
	if tokenID != "" {
		if vs, err := e.AnalyzeVolumeSpikes(ctx, m, tokenID); err == nil {
			volSignal = *vs
		}
	}

	return &MarketSnapshot{
		MarketID:       m.ID,
		Question:       m.Question,
		Active:         m.Active,
		Closed:         m.Closed,
		CurrentPrice:   currentPrice,
		ClobTokenID:    tokenID,
		SkewInfo:       skew,
		RecentWhales:   whales,
		VolumeAnalysis: volSignal,
		Timestamp:      time.Now(),
	}, nil
}
