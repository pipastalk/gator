package main

import (
	"fmt"

	"github.com/pipastalk/gator/internal/config"
)

func main() {
	username := "testuser"
	if err := config.SetUsername(username); err != nil {
		fmt.Println("Error setting config:", err)
		return
	}
	saved_cfg, err := config.Read()
	if err != nil {
		fmt.Println("Error reading saved config:", err)
		return
	}
	fmt.Printf("Saved config: %+v\n", *saved_cfg)
}
