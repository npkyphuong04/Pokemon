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
			server.NewPlayers <- conn
		}
	}
}

func (server *GameServer) addPlayer(conn net.Conn) {
	id := fmt.Sprintf("player_%d", time.Now().UnixNano())
	player := &Player{
		ID: id,
		Position: Coordinate{
			x: rand.Intn(server.WorldSize),
			y: rand.Intn(server.WorldSize),
		},
	}
	server.Mutex.Lock()
	server.Players[id] = player
	server.Mutex.Unlock()
	go server.handlePlayer(conn, player)
}

func (server *GameServer) handlePlayer(conn net.Conn, player *Player) {
	defer func() {
		server.savePlayerPokemons(player)
		conn.Close()
		fmt.Printf("Connection closed for player %s. Pokémon data saved.\n", player.ID)
	}()
	defer conn.Close()
	for {
		buffer := make([]byte, 1024)
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
		server.checkForPokemon(player, conn)
	}
	server.Mutex.Lock()
	delete(server.Players, player.ID)
	server.Mutex.Unlock()
}

func (server *GameServer) showInventory(player *Player, conn net.Conn) {
	player.Mutex.Lock()
	defer player.Mutex.Unlock()
	if len(player.Pokemons) == 0 {
		conn.Write([]byte("Your inventory is empty.\n"))
		return
	}
	inventory := "Your Pokemon inventory:\n"
	for i, pokemon := range player.Pokemons {
		inventory += fmt.Sprintf("%d: %s (Level %d, EV %.2f)\n", i, pokemon.Name, pokemon.Level, pokemon.EV)
	}
	conn.Write([]byte(inventory))
}

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
	pokemon, exists := server.World[player.Position]
	if exists {
		player.Mutex.Lock()
		if len(player.Pokemons) < server.MaxPokemons {
			player.Pokemons = append(player.Pokemons, *pokemon)
			conn.Write([]byte(fmt.Sprintf("You captured a %s at position (%d, %d)!\n", pokemon.Name, player.Position.x, player.Position.y)))
		} else {
			conn.Write([]byte(fmt.Sprintf("Your Pokemon inventory is full at position (%d, %d)!\n", player.Position.x, player.Position.y)))
		}
		player.Mutex.Unlock()
		delete(server.World, player.Position)
	} else {
		conn.Write([]byte(fmt.Sprintf("No Pokemon here at position (%d, %d).\n", player.Position.x, player.Position.y)))
	}
}

func (server *GameServer) spawnPokemon() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
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
				SpawnedAt: time.Now(),
			}
			server.World[coord] = pokemon
			fmt.Printf("Spawned %s at position: (%d, %d)\n", pokemon.Name, coord.x, coord.y)
		}
		server.Mutex.Unlock()
	}
}

func (server *GameServer) gameLoop() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		server.Mutex.Lock()
		for coord, pokemon := range server.World {
			if time.Since(pokemon.SpawnedAt) > server.DespawnTime {
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
	file, err := os.Create(fmt.Sprintf("%s_pokemons.json", player.ID))
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
	rand.Seed(time.Now().UnixNano())
	server := NewGameServer()
	go func() {
		for conn := range server.NewPlayers {
			server.addPlayer(conn)
		}
	}()
	fmt.Println("Starting PokeCat server...")
	server.Start()
}
