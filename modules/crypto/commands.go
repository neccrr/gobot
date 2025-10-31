package crypto

import "github.com/bwmarrin/discordgo"

var CryptoCommand = []*discordgo.ApplicationCommand{
	{
		Name:        "track",
		Description: "Get cryptocurrency information by symbol and track it then periodically update and tag the user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "symbol",
				Description: "The cryptocurrency symbol (e.g., BTC, ETH)",
				Required:    true,
			},
		},
	},
}
