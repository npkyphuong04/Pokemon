package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"encoding/json"
	"io/ioutil"
	"log"
	"github.com/gorilla/websocket"
)

// Định nghĩa cấu trúc Cell, đại diện cho mỗi ô trong bản đồ
type Cell struct {
	Pokemon *Pokemon // Lưu Pokémon nếu có trong ô
}

// Định nghĩa cấu trúc Pokémon
type Pokemon struct {
	Name      string      `json:"name"`      // Tên Pokémon
	Level     int         `json:"level"`     // Cấp độ của Pokémon
	EV        float64     `json:"ev"`        // EV (Effort Value)
	SpawnedAt time.Time   // Thời gian Pokémon xuất hiện
}

// Định nghĩa cấu trúc Player, đại diện cho người chơi
type Player struct {
	ID       int        // ID của người chơi
	X, Y     int        // Vị trí của người chơi trên bản đồ
	Captured []*Pokemon // Danh sách Pokémon mà người chơi đã bắt được
}

const (
	MapSize          = 20              // Kích thước bản đồ
	PokemonPerWave   = 50              // Số lượng Pokémon xuất hiện mỗi lần
	PokemonLifetime  = 5 * time.Minute // Thời gian tồn tại của Pokémon trên bản đồ
	MaxPokemons      = 200             // Số lượng Pokémon tối đa mà người chơi có thể bắt
)

var (
	world    [MapSize][MapSize]Cell // Bản đồ thế giới, lưu trữ các ô có Pokémon
	player   *Player                // Người chơi hiện tại
	lock     sync.Mutex              // Mutex để đồng bộ hóa truy cập vào dữ liệu
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

var globalMutex sync.Mutex

// Biến toàn cục để lưu trữ dữ liệu về Pokémon
var pokemonData []Pokemon

// Hàm để tải dữ liệu Pokémon từ file JSON
func loadPokemonData() {
	data, err := ioutil.ReadFile("pokemon.json")
	if err != nil {
		log.Fatalf("Failed to read pokemon.json: %v", err)
	}

	err = json.Unmarshal(data, &pokemonData)
	if err != nil {
		log.Fatalf("Failed to parse pokemon.json: %v", err)
	}

	// Giới hạn số lượng Pokémon tối đa là 50
	if len(pokemonData) > 50 {
		pokemonData = pokemonData[:50]
	}

	fmt.Printf("Loaded %d Pokémon from json\n", len(pokemonData))
}

// Hàm để spawn (tạo) Pokémon mới trên bản đồ
func spawnPokemon() {
	lock.Lock()
	defer lock.Unlock()

	// Tạo Pokémon mới
	for i := 0; i < PokemonPerWave; i++ {
		if len(pokemonData) == 0 {
			fmt.Println("No Pokémon data available to spawn.")
			return
		}

		// Chọn ngẫu nhiên một Pokémon từ danh sách dữ liệu
		pokemon := pokemonData[rand.Intn(len(pokemonData))]

		// Cấp độ ngẫu nhiên từ 1 đến 100
		level := rand.Intn(100) + 1

		// EV ngẫu nhiên từ 0.5 đến 1
		ev := 0.5 + rand.Float64()*(1.0-0.5)

		// Tọa độ ngẫu nhiên trên bản đồ
		x := rand.Intn(MapSize)
		y := rand.Intn(MapSize)

		// Tạo Pokémon và spawn vào bản đồ tại vị trí (x, y)
		world[x][y].Pokemon = &Pokemon{
			Name:      pokemon.Name,
			Level:     level,
			EV:        ev,
			SpawnedAt: time.Now(),
		}
	}
}

// Hàm để despawn (xóa) Pokémon đã quá thời gian tồn tại
func despawnPokemon() {
	lock.Lock()
	defer lock.Unlock()

	now := time.Now()
	for x := 0; x < MapSize; x++ {
		for y := 0; y < MapSize; y++ {
			if world[x][y].Pokemon != nil && now.Sub(world[x][y].Pokemon.SpawnedAt) > PokemonLifetime {
				world[x][y].Pokemon = nil
			}
		}
	}
}

// Hàm để thêm người chơi vào game
func addPlayer() (*Player, error) {
	lock.Lock()
	defer lock.Unlock()

	if player != nil {
		return nil, fmt.Errorf("a player is already in the game")
	}

	// Tạo người chơi mới với vị trí ngẫu nhiên trên bản đồ
	player = &Player{
		ID: rand.Int(),
		X:  rand.Intn(MapSize),
		Y:  rand.Intn(MapSize),
	}
	return player, nil
}

// Hàm để bắt Pokémon
func capturePokemon(p *Player) {
	// Kiểm tra vị trí người chơi có hợp lệ trong phạm vi bản đồ không
	if p.X < 0 || p.X >= MapSize || p.Y < 0 || p.Y >= MapSize {
		fmt.Println("Player is out of bounds, cannot capture Pokémon.")
		return
	}

	// Sử dụng mutex toàn cục để tránh xung đột trên dữ liệu game
	globalMutex.Lock()
	defer globalMutex.Unlock()

	// Tiến hành bắt Pokémon nếu có tại vị trí người chơi
	cell := &world[p.X][p.Y]
	for cell.Pokemon != nil && len(p.Captured) < MaxPokemons {
		// Cập nhật danh sách Pokémon của người chơi
		p.Captured = append(p.Captured, cell.Pokemon)
		pokemon := cell.Pokemon // Lưu trữ Pokémon tạm thời
		cell.Pokemon = nil

		// In thông tin về Pokémon đã bắt
		fmt.Printf("Player %d captured a Pokémon: %v at position X = %d, Y = %d\n", p.ID, pokemon.Name, p.X, p.Y)

		// In chi tiết Pokémon
		fmt.Printf("Captured Pokémon Details:\n")
		fmt.Printf("- Name: %s\n", pokemon.Name)
		fmt.Printf("- Level: %d\n", pokemon.Level)
		fmt.Printf("- EV: %.2f\n", pokemon.EV)

		// Kiểm tra lại ô hiện tại để xem có Pokémon mới không
		cell = &world[p.X][p.Y]
	}
	if cell.Pokemon == nil {
		fmt.Printf("No Pokémon to capture at position X = %d, Y = %d\n", p.X, p.Y)
	}

	// In ra dãy Pokémon đã bắt được
	fmt.Println("Captured Pokémon:")
	for _, pokemon := range p.Captured {
		fmt.Printf("- %s\n", pokemon.Name)
	}
}
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}

	p, err := addPlayer()
	if err != nil {
		conn.WriteJSON(map[string]string{"error": "Only one player allowed"})
		fmt.Println(err)
		conn.Close()
		return
	}

	fmt.Printf("Player %d connected\n", p.ID)

	defer func() {
		lock.Lock()
		player = nil
		lock.Unlock()
		fmt.Printf("Player %d disconnected\n", p.ID)
	}()

	gameLoop(conn, p)
}

