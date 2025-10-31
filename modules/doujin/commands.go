package doujin

import "github.com/bwmarrin/discordgo"

var DoujinCommand = []*discordgo.ApplicationCommand{
	{
		Name:        "doujin",
		Description: "Get doujin information from nhentai.net by code",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "code",
				Description: "The nhentai doujin code (e.g., 297974)",
				Required:    true,
			},
		},
	},
}