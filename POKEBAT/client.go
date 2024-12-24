package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// PlayerNumber represents the player number assigned by the server.
type PlayerNumber struct {
	PlayerNum int `json:"playerNum"`
}

// PokemonChoice represents a player's choice of Pokémon.
type PokemonChoice struct {
	Choice int `json:"choice"`
}
type ActionResult struct {
	Result string `json:"result"`
	Damage int    `json:"damage"`
	RemainingHP int    `json:"remainingHP"` // Opponent's remaining HP

}

// ActionChoice represents a player's chosen action during their turn.
type ActionChoice struct {
	Action string `json:"action"`
}

// Response represents a generic response message from the server.
type Response struct {
	Result string `json:"result"`
}

func main() {
	// Connect to the server at localhost:8080.
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close() // Ensure connection is closed when the program exits.

	// Create a buffered reader for user input and JSON encoders/decoders for communication.
	reader := bufio.NewReader(os.Stdin) // Đọc đầu vào từ người dùng qua bàn phím.
	encoder := json.NewEncoder(conn) //Mã hóa dữ liệu Go thành JSON và gửi qua WebSocket.
	decoder := json.NewDecoder(conn) //Giải mã (chuyển đổi) dữ liệu JSON nhận được từ WebSocket thành các đối tượng Go.

	// Step 1: Receive and display the player number assigned by the server.
	var playerNum int
	if err := decoder.Decode(&playerNum); err != nil {
		fmt.Println("Error decoding player number:", err)
		return
	}

	// Prompt the player to enter their name.
	fmt.Printf("Enter your player name (Player #%d): ", playerNum)
	playerName, _ := reader.ReadString('\n')
	playerName = strings.TrimSpace(playerName)

	// Validate that the player name is not empty.
	if playerName == "" {
		fmt.Println("Player name cannot be empty. Exiting...")
		return
	}

	// Send the player's name to the server.
	if err := encoder.Encode(map[string]string{"name": playerName}); err != nil {
		fmt.Println("Error sending player name:", err)
		return
	}

	// Step 2: Receive and display the message to select Pokémon from the server.
	var response Response
	// After sending the player's name to the server.
	if err := encoder.Encode(map[string]string{"name": playerName}); err != nil {
		fmt.Println("Error sending player name:", err)
		return
	}

	// Wait for the server's response to acknowledge the name.
	if err := decoder.Decode(&response); err != nil {
		fmt.Println("Error decoding server response:", err)
		return
	}

	// Display server's response about the name acknowledgment.
	fmt.Println(response.Result)

	// Proceed to Pokémon selection after the server's acknowledgment.
	fmt.Println("Select your Pokémon!")
	for i := 1; i <= 3; i++ {
		for {
			fmt.Printf("Choose your Pokémon #%d (0-2): ", i)
			var choice int
			_, err := fmt.Scanf("%d\n", &choice)
			if err != nil || choice < 0 || choice > 2 {
				fmt.Println("Invalid choice. Please select a Pokémon between 0 and 2.")
				continue
			}

			// Send Pokémon choice to the server.
			if err := encoder.Encode(PokemonChoice{Choice: choice}); err != nil {
				fmt.Println("Error sending Pokémon choice:", err)
				return
			}

			// Wait for the server's acknowledgment.
			if err := decoder.Decode(&response); err != nil {
				fmt.Println("Error decoding server response:", err)
				return
			}

			// Display acknowledgment and proceed if valid.
			fmt.Println(response.Result)
			if strings.HasPrefix(response.Result, "You chose") {
				break
			}
		}
	}

// Step 4: Enter the game loop, alternating turns with the opponent.
	for {
		// Receive and display the server's response.
		var response Response
		if err := decoder.Decode(&response); err != nil {
			fmt.Println("Error decoding server response:", err)
			return
		}
		fmt.Println(response.Result)

		// Check for a game-over condition.
		if strings.HasPrefix(response.Result, "Game Over") {
			fmt.Println("Game has ended. Thank you for playing!")
			break
		}

		// If it's the player's turn, prompt for an action.
		if response.Result == "It's your turn!" {
			var action string
			for {
				fmt.Println("Choose an action: [attack/switch/surrender]")
				fmt.Scanln(&action)
				action = strings.TrimSpace(action) // Ensure no trailing whitespace or newline

				
				// Validate the action and break out of the loop only if the action is valid.
				if action == "attack" || action == "switch" || action == "surrender" {
					// Send the action choice to the server before breaking.
					actionChoice := ActionChoice{Action: action}

					if err := encoder.Encode(actionChoice); err != nil {
						fmt.Println("Error sending action choice:", err)
						return
					}
					// Once the action is sent, break out of the loop.
					break
				}

				fmt.Println("Invalid action. Try again.")
			}
			var actionResult ActionResult
			if err := decoder.Decode(&actionResult); err != nil {
				fmt.Println("Error decoding action result:", err)
				return
			}
			fmt.Println("Server Response:", actionResult.Result)
		}
	}

}
