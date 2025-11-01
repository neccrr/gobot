package math

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func RegisterCollatzConjectureHandler(s *discordgo.Session) {
	s.AddHandler(handleCollatzConjectureCommand)
}

func collatzConjecture(n int) []int {
	sequence := []int{n}
	for n != 1 {
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
		sequence = append(sequence, n)
	}
	return sequence
}

func processInput(inputStr string) ([]int, error) {
	var numbers []int
	parts := strings.Split(inputStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid number in range: %s", part)
			}
			for i := start; i <= end; i++ {
				numbers = append(numbers, i)
			}
		} else {
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", part)
			}
			numbers = append(numbers, num)
		}
	}
	return numbers, nil
}

// ProcessCollatzConjecture Now takes input string as a parameter instead of reading from stdin
func ProcessCollatzConjecture(inputStr string) {
	inputNumbers, err := processInput(strings.TrimSpace(inputStr))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	startTime := time.Now()

	for _, num := range inputNumbers {
		result := collatzConjecture(num)
		fmt.Printf("Collatz sequence for %d: %v\n", num, result)
	}

	elapsed := time.Since(startTime)

	// System info
	fmt.Println("\n=== Summary ===")
	fmt.Printf("Machine Info: %s on %s (%s)\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Printf("NumCPU: %d, GOMAXPROCS: %d\n", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	fmt.Printf("Performance: Time taken - %.6f seconds\n", elapsed.Seconds())
}

func handleCollatzConjectureCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isCollatzConjectureCommand(i) {
		return
	}

	inputStr := i.ApplicationCommandData().Options[0].StringValue()

	inputNumbers, err := processInput(strings.TrimSpace(inputStr))
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error: " + err.Error(),
			},
		})
		return
	}

	// send waiting response to avoid interaction timeout
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		fmt.Println("failed to send deferred response:", err)
		return
	}

	startTime := time.Now()
	var responseStrings []string

	for _, num := range inputNumbers {
		result := collatzConjecture(num)
		responseStrings = append(responseStrings, fmt.Sprintf("Collatz sequence for %d: %v", num, result))
	}

	elapsed := time.Since(startTime)

	summary := fmt.Sprintf("\n=== Summary ===\nMachine Info: %s on %s (%s)\nNumCPU: %d, GOMAXPROCS: %d\nPerformance: Time taken - %.6f seconds",
		runtime.GOOS, runtime.GOARCH, runtime.Version(),
		runtime.NumCPU(), runtime.GOMAXPROCS(0), elapsed.Seconds())

	fullResponse := strings.Join(responseStrings, "\n")
	// save full response to local filesystem ./calc/collatz_conjecture_output_TIMESTAMP.txt
	saveToFile := func(content string) {
		filename := fmt.Sprintf("./calc/collatz_conjecture_output_%s.txt", time.Now().Format("20060102150405"))
		err := os.WriteFile(filename, []byte(content+summary), 0644)
		if err != nil {
			fmt.Println("failed to save output to file:", err)
		} else {
			fmt.Println("output saved to file:", filename)
		}
	}
	saveToFile(fullResponse)

	first2000Response := fullResponse
	if len(fullResponse) > 1000 {
		first2000Response = fullResponse[:1000] + "\n\n[Output truncated. See full output below.]" + summary
	}
	// cancel the previous deferred response and send the full response
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &first2000Response,
		// Full response in file attachment if too long
		Files: func() []*discordgo.File {
			if len(fullResponse) > 1000 {
				return []*discordgo.File{
					{
						// random name for the file
						Name:        "collatz_conjecture_output_" + time.Now().Format("20060102150405") + ".txt",
						ContentType: "text/plain",
						Reader:      strings.NewReader(fullResponse + summary),
					},
				}
			}
			return nil
		}(),
	})
	if err != nil {
		fmt.Println("failed to edit response:", err)
	}

	fmt.Println("\n=== Summary ===")
	fmt.Printf("Machine Info: %s on %s (%s)\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Printf("NumCPU: %d, GOMAXPROCS: %d\n", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	fmt.Printf("Performance: Time taken - %.6f seconds\n", elapsed.Seconds())
	fmt.Printf("Memory Stats before GC: %+v\n", runtime.MemStats{})

	//GC to free memory
	runtime.GC()
	fmt.Println("\n=== GARBAGE COLLECTION ===")
	fmt.Printf("Memory Stats after GC: %+v\n", runtime.MemStats{})
}

func isCollatzConjectureCommand(i *discordgo.InteractionCreate) bool {
	return i != nil &&
		i.Interaction != nil &&
		i.Type == discordgo.InteractionApplicationCommand &&
		i.ApplicationCommandData().Name == "collatzconjecture"
}
