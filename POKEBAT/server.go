package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

// Define the structure for Pokémon with stats and elemental effects.
type Pokemon struct {
	Name             string
	HP               int
	Attack           int
	Defense          int
	SpecialAttack    int
	SpecialDefense   int
	Speed            int
	ElementalEffects map[string]float64 // e.g., "fire": 1.5, "water": 0.8
	Experience       int
	IsFainted        bool  
}

// Structure for receiving Pokémon selection from the client.
type PokemonChoice struct {
	Choice int `json:"choice"`
}

// Structure representing a player, including connection and active Pokémon info.
type Player struct {
	Name     string
	Pokemons []Pokemon
	Active   int // Index of the active Pokémon
	Conn     net.Conn
	IsFainted bool
}

// Response structure used for communication with clients.
type Response struct {
	Result string `json:"result"`
}

type ActionRequest struct {
	Action string `json:"action"`
}

// Represents the game's state, including players and whose turn it is.
type GameState struct {
	Player1     Player
	Player2     Player
	Turn        int  // 1 for Player1, 2 for Player2
	Player1Done bool // Flag to track if Player 1 is done selecting Pokémon
	Player2Done bool // Flag to track if Player 2 is done selecting Pokémon
}

// Utility function to send JSON-encoded messages to a client.
func sendJSON(conn net.Conn, message string) {
	err := json.NewEncoder(conn).Encode(Response{Result: message})
	if err != nil {
		fmt.Println("Error sending JSON:", err)
	}
}

// Load data from a JSON file into the provided interface.
func LoadJSON(filename string, v interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(v)
}

// Handle player name input.
func handlePlayerName(conn net.Conn) string {
	var data map[string]string
	decoder := json.NewDecoder(conn)

	if err := decoder.Decode(&data); err != nil {
		fmt.Println("Error decoding player name:", err)
		sendJSON(conn, "Failed to receive player name.")
		return "Unknown"
	}

	name, ok := data["name"]
	if !ok || strings.TrimSpace(name) == "" {
		sendJSON(conn, "Invalid name received. Defaulting to 'Player'.")
		return "Player"
	}

	sendJSON(conn, fmt.Sprintf("Welcome, %s! Please select your Pokémon.", name))
	return name
}

func handlePokemonSelection(player *Player, pokedex []Pokemon, gameState *GameState) {
	decoder := json.NewDecoder(player.Conn)
	player.Pokemons = make([]Pokemon, 0, 3) // Initialize a slice to store up to 3 Pokémon choices.
	selectedIndexes := make(map[int]bool) // Map to track already selected Pokémon indexes.

	// Wait for three Pokémon choices from the player
	for i := 0; i < 3; i++ {
		var choice PokemonChoice
		if err := decoder.Decode(&choice); err != nil || choice.Choice < 0 || choice.Choice >= len(pokedex) || selectedIndexes[choice.Choice] {
			// If the choice is invalid or already selected, send an error message and reject the selection
			sendJSON(player.Conn, "Invalid or already selected Pokémon choice. Please select a different Pokémon.")
			i-- // Decrement to retry this choice
			continue
		}

		// Add the selected Pokémon to the player's collection and mark it as selected
		player.Pokemons = append(player.Pokemons, pokedex[choice.Choice])
		selectedIndexes[choice.Choice] = true // Mark this Pokémon as selected

		// Notify the player of their choice
		sendJSON(player.Conn, fmt.Sprintf("You chose %s as your Pokémon #%d.", player.Pokemons[i].Name, i+1))
	}

	// After all three Pokémon have been selected, notify the player
	sendJSON(player.Conn, "You have selected all your Pokémon.")

	// Mark this player as done
	if player == &gameState.Player1 {
		gameState.Player1Done = true
	} else {
		gameState.Player2Done = true
	}

	// Wait until both players are done selecting Pokémon
	if gameState.Player1Done && gameState.Player2Done {
		sendJSON(gameState.Player1.Conn, "Both players have selected their Pokémon. The battle will begin now!")
		sendJSON(gameState.Player2.Conn, "Both players have selected their Pokémon. The battle will begin now!") //Gửi tin nhắn tới người chơi qua kết nối mạng
	}
}

// Calculate damage dealt by an attack based on the Pokémon's stats and effects.
func calculateDamage(attacker, defender Pokemon, isSpecial bool) int {
	attackStat := attacker.Attack
	defenseStat := defender.Defense

	if isSpecial {
		attackStat = attacker.SpecialAttack
		defenseStat = defender.SpecialDefense
	}

	// Simple formula for damage
	damage := (attackStat - defenseStat/2) + rand.Intn(10)
	if damage < 0 {
		damage = 0
	}
	return damage
}

