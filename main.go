package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"

	_ "github.com/lib/pq"

	"github.com/pipastalk/gator/internal/config"
	"github.com/pipastalk/gator/internal/database"
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
	//region commands registration
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegisterUser)
	cmds.register("reset", handlerResetUsers)
	cmds.register("users", handlerListUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", handlerAddFeed)
	cmds.register("feeds", handlerListFeeds)
	cmds.register("follow", handlerFollowFeed)
	cmds.register("following", handlerFollowingFeeds)
	//endregion
	cmd := command{
		Name: os.Args[1],
		Args: os.Args[2:],
	}

	db, err := sql.Open("postgres", userState.config.DB_URL)
	userState.db = database.New(db)
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("Successfully connected to the database!")

	if err := cmds.run(&userState, cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type state struct {
	config *config.Config
	db     *database.Queries
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
	user, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user %v not found. Please register first.", username)
		}
		return fmt.Errorf("Error fetching user from database: %w", err)
	}
	if err := s.config.SetUsername(user.Name); err != nil {
		return fmt.Errorf("Login Error: %w", err)
	}
	fmt.Printf("Username set to %v\n", s.config.CurrentUserName)
	return nil

}

func handlerRegisterUser(s *state, cmd command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Incorrect usage of register command. Usage: register <username>")
	}
	var username string
	username = cmd.Args[0]
	if username == "" {
		return fmt.Errorf("username not provided")
	}
	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		Name:      username,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		fmt.Printf("Error registering user: %v\n", err)
		os.Exit(1)
	}
	if err := s.config.SetUsername(user.Name); err != nil {
		return fmt.Errorf("Error setting username after registration: %w", err)
	}
	fmt.Printf("User %v registered successfully\n", username)
	handlerLogin(s, command{Name: "login", Args: []string{username}})
	return nil

}

func handlerResetUsers(s *state, cmd command) error {
	if err := s.db.Reset(context.Background()); err != nil {
		return fmt.Errorf("Error resetting users: %w", err)
	}
	fmt.Println("All users have been reset successfully.")
	return nil
}

func handlerListUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Error fetching users: %w", err)
	}
	if len(users) == 0 {
		fmt.Println("No users found.")
		return nil
	}
	fmt.Println("Registered Users:")
	for _, user := range users {
		is_current := ""
		if user.Name == s.config.CurrentUserName {
			is_current = " (current)"
		}
		fmt.Printf("* %s%s\n", user.Name, is_current)
	}
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

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	_, err := url.ParseRequestURI(feedURL)
	if err != nil {
		return nil, fmt.Errorf("Invalid URL: %v", feedURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Gator")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML: %w", err)
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}
	return &feed, nil
}

func handlerAgg(s *state, cmd command) error {
	/*
		if len(cmd.Args) != 1 {
			return fmt.Errorf("Incorrect usage of agg command. Usage: agg <feed_url>")
		}
		feedURL := cmd.Args[0]
	*/
	feedURL := "https://www.wagslane.dev/index.xml"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	feed, err := fetchFeed(ctx, feedURL)
	if err != nil {
		return fmt.Errorf("Error fetching feed: %w", err)
	}
	fmt.Printf("Feed Title: %s\n", feed.Channel.Title)
	fmt.Printf("Feed Description: %s\n", feed.Channel.Description)
	fmt.Println("Items:")
	for _, item := range feed.Channel.Item {
		fmt.Printf("- %s (%s)\n  %s\n  Published on: %s\n", item.Title, item.Link, item.Description, item.PubDate)
	}
	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Error fetching current user: %w", err)
	} else if user.Name == "" {
		return fmt.Errorf("No user logged in. Please login first.")
	}
	if len(cmd.Args) != 2 {
		return fmt.Errorf("Incorrect usage of addfeed command. Usage: addfeed <feed_url>, got: %+v", cmd.Args)
	}

	feedName := cmd.Args[0]
	_, err = url.Parse(cmd.Args[1])
	feedURL := cmd.Args[1]
	if err != nil {
		return fmt.Errorf("Invalid URL: %v", cmd.Args[1])
	}
	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		Name:   feedName,
		Url:    feedURL,
		UserID: user.ID,
	})
	if err != nil {
		return fmt.Errorf("Error adding feed: %w", err)
	}
	fmt.Printf("Feed '%s' added successfully!\n", feed.Name)
	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Error following feed after adding: %w", err)
	}
	fmt.Printf("%s are now following '%s'!\n", user.Name, feed.Name)

	return nil
}

func handlerListFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Error fetching feeds: %w", err)
	}
	if len(feeds) == 0 {
		fmt.Println("No feeds found.")
		return nil
	}
	fmt.Println("Registered Feeds:")
	for _, feed := range feeds {
		fmt.Printf("* %s (%s) - added by %s\n", feed.Name, feed.Url, feed.Username)
	}
	return nil
}

func handlerFollowFeed(s *state, cmd command) error {
	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Error fetching current user: %w", err)
	} else if user.Name == "" {
		return fmt.Errorf("No user logged in. Please login first.")
	}
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Incorrect usage of follow command. Usage: follow <feed_url>, got: %+v", cmd.Args)
	}
	feedUrl := cmd.Args[0]
	feed, err := s.db.GetFeed(context.Background(), feedUrl)
	if err != nil {
		if err == sql.ErrNoRows {
			handlerAddFeed(s, command{Name: "addfeed", Args: []string{feedUrl, feedUrl}})
			err = nil
			feed, err = s.db.GetFeed(context.Background(), feedUrl)
			if err != nil {
				return fmt.Errorf("Error fetching feed after adding: %w", err)
			}
		}
	}
	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Error following feed: %w", err)
	}
	fmt.Printf("%s are now following '%s'!\n", user.Name, feed.Name)
	return nil
}

func handlerFollowingFeeds(s *state, cmd command) error {
	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Error fetching current user: %w", err)
	} else if user.Name == "" {
		return fmt.Errorf("No user logged in. Please login first.")
	}
	feeds, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("Error fetching followed feeds: %w", err)
	}
	if len(feeds) == 0 {
		fmt.Println("You are not following any feeds.")
		return nil
	}
	fmt.Printf("Feeds %s are following:\n", user.Name)
	for _, feed := range feeds {
		fmt.Printf("* %s (%s)\n", feed.FeedName, feed.FeedUrl)
	}
	return nil
}
