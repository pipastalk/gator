package main

import (
	"fmt"
	"os"

	"github.com/pipastalk/gator/internal/config"
)

func main() {
	fmt.Println("Welcome to Gator REPL!")
	if len(os.Args) < 2 {
		fmt.Println("No command provided. Usage: gator <command> [args]")
		os.Exit(1)
	}
	conf, err := config.Read()
	if err != nil {
		fmt.Println("Error reading config:", err)
		os.Exit(1)
	}
	userState := state{
		config: conf,
	}
	cmds := commands{
		commands: make(map[string]func(*state, command) error),
	}
	cmds.register("login", handlerLogin)
	cmd := command{
		Name: os.Args[1],
		Args: os.Args[2:],
	}
	if err := cmds.run(&userState, cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("state username is %+v\n", userState.config.CurrentUserName)
}

type state struct {
	config *config.Config
}
type command struct {
	Name string
	Args []string
}
type commands struct {
	commands map[string]func(*state, command) error
}

func handlerLogin(s *state, cmd command) error {
	username := ""
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Incorrect usage of login command. Usage: login <username>")
	}
	username = cmd.Args[0]
	if username == "" {
		return fmt.Errorf("username not provided")
	}
	if err := s.config.SetUsername(username); err != nil {
		return fmt.Errorf("Login Error: %w", err)
	}
	fmt.Printf("Username set to %v\n", s.config.CurrentUserName)
	return nil

}

func (c *commands) run(s *state, cmd command) error {
	command, ok := c.commands[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown command: %v", cmd.Name)
	}
	if err := command(s, cmd); err != nil {
		return fmt.Errorf("Error executing command %v: %w", cmd.Name, err)
	}
	return nil
}

func (c *commands) register(name string, handler func(*state, command) error) {
	c.commands[name] = handler
}