// Switch the active Pokémon for the player.
func switchPokemon(player *Player) {
	player.Active = (player.Active + 1) % len(player.Pokemons)
}

func handleBattle(gameState *GameState) {
	defer gameState.Player1.Conn.Close()
	defer gameState.Player2.Conn.Close()

	// Announce initial Pokémon for both players.
	sendJSON(gameState.Player1.Conn, fmt.Sprintf("Your Pokémon: %s (HP: %d)", gameState.Player1.Pokemons[gameState.Player1.Active].Name, gameState.Player1.Pokemons[gameState.Player1.Active].HP))
	sendJSON(gameState.Player2.Conn, fmt.Sprintf("Opponent Pokémon: %s (HP: %d)", gameState.Player1.Pokemons[gameState.Player1.Active].Name, gameState.Player1.Pokemons[gameState.Player1.Active].HP))
	sendJSON(gameState.Player2.Conn, fmt.Sprintf("Your Pokémon: %s (HP: %d)", gameState.Player2.Pokemons[gameState.Player2.Active].Name, gameState.Player2.Pokemons[gameState.Player2.Active].HP))
	sendJSON(gameState.Player1.Conn, fmt.Sprintf("Opponent Pokémon: %s (HP: %d)", gameState.Player2.Pokemons[gameState.Player2.Active].Name, gameState.Player2.Pokemons[gameState.Player2.Active].HP))

	// Determine who goes first based on the Speed stat of each player's active Pokémon.
	player1Speed := gameState.Player1.Pokemons[gameState.Player1.Active].Speed
	player2Speed := gameState.Player2.Pokemons[gameState.Player2.Active].Speed

	if player1Speed > player2Speed {
		gameState.Turn = 1 // Player 1 goes first.
	} else if player2Speed > player1Speed {
		gameState.Turn = 2 // Player 2 goes first.
	} else {
		// If the speeds are the same, you can either randomize or use the current turn order.
		gameState.Turn = rand.Intn(2) + 1
	}
	
	for {
		var currentPlayer, opponent *Player
		if gameState.Turn == 1 {
			currentPlayer = &gameState.Player1
			opponent = &gameState.Player2
		} else {
			currentPlayer = &gameState.Player2
			opponent = &gameState.Player1
		}

		// Notify players about the turn status.
		sendJSON(currentPlayer.Conn, "It's your turn!")
		sendJSON(opponent.Conn, "Waiting for opponent's move")

		// Read action from the current player.
		actionBytes := make([]byte, 256)
		n, err := currentPlayer.Conn.Read(actionBytes)
		if err != nil {
			fmt.Println("Error reading action:", err)
			return
		}

		//Converting action a string 
		actionStr := strings.TrimSpace(string(actionBytes[:n]))
		fmt.Println("Received action:", actionStr)

		// Parse the action from the JSON string.
		var actionRequest ActionRequest
		err = json.Unmarshal([]byte(actionStr), &actionRequest)
		if err != nil {
			fmt.Println("Error parsing action:", err)
			sendJSON(currentPlayer.Conn, "Invalid action format. Please try again.")
			return
		}

		action := actionRequest.Action
		fmt.Println("Parsed action:", action)

		// Handle the action: attack, switch, or surrender.
		switch action {
		case "attack":
			isSpecial := rand.Intn(2) == 0 // Randomly decide if it's a special attack.
			damage := calculateDamage(currentPlayer.Pokemons[currentPlayer.Active], opponent.Pokemons[opponent.Active], isSpecial)
		
			opponent.Pokemons[opponent.Active].HP -= damage
			if opponent.Pokemons[opponent.Active].HP < 0 {
				opponent.Pokemons[opponent.Active].HP = 0
			}
		
			sendJSON(currentPlayer.Conn, fmt.Sprintf("You dealt %d damage to %s. Remaining HP: %d",
				damage,
				opponent.Pokemons[opponent.Active].Name,
				opponent.Pokemons[opponent.Active].HP))
			sendJSON(opponent.Conn, fmt.Sprintf("%s dealt %d damage to your %s. Remaining HP: %d",
				currentPlayer.Pokemons[currentPlayer.Active].Name,
				damage,
				opponent.Pokemons[opponent.Active].Name,
				opponent.Pokemons[opponent.Active].HP))
		
			// Check if the opponent's active Pokémon fainted.
			if opponent.Pokemons[opponent.Active].HP == 0 {
				opponent.Pokemons[opponent.Active].IsFainted = true // Mark the Pokémon as fainted with a flag.
				sendJSON(opponent.Conn, fmt.Sprintf("%s fainted!", opponent.Pokemons[opponent.Active].Name))
		
				// Check if all Pokémon are fainted.
				allFainted := true
				for _, pkmn := range opponent.Pokemons {
					if !pkmn.IsFainted {
						allFainted = false
						break
					}
				}
		
				if allFainted {
					// All opponent's Pokémon have fainted, calculate experience and end the game.
					sendJSON(currentPlayer.Conn, "You win!")
					sendJSON(opponent.Conn, "You lose!")
					distributeExperience(currentPlayer, opponent)
					return
				}
		
				// Switch to a new Pokémon if some are still available.
				switchPokemon(opponent)
			}

			case "switch":
				// Switch the active Pokémon.
				switchPokemon(currentPlayer)
				sendJSON(currentPlayer.Conn, fmt.Sprintf("Switched to %s.", currentPlayer.Pokemons[currentPlayer.Active].Name))

			case "surrender":
				// Handle surrender action.
				sendJSON(currentPlayer.Conn, "You surrendered! Game over.")
				sendJSON(opponent.Conn, "Your opponent surrendered! You win!")
				distributeExperience(opponent, currentPlayer)
				return

			
		}
		// Switch the turn to the other player.
		gameState.Turn = 3 - gameState.Turn // Alternates between 1 and 2.
	}
	}

	func distributeExperience(winningPlayer, losingPlayer *Player) {
		// Calculate the total experience from the losing team's Pokémon.
		totalExp := 0
		for _, pkmn := range losingPlayer.Pokemons {
			totalExp += pkmn.Experience
		}
	
		if totalExp == 0 {
			sendJSON(winningPlayer.Conn, "No experience gained as the losing team has no accumulated experience.")
			return
		}
	
		// Each Pokémon in the winning team gets 1/3 of the total experience.
		expShare := totalExp / 3
	
		for i := range winningPlayer.Pokemons {
			winningPlayer.Pokemons[i].Experience += expShare
		}
	
		for i := range winningPlayer.Pokemons {
			beforeExp := winningPlayer.Pokemons[i].Experience
			winningPlayer.Pokemons[i].Experience += expShare
			afterExp := winningPlayer.Pokemons[i].Experience
			sendJSON(winningPlayer.Conn, fmt.Sprintf(
				"%s gained %d experience. Total experience: %d -> %d.",
				winningPlayer.Pokemons[i].Name,
				expShare,
				beforeExp,
				afterExp,
			))
		}
		
		sendJSON(winningPlayer.Conn, fmt.Sprintf("Each of your Pokémon gained %d experience.", expShare))
	}

