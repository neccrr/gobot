package doujin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RegisterDoujinHandler sets up command and reaction handlers
func RegisterDoujinHandler(session *discordgo.Session) {
	session.AddHandler(handleCommand)
	session.AddHandler(handleReaction)
}

// handleCommand processes the /doujin command
func handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isDoujinCommand(i) {
		return
	}

	code := extractCode(i)
	if code == "" {
		respondError(s, i, "❌ Please provide a valid code")
		return
	}

	if err := acknowledgeInteraction(s, i); err != nil {
		log.Printf("Failed to acknowledge interaction: %v", err)
		return
	}

	doujin, err := fetchDoujin(code)
	if err != nil {
		sendFollowupError(s, i, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	embed := buildInfoEmbed(doujin, code)
	msg, err := sendFollowup(s, i, embed)
	if err != nil {
		log.Printf("Failed to send followup (error): %v", err)
		sendFollowupError(s, i, "❌ Failed to send message")
		return
	}
	if msg == nil {
		log.Printf("Failed to send followup: message is nil but no error returned")
		sendFollowupError(s, i, "❌ Failed to send message (nil response)")
		return
	}

	storeSession(msg.ID, createSession(doujin, code, msg.ChannelID, getUserID(i)))
	err = s.MessageReactionAdd(msg.ChannelID, msg.ID, "📖")
	if err != nil {
		log.Printf("Failed to add reaction: %v", err)
		return
	}

	downloadDoujin, s2, err := FetchAndDownloadDoujin(code, "./downloads")
	if err != nil {
		log.Printf("Failed to fetch and download doujin: %v", err)
		return
	}
	if downloadDoujin == nil {
		log.Printf("Failed to fetch and download doujin: doujin is %v", downloadDoujin)
		return
	}
	if s2 == "" {
		log.Printf("Failed to fetch and download doujin: path is empty")
		return
	}
}

// handleReaction processes emoji reactions
func handleReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if !isValidReaction(s, r) {
		return
	}

	switch r.Emoji.Name {
	case "📖":
		openReader(s, r)
	case "⬅️", "➡️", "⏹️":
		navigateReader(s, r)
	}
}

// isDoujinCommand checks if interaction is for /doujin command
func isDoujinCommand(i *discordgo.InteractionCreate) bool {
	return i != nil &&
		i.Interaction != nil &&
		i.Type == discordgo.InteractionApplicationCommand &&
		i.ApplicationCommandData().Name == "doujin"
}

// isValidReaction checks if reaction is valid and not from bot
func isValidReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) bool {
	return s != nil &&
		r != nil &&
		s.State != nil &&
		s.State.User != nil &&
		r.UserID != s.State.User.ID
}

// extractCode gets the code parameter from command options
func extractCode(i *discordgo.InteractionCreate) string {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return ""
	}
	return strings.TrimSpace(options[0].StringValue())
}

// fetchDoujin retrieves data from the API
func fetchDoujin(code string) (*DoujinData, error) {
	url := fmt.Sprintf("https://nhentai.net/api/gallery/%s", code)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("code %s not found", code)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var data DoujinData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %v", err)
	}

	return &data, nil
}

// buildInfoEmbed creates the information embed
func buildInfoEmbed(doujin *DoujinData, code string) *discordgo.MessageEmbed {
	artists, languages, tags := extractTags(doujin)

	embed := &discordgo.MessageEmbed{
		Title: doujin.Title.Pretty,
		URL:   fmt.Sprintf("https://nhentai.net/g/%s", code),
		Color: 0x8A2BE2,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Pages", Value: fmt.Sprintf("%d", doujin.NumPages), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Code: %s", code)},
	}

	if len(doujin.Images.Pages) > 0 {
		ext := getExtension(doujin.Images.Pages[0].Type)
		embed.Image = &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("https://t.nhentai.net/galleries/%s/cover.%s", doujin.MediaID, ext),
		}
	}

	if len(artists) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: "Artists", Value: strings.Join(artists, ", "), Inline: true,
		})
	}
	if len(languages) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: "Languages", Value: strings.Join(languages, ", "), Inline: true,
		})
	}
	if len(tags) > 0 {
		maxTags := min(5, len(tags))
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: "Tags", Value: strings.Join(tags[:maxTags], ", "), Inline: false,
		})
	}

	return embed
}

// extractTags separates tags by type
func extractTags(doujin *DoujinData) (artists, languages, tags []string) {
	for _, tag := range doujin.Tags {
		switch tag.Type {
		case "artist":
			artists = append(artists, tag.Name)
		case "language":
			languages = append(languages, tag.Name)
		case "tag":
			tags = append(tags, tag.Name)
		}
	}
	return
}

