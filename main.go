package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	_ "strings"
	"time"

	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	"test/modules/crypto"
	"test/modules/doujin"
	"test/modules/math"
)

// use godot package to load/read the .env file and
// return the value of the key
func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	log.Println(os.Getenv(key))

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

// Bot parameters
var (
	GuildID        = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	BotToken       = flag.String("token", goDotEnvVariable("TOKEN"), "Bot access token")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
)

var s *discordgo.Session

func init() {
	flag.Parse()

	var err error
	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}

	// go routine for garbage collection
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			runtime.GC()
			fmt.Println("Garbage collection executed")
			fmt.Println("Memory stats:", runtime.MemStats{})
		}
	}()
}

func main() {
	// Initialize logger
	if err := initLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer closeLogger()

	log.Println("Bot starting up...")

	// Register command handlers
	doujin.RegisterDoujinHandler(s)
	crypto.RegisterCryptoHandler(s)
	math.RegisterCollatzConjectureHandler(s)

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(doujin.DoujinCommand), len(crypto.CryptoCommand))
	for i, v := range doujin.DoujinCommand {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
		log.Printf("Added '%v' command: %v", v.Name, v.Description)
	}
	for _, v := range crypto.CryptoCommand {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands = append(registeredCommands, cmd)
		log.Printf("Added '%v' command: %v", v.Name, v.Description)
	}
	for _, v := range math.CalculateCommand {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands = append(registeredCommands, cmd)
		log.Printf("Added '%v' command: %v", v.Name, v.Description)
	}

	server := "172.67.68.109"
	port := uint16(25565)
	username := "nigergamer"

	startRoutine(server, port, username)
	log.Println("Bot is now running.  Press CTRL-C to exit.")

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *RemoveCommands {
		log.Println("Removing commands...")
		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, *GuildID, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}

func writeVarInt(buf *bytes.Buffer, value int) {
	for {
		temp := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			temp |= 0x80
		}
		buf.WriteByte(temp)
		if value == 0 {
			break
		}
	}
}

func writeString(buf *bytes.Buffer, s string) {
	writeVarInt(buf, len(s))
	buf.WriteString(s)
}

func sendMinecraftHandshake(conn net.Conn, serverAddr string, port uint16, username string) error {
	// Handshake packet
	handshake := &bytes.Buffer{}
	writeVarInt(handshake, 0x00)       // Packet ID
	writeVarInt(handshake, 754)        // Protocol version
	writeString(handshake, serverAddr) // Server address
	writeString(handshake, "ily faced")
	err := binary.Write(handshake, binary.BigEndian, port)
	if err != nil {
		return err
	} // Port
	writeVarInt(handshake, 2) // Next state: login

	// Send handshake
	packet := &bytes.Buffer{}
	writeVarInt(packet, handshake.Len())
	packet.Write(handshake.Bytes())
	if _, err := conn.Write(packet.Bytes()); err != nil {
		return err
	}

	// Login Start packet
	login := &bytes.Buffer{}
	writeVarInt(login, 0x00)     // Packet ID
	writeString(login, username) // Username
	fmt.Fprintf(conn, handshake.String(), handshake.String())
	log.Printf("Handshake sent")

	packet.Reset()
	writeVarInt(packet, login.Len())
	packet.Write(login.Bytes())
	if _, err := conn.Write(packet.Bytes()); err != nil {
		return err
	}

	return nil
}

func startRoutine(server string, port uint16, username string) {
	go func() {
		for {
			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
			if err != nil {
				fmt.Println("Connection error:", err)
				time.Sleep(2 * time.Second)
				continue
			}

			err = sendMinecraftHandshake(conn, server, port, username)
			log.Println("Sent handshake to", server)
			if err != nil {
				fmt.Println("Packet error:", err)
			} else {
				fmt.Println("Packets sent successfully.")
			}

			conn.Close()
			time.Sleep(2 * time.Second) // Repeat every 10 seconds
		}
	}()
}