func main() {
	rand.Seed(time.Now().UnixNano()) // Initialize random seed for gameplay randomness.

	// Load Pokémon data for both players from JSON files.
	var pokedex1, pokedex2 []Pokemon
	if err := LoadJSON("pokedex_player1.json", &pokedex1); err != nil { //Tải dữ 
		fmt.Println("Error loading pokedex_player1.json:", err)
		return
	}
	if err := LoadJSON("pokedex_player2.json", &pokedex2); err != nil {
		fmt.Println("Error loading pokedex_player2.json:", err)
		return
	}

	// Step 1: Open port 8080 to connect between 2 clients
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Server started, waiting for players...")

	// Step 2: Connect to 2 clients
	conn1, err := listener.Accept()
	if err != nil {
		fmt.Println("Error accepting connection:", err)
		return
	}
	fmt.Println("Player 1 connected")

	conn2, err := listener.Accept()
	if err != nil {
		fmt.Println("Error accepting connection:", err)
		return
	}
	fmt.Println("Player 2 connected")

	//Step 3: Initialize game state, player name, connect internet of 2 player, player 1 and 2
	gameState := GameState{
		Player1: Player{Name: "Player 1", Conn: conn1, Pokemons: pokedex1},
		Player2: Player{Name: "Player 2", Conn: conn2, Pokemons: pokedex2},
		Turn:    1,
	}


	// Encryption number of players from 1, 2 to json so that it can transmist information
	fmt.Println("Sending player numbers...")
	json.NewEncoder(conn1).Encode(1)
	json.NewEncoder(conn2).Encode(2)

	//Step 3: Process name players
	fmt.Println("Waiting for players to send their names...")
	gameState.Player1.Name = handlePlayerName(conn1)
	gameState.Player2.Name = handlePlayerName(conn2)

	//Step 4: Chooose pokemon
	fmt.Println("Waiting for Pokémon selection from players...")
	handlePokemonSelection(&gameState.Player1, pokedex1, &gameState)
	handlePokemonSelection(&gameState.Player2, pokedex2, &gameState)

	// Start the battle.
	
	fmt.Println("Starting the battle...")
	handleBattle(&gameState)
	
	
}