// Hàm chính để xử lý vòng lặp game
func gameLoop(conn *websocket.Conn, p *Player) {
	for {
		// Gửi dữ liệu thế giới và người chơi cho client
		data := map[string]interface{}{
			"player": map[string]int{
				"X": p.X,
				"Y": p.Y,
			},
			"world": world,
		}
		err := conn.WriteJSON(data)
		if err != nil {
			fmt.Println("Error writing to websocket:", err)
			return
		}

		// Đọc dữ liệu di chuyển từ client
		var move map[string]interface{}
		err = conn.ReadJSON(&move)
		if err != nil {
			fmt.Println("Error reading from websocket:", err)
			return
		}

		fmt.Printf("Received JSON: %v\n", move)

		// Kiểm tra nếu "capture" là boolean
		if capture, ok := move["capture"].(bool); ok && capture {
			fmt.Println("Attempting to capture a Pokémon!")
			capturePokemon(p)
			continue
		}

		// Kiểm tra giá trị "dx" và "dy"
		dx, dxOk := move["dx"].(float64)
		dy, dyOk := move["dy"].(float64)
		if dxOk && dyOk {
			// Cập nhật vị trí người chơi
			if p.X+int(dx) >= 0 && p.X+int(dx) < MapSize {
				p.X += int(dx)
			}
			if p.Y+int(dy) >= 0 && p.Y+int(dy) < MapSize {
				p.Y += int(dy)
			}
			fmt.Printf("Player new position: X = %d, Y = %d\n", p.X, p.Y)
		}

		// Tự động bắt Pokémon nếu có
		capturePokemon(p)
	}
}


// Step 1: Khởi tạo hàm chính của server
func main() {
	rand.Seed(time.Now().UnixNano())

	// Tải dữ liệu Pokémon từ JSON
	loadPokemonData()

	// Tạo Pokémon mỗi phút
	go func() {
		for {
			spawnPokemon()
			time.Sleep(1 * time.Minute)
		}
	}()

	// Xóa Pokémon quá thời gian tồn tại
	go func() {
		for {
			despawnPokemon()
			time.Sleep(10 * time.Second)
		}
	}()

	// Khởi động server HTTP
	http.HandleFunc("/ws", wsHandler)
	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", nil)
}