// openReader opens a new reader session
func openReader(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	sessionMutex.RLock()
	original, exists := originalMessages[r.MessageID]
	sessionMutex.RUnlock()

	if !exists || original == nil || len(original.PageExts) == 0 {
		return
	}

	embed := buildReaderEmbed(original, 0)
	msg, err := s.ChannelMessageSendEmbed(original.ChannelID, embed)
	if err != nil || msg == nil {
		log.Printf("Failed to send reader embed: %v", err)
		return
	}

	addNavigationReactions(s, msg)

	newSession := &ReadSession{
		OwnerID:   r.UserID,
		MediaID:   original.MediaID,
		PageExts:  original.PageExts,
		Current:   0,
		Total:     original.Total,
		ChannelID: msg.ChannelID,
		Code:      original.Code,
	}

	sessionMutex.Lock()
	activeReaders[msg.ID] = newSession
	sessionMutex.Unlock()

	s.MessageReactionRemove(original.ChannelID, r.MessageID, "📖", r.UserID)
}

// navigateReader handles page navigation
func navigateReader(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	sessionMutex.RLock()
	session, exists := activeReaders[r.MessageID]
	sessionMutex.RUnlock()

	if !exists || session == nil || r.UserID != session.OwnerID {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return
	}

	emoji := r.Emoji.Name

	if emoji == "⏹️" {
		s.ChannelMessageDelete(session.ChannelID, r.MessageID)
		sessionMutex.Lock()
		delete(activeReaders, r.MessageID)
		sessionMutex.Unlock()
		return
	}

	if emoji == "⬅️" && session.Current > 0 {
		session.Current--
	} else if emoji == "➡️" && session.Current < session.Total-1 {
		session.Current++
	}

	updateReader(s, session, r.MessageID)
	s.MessageReactionRemove(session.ChannelID, r.MessageID, emoji, r.UserID)
}

// buildReaderEmbed creates a reader page embed
func buildReaderEmbed(session *ReadSession, page int) *discordgo.MessageEmbed {
	if page < 0 || page >= len(session.PageExts) {
		log.Printf("Invalid page index: %d (total pages: %d)", page, len(session.PageExts))
		return &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("%s — Error", session.Code),
			Description: "Invalid page number",
			Color:       0xFF0000,
		}
	}

	imgURL := fmt.Sprintf("https://i.nhentai.net/galleries/%s/%d.%s",
		session.MediaID, page+1, session.PageExts[page])

	log.Printf("Reader embed - Code: %s, Page: %d/%d, URL: %s",
		session.Code, page+1, session.Total, imgURL)

	return &discordgo.MessageEmbed{
		Title: fmt.Sprintf("%s — Page %d/%d", session.Code, page+1, session.Total),
		Image: &discordgo.MessageEmbedImage{URL: imgURL},
		Color: 0x8A2BE2,
	}
}

// updateReader updates the current page
func updateReader(s *discordgo.Session, session *ReadSession, msgID string) {
	if session.Current < 0 || session.Current >= len(session.PageExts) {
		return
	}

	embed := buildReaderEmbed(session, session.Current)
	if _, err := s.ChannelMessageEditEmbed(session.ChannelID, msgID, embed); err != nil {
		log.Printf("Failed to update reader: %v", err)
	}
}

// addNavigationReactions adds navigation buttons
func addNavigationReactions(s *discordgo.Session, msg *discordgo.Message) {
	reactions := []string{"⬅️", "⏹️", "➡️"}
	for _, emoji := range reactions {
		s.MessageReactionAdd(msg.ChannelID, msg.ID, emoji)
	}
}

// createSession creates a new session from doujin data
func createSession(doujin *DoujinData, code, channelID, ownerID string) *ReadSession {
	pageExts := make([]string, len(doujin.Images.Pages))
	for i, page := range doujin.Images.Pages {
		pageExts[i] = getExtension(page.Type)
	}

	return &ReadSession{
		OwnerID:   ownerID,
		MediaID:   doujin.MediaID,
		PageExts:  pageExts,
		Total:     doujin.NumPages,
		ChannelID: channelID,
		Code:      code,
	}
}

// storeSession stores a session in the original messages map
func storeSession(msgID string, session *ReadSession) {
	sessionMutex.Lock()
	originalMessages[msgID] = session
	sessionMutex.Unlock()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Interaction helpers
func acknowledgeInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func sendFollowup(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if i == nil || i.Interaction == nil {
		return nil, fmt.Errorf("invalid interaction object")
	}

	// Note: The second parameter is 'wait' - true means wait for the message object to be returned
	msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "🔞 NSFW Content",
		Embeds:  []*discordgo.MessageEmbed{embed},
	})

	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, fmt.Errorf("FollowupMessageCreate returned nil message without error")
	}

	return msg, nil
}

func sendFollowupError(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}
