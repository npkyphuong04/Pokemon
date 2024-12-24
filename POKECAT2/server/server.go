package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DefaultWorldSize   = 1000
	DefaultMaxPokemons = 200
	DefaultSpawnRate   = 50
	DefaultDespawnTime = 5 * time.Minute
)

type Coordinate struct {
	x, y int
}

type Pokemon struct {
	Name      string
	Level     int
	EV        float64
	SpawnedAt time.Time
}

type Player struct {
	ID       string
	Position Coordinate
	Pokemons []Pokemon
	Mutex    sync.Mutex
}

type GameServer struct {
	World       map[Coordinate]*Pokemon
	Players     map[string]*Player
	Mutex       sync.Mutex
	Pokedex     []string
	NewPlayers  chan net.Conn
	WorldSize   int
	MaxPokemons int
	SpawnRate   int
	DespawnTime time.Duration
}

func NewGameServer() *GameServer {
	return &GameServer{
		World:       make(map[Coordinate]*Pokemon),
		Players:     make(map[string]*Player),
		Pokedex:     []string{"Pikachu", "Charmander", "Bulbasaur", "Squirtle"},
		NewPlayers:  make(chan net.Conn),
		WorldSize:   DefaultWorldSize,
		MaxPokemons: DefaultMaxPokemons,
		SpawnRate:   DefaultSpawnRate,
		DespawnTime: DefaultDespawnTime,
	}
}

func (server *GameServer) Start() {
	go server.acceptConnections()
	go server.spawnPokemon()
	server.gameLoop()
}

func (server *GameServer) acceptConnections() {
	ln, err := net.Listen("tcp", ":8000")
	if err != nil {
		panic(err)
	}
	fmt.Println("Server started and listening on port: " + ln.Addr().String())
	for {
		conn, err := ln.Accept()
		if err == nil {
			server.NewPlayers <- conn // Add new player to the channel
		}
	}
}

func (server *GameServer) addPlayer(conn net.Conn) {
	id := fmt.Sprintf("player_%d", time.Now().UnixNano())
	player := &Player{ // Create a new player
		ID: id,
		Position: Coordinate{
			x: rand.Intn(server.WorldSize),
			y: rand.Intn(server.WorldSize),
		},
	}
	server.Mutex.Lock()                  // Lock the server mutex
	server.Players[id] = player          // Add the player to the server
	server.Mutex.Unlock()                // Unlock the server mutex
	go server.handlePlayer(conn, player) // Handle the player in a goroutine
}

func (server *GameServer) handlePlayer(conn net.Conn, player *Player) {
	defer func() {
		server.savePlayerPokemons(player)
		conn.Close()
		fmt.Printf("Connection closed for player %s. Pokémon data saved.\n", player.ID)
	}()
	defer conn.Close()
	for {
		buffer := make([]byte, 1024) // Create a buffer to read the command
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}
		command := string(buffer[:n])
		command = strings.TrimSpace(command)
		switch command {
		case "UP":
			player.Move(0, -1, server.WorldSize)
		case "DOWN":
			player.Move(0, 1, server.WorldSize)
		case "LEFT":
			player.Move(-1, 0, server.WorldSize)
		case "RIGHT":
			player.Move(1, 0, server.WorldSize)
		case "INVENTORY":
			server.showInventory(player, conn)
		default:
			conn.Write([]byte("Invalid command\n"))
			continue
		}
		server.checkForPokemon(player, conn) // Check for Pokemon in the player's position
	}
	server.Mutex.Lock()
	delete(server.Players, player.ID)
	server.Mutex.Unlock()
}

func (server *GameServer) showInventory(player *Player, conn net.Conn) {
	player.Mutex.Lock() // Lock the player mutex
	defer player.Mutex.Unlock()
	if len(player.Pokemons) == 0 { // Check if the player has any Pokemon
		conn.Write([]byte("Your inventory is empty.\n"))
		return
	}
	inventory := "Your Pokemon inventory:\n"
	for i, pokemon := range player.Pokemons {
		inventory += fmt.Sprintf("%d: %s (Level %d, EV %.2f)\n", i, pokemon.Name, pokemon.Level, pokemon.EV)
	}
	conn.Write([]byte(inventory)) // Send the inventory message to the player
}

