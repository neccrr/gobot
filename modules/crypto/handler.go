package crypto

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TrackingEntry represents a user's cryptocurrency tracking request
type TrackingEntry struct {
	UserID    string
	Symbol    string
	ChannelID string
	MessageID string
	LastPrice float64
}

var (
	// trackingMap stores active tracking entries
	trackingMap   = make(map[string]*TrackingEntry)
	trackingMutex sync.RWMutex
)

func RegisterCryptoHandler(session *discordgo.Session) {
	session.AddHandler(TrackHandler)
}

// TrackHandler handles the /track command
func TrackHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer the response first to avoid interaction timeout
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		log.Printf("Failed to acknowledge interaction: %v", err)
		return
	}

	// Get the symbol from command options
	options := i.ApplicationCommandData().Options
	symbol := strings.ToUpper(options[0].StringValue())

	// Validate symbol
	if symbol == "" {
		sendFollowupError(s, i, "❌ Please provide a valid cryptocurrency symbol (e.g., BTC, ETH)")
		return
	}

	// Get current price
	price, err := getCryptoPrice(symbol)
	if err != nil {
		log.Printf("Error fetching price for %s: %v", symbol, err)
		sendFollowupError(s, i, fmt.Sprintf("❌ Error fetching price for %s: %s", symbol, err.Error()))
		return
	}

	// Create initial response message
	embed := createPriceEmbed(symbol, price, getUser(i))

	// Send followup message with the embed
	msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Stop Tracking",
						Style:    discordgo.DangerButton,
						CustomID: fmt.Sprintf("stop_tracking_%s", symbol),
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Error sending followup message: %v", err)
		return
	}

	// Store tracking entry
	trackingMutex.Lock()
	trackingKey := fmt.Sprintf("%s_%s", getUserID(i), symbol)
	trackingMap[trackingKey] = &TrackingEntry{
		UserID:    getUserID(i),
		Symbol:    symbol,
		ChannelID: msg.ChannelID,
		MessageID: msg.ID,
		LastPrice: price,
	}
	trackingMutex.Unlock()

	log.Printf("Started tracking %s for user %s", symbol, getUser(i).Username)
}

// StopTrackingHandler handles the stop tracking button
func StopTrackingHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.MessageComponentData().CustomID == "" {
		return
	}

	// Extract symbol from custom ID
	customID := i.MessageComponentData().CustomID
	if !strings.HasPrefix(customID, "stop_tracking_") {
		return
	}

	symbol := strings.TrimPrefix(customID, "stop_tracking_")
	trackingKey := fmt.Sprintf("%s_%s", getUserID(i), symbol)

	// Remove from tracking
	trackingMutex.Lock()
	delete(trackingMap, trackingKey)
	trackingMutex.Unlock()

	// Update message to show tracking stopped
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("❌ Stopped Tracking %s", symbol),
		Description: fmt.Sprintf("No longer tracking %s for <@%s>", symbol, getUserID(i)),
		Color:       0xff0000, // Red
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}); err != nil {
		log.Printf("Error responding to stop tracking: %v", err)
		return
	}

	log.Printf("Stopped tracking %s for user %s", symbol, getUser(i).Username)
}

// UpdateTrackedPrices periodically updates all tracked cryptocurrency prices
func UpdateTrackedPrices(s *discordgo.Session) {
	ticker := time.NewTicker(5 * time.Minute) // Update every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		trackingMutex.RLock()
		trackings := make([]*TrackingEntry, 0, len(trackingMap))
		for _, entry := range trackingMap {
			trackings = append(trackings, entry)
		}
		trackingMutex.RUnlock()

		for _, entry := range trackings {
			go updateSinglePrice(s, entry)
			time.Sleep(1 * time.Second) // Rate limiting between updates
		}
	}
}

// updateSinglePrice updates the price for a single tracking entry
func updateSinglePrice(s *discordgo.Session, entry *TrackingEntry) {
	currentPrice, err := getCryptoPrice(entry.Symbol)
	if err != nil {
		log.Printf("Error updating price for %s: %v", entry.Symbol, err)
		return
	}

	// Only update if price has changed significantly (1% threshold)
	priceChange := (currentPrice - entry.LastPrice) / entry.LastPrice * 100
	if abs(priceChange) < 1.0 && entry.LastPrice != 0 {
		return
	}

	// Get user info for the embed
	user, err := s.User(entry.UserID)
	if err != nil {
		log.Printf("Error getting user info: %v", err)
		return
	}

	embed := createPriceEmbed(entry.Symbol, currentPrice, user)

	// Update the message using the same pattern as your reference
	_, err = s.ChannelMessageEditEmbed(entry.ChannelID, entry.MessageID, embed)
	if err != nil {
		log.Printf("Error updating message for %s: %v", entry.Symbol, err)
		// Remove tracking if message was deleted
		if strings.Contains(err.Error(), "Unknown Message") {
			trackingKey := fmt.Sprintf("%s_%s", entry.UserID, entry.Symbol)
			trackingMutex.Lock()
			delete(trackingMap, trackingKey)
			trackingMutex.Unlock()
		}
		return
	}

	// Update last price
	trackingMutex.Lock()
	if existingEntry, exists := trackingMap[fmt.Sprintf("%s_%s", entry.UserID, entry.Symbol)]; exists {
		existingEntry.LastPrice = currentPrice
	}
	trackingMutex.Unlock()

	// Send ping if price changed significantly
	if abs(priceChange) >= 5.0 {
		pingMsg := fmt.Sprintf("<@%s> %s price update: $%.2f (%.2f%%)",
			entry.UserID, entry.Symbol, currentPrice, priceChange)
		s.ChannelMessageSend(entry.ChannelID, pingMsg)
	}
}

// createPriceEmbed creates a discord embed for cryptocurrency price
func createPriceEmbed(symbol string, price float64, user *discordgo.User) *discordgo.MessageEmbed {
	var color int
	if price > 0 {
		color = 0x00ff00 // Green
	} else {
		color = 0xff0000 // Red
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("💰 %s Price Tracking", symbol),
		Description: fmt.Sprintf("Tracking **%s** for <@%s>", symbol, user.ID),
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Current Price",
				Value:  fmt.Sprintf("$%.2f", price),
				Inline: true,
			},
			{
				Name:   "Symbol",
				Value:  symbol,
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    fmt.Sprintf("Tracking for %s", user.Username),
			IconURL: user.AvatarURL(""),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// Helper functions matching your reference pattern
func getUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User
	}
	if i.User != nil {
		return i.User
	}
	return &discordgo.User{Username: "Unknown"}
}

func getUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func sendFollowupError(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
