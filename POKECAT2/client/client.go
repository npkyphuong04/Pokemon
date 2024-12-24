package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to PokeCat server!")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter command (UP, DOWN, LEFT, RIGHT, INVENTORY): ")
		command, _ := reader.ReadString('\n')
		command = strings.TrimSpace(command)
		_, err := conn.Write([]byte(command + "\n"))
		if err != nil {
			fmt.Println("Error sending command:", err)
			break
		}

		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil {
			fmt.Println("Error reading response:", err)
			break
		}
		fmt.Println("Server response:", string(response[:n]))
	}
}