// Update the position of the player
func (player *Player) Move(dx, dy, worldSize int) {
	player.Mutex.Lock()
	defer player.Mutex.Unlock()
	player.Position.x = (player.Position.x + dx + worldSize) % worldSize
	player.Position.y = (player.Position.y + dy + worldSize) % worldSize
	fmt.Printf("Player moved to position: (%d, %d)\n", player.Position.x, player.Position.y)
}

func (server *GameServer) checkForPokemon(player *Player, conn net.Conn) {
	server.Mutex.Lock()
	defer server.Mutex.Unlock()
	pokemon, exists := server.World[player.Position] // Check if there is a Pokemon in the player's position
	if exists {
		player.Mutex.Lock()
		if len(player.Pokemons) < server.MaxPokemons { // Check if the player's inventory is full
			player.Pokemons = append(player.Pokemons, *pokemon)
			conn.Write([]byte(fmt.Sprintf("You captured a %s at position (%d, %d)!\n", pokemon.Name, player.Position.x, player.Position.y)))
		} else {
			conn.Write([]byte(fmt.Sprintf("Your Pokemon inventory is full at position (%d, %d)!\n", player.Position.x, player.Position.y)))
		}
		player.Mutex.Unlock()
		delete(server.World, player.Position) // Remove the Pokemon from the world
	} else {
		conn.Write([]byte(fmt.Sprintf("No Pokemon here at position (%d, %d).\n", player.Position.x, player.Position.y)))
	}
}

func (server *GameServer) spawnPokemon() {
	ticker := time.NewTicker(time.Minute) // Create a ticker that ticks every minute
	for range ticker.C {                  //The loop will execute once for each value sent by the ticker's channel c
		server.Mutex.Lock()
		for i := 0; i < server.SpawnRate; i++ {
			coord := Coordinate{
				x: rand.Intn(server.WorldSize),
				y: rand.Intn(server.WorldSize),
			}
			pokemon := &Pokemon{
				Name:      server.Pokedex[rand.Intn(len(server.Pokedex))],
				Level:     rand.Intn(100) + 1,
				EV:        0.5 + rand.Float64()*0.5,
				SpawnedAt: time.Now(), // Set the spawn time to the current time
			}
			server.World[coord] = pokemon // Add the Pokemon to the world
			fmt.Printf("Spawned %s at position: (%d, %d)\n", pokemon.Name, coord.x, coord.y)
		}
		server.Mutex.Unlock()
	}
}

func (server *GameServer) gameLoop() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		server.Mutex.Lock()

		// Iterate over the world and despawn Pokemon that have been spawned for more than the despawn time
		for coord, pokemon := range server.World {
			if time.Since(pokemon.SpawnedAt) > server.DespawnTime { // Check if the Pokemon has been spawned for more than the despawn time
				delete(server.World, coord)
				fmt.Printf("Despawned %s from position: (%d, %d)\n", pokemon.Name, coord.x, coord.y)
			}
		}
		server.Mutex.Unlock()
	}
}

func (server *GameServer) savePlayerPokemons(player *Player) {
	player.Mutex.Lock()
	defer player.Mutex.Unlock()
	file, err := os.Create(fmt.Sprintf("%s_pokemons.json", player.ID)) // Create a file with the player's ID
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(player.Pokemons)
	if err != nil {
		fmt.Printf("Error encoding JSON: %v\n", err)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano()) // Seed the random number generator
	server := NewGameServer()
	go func() {
		for conn := range server.NewPlayers { // Add new players to the server
			server.addPlayer(conn)
		}
	}()
	fmt.Println("Starting PokeCat server...")
	server.Start()
}
