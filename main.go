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
	"strconv"
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
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerListFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollowFeed))
	cmds.register("following", middlewareLoggedIn(handlerFollowingFeeds))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollowFeed))
	cmds.register("browse", handlerBrowse)
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

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if user.Name == "" {
		return fmt.Errorf("No user logged in. Please login first.")
	}
	if len(cmd.Args) != 2 {
		return fmt.Errorf("Incorrect usage of addfeed command. Usage: addfeed <feed_url>, got: %+v", cmd.Args)
	}

	feedName := cmd.Args[0]
	_, err := url.Parse(cmd.Args[1])
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

func handlerFollowFeed(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Incorrect usage of follow command. Usage: follow <feed_url>, got: %+v", cmd.Args)
	}
	feedUrl := cmd.Args[0]
	feed, err := s.db.GetFeed(context.Background(), feedUrl)
	if err != nil {
		if err == sql.ErrNoRows {
			handlerAddFeed(s, command{Name: "addfeed", Args: []string{feedUrl, feedUrl}}, user)
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

func handlerFollowingFeeds(s *state, cmd command, user database.User) error {
	if user.Name == "" {
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

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
		if err != nil {
			return fmt.Errorf("Error fetching current user: %w", err)
		} else if user.Name == "" {
			return fmt.Errorf("No user logged in. Please login first.")
		}
		return handler(s, cmd, user)
	}
}

func handlerUnfollowFeed(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Incorrect usage of unfollow command. Usage: unfollow <feed_url>, got: %+v", cmd.Args)
	}
	feedUrl := cmd.Args[0]
	feed, err := s.db.GetFeed(context.Background(), feedUrl)
	if err != nil {
		return fmt.Errorf("Error fetching feed: %w", err)
	}
	err = s.db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Error unfollowing feed: %w", err)
	}
	fmt.Printf("%s has unfollowed '%s'!\n", user.Name, feed.Name)
	return nil
}

func scrapeFeeds(s *state) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("Error fetching next feed to scrape: %w", err)
	}
	data, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		return fmt.Errorf("Error fetching feed data: %w", err)
	}
	fmt.Printf("Fetched feed '%s' with %d items\n", feed.Name, len(data.Channel.Item))
	_, err = s.db.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		return fmt.Errorf("Error marking feed as fetched: %w", err)
	}
	preExistingPostCount := 0
	totalPostCount := 0
	for i, item := range data.Channel.Item {
		postParam, preExisting := prepPost(s, item, feed)
		if preExisting {
			preExistingPostCount++
			continue
		}
		totalPostCount++
		_, err := s.db.CreatePost(context.Background(), postParam)
		if err != nil {
			fmt.Printf("Error creating post: %v\n", err)
		}
		fmt.Printf("Processed post %d: %s\n", i+1, item.Title)
	}
	fmt.Printf("Finished scraping feed '%s'. Total new posts: %d, pre-existing posts: %d\n", feed.Name, totalPostCount, preExistingPostCount)
	return nil
}

func prepPost(s *state, item RSSItem, feed database.Feed) (postParams database.CreatePostParams, preExisting bool) {
	existingPost, _ := s.db.GetPost(context.Background(), item.Link)
	if item.Link == existingPost.Url {
		postParams = database.CreatePostParams{
			Title:       existingPost.Title,
			Url:         existingPost.Url,
			Description: existingPost.Description,
			PublishedAt: existingPost.PublishedAt,
			FeedID:      existingPost.FeedID,
		}
		return postParams, true
	}
	title := sql.NullString{
		String: item.Title,
		Valid:  item.Title != "",
	}
	description := sql.NullString{
		String: item.Description,
		Valid:  item.Description != "",
	}
	publishAt_t, _ := time.Parse(time.RFC1123Z, item.PubDate)
	publishAt := sql.NullTime{
		Time:  publishAt_t,
		Valid: publishAt_t != time.Time{},
	}
	postParams = database.CreatePostParams{
		Title:       title,
		Url:         item.Link,
		Description: description,
		PublishedAt: publishAt,
		FeedID:      feed.ID,
	}
	return postParams, false
}

func handlerAgg(s *state, cmd command) error {
	var time_between_reqs string
	if cmd.Args[0] != "" {
		time_between_reqs = cmd.Args[0]
	}
	timer, err := time.ParseDuration(time_between_reqs)
	minimumTimer := 5 * time.Second
	if timer <= minimumTimer {
		fmt.Printf("Duration too short: %v. Setting to minimum is %v.\n", timer, minimumTimer)
		timer = minimumTimer
	}
	if err != nil {
		fmt.Printf("Invalid duration format: %v\n", err)
		return fmt.Errorf("Invalid duration format: %w", err)
	}
	ticker := time.NewTicker(timer)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		if err := scrapeFeeds(s); err != nil {
			fmt.Printf("Error scraping feeds: %v\n", err)
		}
	}
}

func handlerBrowse(s *state, cmd command) error {
	limit := "2"
	if len(cmd.Args) == 1 {
		limit = cmd.Args[0]
	}
	limitInt, err := strconv.ParseInt(limit, 10, 32)
	if err != nil {
		return fmt.Errorf("Invalid limit format: %w", err)
	}
	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Error fetching user: %w", err)
	}

	posts, err := s.db.GetUsersFeedPosts(context.Background(), database.GetUsersFeedPostsParams{
		UserID: user.ID,
		Limit:  int32(limitInt),
	})
	if err != nil {
		return fmt.Errorf("Error fetching posts: %w", err)
	}
	if len(posts) == 0 {
		fmt.Println("No posts found in your feed. Try following some feeds first!")
		return nil
	}
	for _, post := range posts {
		fmt.Printf("Title: %s\n", post.Title.String)
		fmt.Printf("Description: %s\n", post.Description.String)
		fmt.Printf("URL: %s\n", post.Url)
		fmt.Printf("Published At: %v\n", post.PublishedAt.Time)
		fmt.Printf("Feed ID: %v\n", post.FeedID)
		fmt.Println("-----")
	}
	return nil

}
