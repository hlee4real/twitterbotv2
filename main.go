package main

import (
	"context"
	"fmt"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	tb "gopkg.in/telebot.v3"
)

type Client struct {
	Token     string
	ChatId    tb.ChatID
	botClient *tb.Bot
}

var collection *mongo.Collection

func init() {
}

func (c *Client) startBotCommands() {
	go c.crontab()
	c.botClient.Handle("/help", func(ctx tb.Context) error {
		c.helpCommands()
		return nil
	})
	c.botClient.Handle("/usage", func(ctx tb.Context) error {
		c.botClient.Send(c.ChatId, "First of all, bot will get 20 newest tweets of user. Then, Bot will notify you when your favorite twitter user post new tweet, it can be delay 2-5 minutes after the tweet is posted because of APIs policies.\nIf you want to track new twitter account, use /add username. For example, /add hoangdeptrai")
		return nil
	})
	c.botClient.Handle("/start", func(ctx tb.Context) error {
		c.botClient.Send(c.ChatId, "Welcome to twitter tracker bot, to know how to use this bot, please type /help")
		return nil
	})
	c.botClient.Handle("/add", func(ctx tb.Context) error {
		usernames := ctx.Args()
		for _, username := range usernames {
			c.addTwitterUsername(username)
		}
		return nil
	})
	fmt.Println("start listen command")
	c.botClient.Start()
}
func (c *Client) addTwitterUsername(username string) {
	go trackTweet(username)
}
func (c *Client) helpMessages() string {
	return "/add - add twitter username to track.\n/help - show help commands.\n/usage - show how to use this bot."
}
func (c *Client) helpCommands() {
	c.botClient.Send(c.ChatId, c.helpMessages())
}
func trackTweet(username string) {
	// tweetClient
	// get update tweet
	// insert DB
	collection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "username", Value: 1},
			{Key: "URLs", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	tweetURLs := scraper(username)
	for _, tweetURL := range tweetURLs {
		collection.InsertOne(context.Background(), bson.D{
			{Key: "username", Value: username},
			{Key: "URLs", Value: tweetURL},
			{Key: "isSent", Value: false},
		})
	}
}
func (c *Client) scheduleTrackingTweet() {
	//get all username from database
	mongoclient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println(err)
		return
	}
	collection = mongoclient.Database("tracking").Collection("tweets")
	result := make([]string, 0)
	cursor, err := collection.Distinct(context.Background(), "username", bson.D{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, username := range cursor {
		result = append(result, username.(string))
	}
	for _, username := range result {
		go trackTweet(username)
	}
}
func (c *Client) scheduleSendMessage() {
	// get all tweets from database/config where isSend = false
	mongoclient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println(err)
		return
	}
	collection = mongoclient.Database("tracking").Collection("tweets")
	result := make([]string, 0)
	cursor, err := collection.Find(context.Background(), bson.D{
		{Key: "isSent", Value: false},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	for cursor.Next(context.Background()) {
		var tweet bson.M
		cursor.Decode(&tweet)
		result = append(result, tweet["URLs"].(string))
	}
	for _, url := range result {
		c.sendMessage(url)
		//update isSend = true
		collection.UpdateOne(context.Background(), bson.D{
			{Key: "URLs", Value: url},
		}, bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "isSent", Value: true},
			}},
		})
	}
}
func (c *Client) sendMessage(content string) {
	c.botClient.Send(c.ChatId, content)
}
func (c *Client) crontab() {
	schedule := cron.New()
	//every 10 second
	schedule.AddFunc("@every 4m", c.scheduleTrackingTweet)
	schedule.AddFunc("@every 1m", c.scheduleSendMessage)
	schedule.Start()
}
func scraper(username string) []string {
	scraper := twitterscraper.New()
	var tweetURLs []string
	for tweet := range scraper.GetTweets(context.Background(), username, 20) {
		if tweet.Error != nil {
			fmt.Println(tweet.Error)
			continue
		}
		tweetURLs = append(tweetURLs, tweet.PermanentURL)
	}
	return tweetURLs
}
func main() {
	client := Client{
		Token:  "5734397761:AAFTX1ufJawwW2ERFN5YOe3n3OvFoVntTVg",
		ChatId: 1262995839,
	}
	client.botClient, _ = tb.NewBot(tb.Settings{
		Token:  client.Token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	client.startBotCommands()
}
