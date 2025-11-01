package math

import "github.com/bwmarrin/discordgo"

var CalculateCommand = []*discordgo.ApplicationCommand{
	{
		Name:        "collatzconjecture",
		Description: "Perform a mathematical calculation",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "int",
				Description: "No matter what positive integer you start with, you will eventually reach the number 1.",
				Required:    true,
			},
		},
	},
}
