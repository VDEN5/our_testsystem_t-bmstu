package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var mu sync.Mutex

func compile(goFunc string) (error, string) {

	goCode := `
	package main

	import "fmt"
	` +
		goFunc +
		`
		func main() {
		if (ab(1,2)!=3){
		fmt.Println("wrong answer in 1st test")
		return
		}
		if (ab(2024,1234)!=3258){
		fmt.Println("wrong answer in 2nd test")
		return
		}
		if (ab(-4,5)!=1){
		fmt.Println("wrong answer in 3rd test")
		return
		}
	    fmt.Println("ok")
	}
	`

	// Запись временного файла с Go кодом
	tmpFileName := "code.go"
	defer os.Remove(tmpFileName)
	err := os.WriteFile(tmpFileName, []byte(goCode), 0644)
	if err != nil {
		fmt.Println("Ошибка при создании временного файла:", err)
		return err, ""
	}

	// Компиляция Go-кода
	cmd := exec.Command("go", "run", tmpFileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Ошибка при выполнении кода:", err)
		return err, "CE"
	}

	// Вывод результата
	return nil, string(output)
}
func main() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tmp.html", nil)
	})

	router.GET("/ws", handleWebSocket)

	router.Run(":8080")
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	mu.Lock()
	clients[conn] = true
	mu.Unlock()

	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.TextMessage {
			err, res := compile(string(msg))
			if err != nil {
				if res == "CE" {
					broadcastMessage("compile error")
					continue
				}
				fmt.Errorf("error in compiling: %s", err)
				continue
			}
			broadcastMessage(res)
		}
	}
}

func broadcastMessage(msg string) {
	mu.Lock()
	defer mu.Unlock()
	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}
