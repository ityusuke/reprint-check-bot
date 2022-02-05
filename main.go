package main

import (
	vision "cloud.google.com/go/vision/apiv1"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"io"
	"log"
	"os"
)

type (
	detectionWebPage struct {
		url       string `protobuf:"bytes,1,opt,name=url,proto3" json:"url,omitempty"`
		pageTitle string `protobuf:"bytes,3,opt,name=page_title,json=pageTitle,proto3" json:"page_title,omitempty"`
	}
)

const (
	lineMessageAPIChannelSecretKey = "CHANNEL_SECRET"
	lineMessageAPIChannelTokenKey  = "CHANNEL_TOKEN"
	lineMessageAPICallBackEndpoint = "/callback"
	port                           = ":3001"
)

func main() {
	engine := gin.Default()
	ctx := context.Background()

	// Open Vision API のクライアントを初期化
	visClient, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// LINE Message API のクライアントを初期化
	lineClient, err := linebot.New(os.Getenv(lineMessageAPIChannelSecretKey), os.Getenv(lineMessageAPIChannelTokenKey))
	if err != nil {
		log.Fatal(err)
	}

	engine.GET(lineMessageAPICallBackEndpoint, func(c *gin.Context) {
		err = exec(lineClient, visClient, ctx, c)
		if err != nil {
			log.Print(err)
		}
	})
	engine.Run(port)
}

func exec(lineClient *linebot.Client, visClient *vision.ImageAnnotatorClient, ctx context.Context, c *gin.Context) error {
	events, err := lineClient.ParseRequest(c.Request)
	if err != nil {
		return err
	}
	file, replyToken, err := extractImageFromLINEMessage(lineClient, events)
	if err != nil {
		return err
	}
	detectionWebPages, err := checkReprint(file, visClient, ctx)
	if err != nil {
		return err
	}
	if err = sendLINEMessageWithMatchWebPages(lineClient, replyToken, detectionWebPages); err != nil {
		return err
	}
}

func sendLINEMessageWithMatchWebPages(lineClient *linebot.Client, replyToken string, detectionWebPages []*detectionWebPage) error {
	if len(detectionWebPages) == 0 {
		if _, err := lineClient.ReplyMessage(replyToken, linebot.NewTextMessage("拾い画ではありません")).Do(); err != nil {
			return err
		}
	} else {
		var message string
		for i, detectionWebPage := range detectionWebPages {
			message += fmt.Sprintf("WEBページ名: %s\n", detectionWebPage.pageTitle)
			message += fmt.Sprintf("URL: %s\n", detectionWebPage.url)
			if i < len(detectionWebPages) {
				message += fmt.Sprintln("\n")
			}
		}
		if _, err := lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(message)).Do(); err != nil {
			return err
		}
	}
	return errors.New("failed to send line message")
}

func extractImageFromLINEMessage(lineClient *linebot.Client, events []*linebot.Event) (*os.File, string, error) {
	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.ImageMessage:
				file, err := os.Create("sample.png")
				if err != nil {
					return nil, "", err
				}
				defer file.Close()

				content, err := lineClient.GetMessageContent(message.ID).Do()
				if err != nil {
					return nil, "", err
				}
				defer content.Content.Close()
				io.Copy(file, content.Content)
				return file, event.ReplyToken, nil
			default:
				if _, err := lineClient.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("画像を送信してください")).Do(); err != nil {
					return nil, "", err
				}
			}
		}
	}
}

func checkReprint(file *os.File, visClient *vision.ImageAnnotatorClient, ctx context.Context) ([]*detectionWebPage, error) {
	detectionWebPages := make([]*detectionWebPage, 0)

	image, err := vision.NewImageFromReader(file)
	if err != nil {
		return detectionWebPages, err
	}

	detection, err := visClient.DetectWeb(ctx, image, nil)
	if err != nil {
		return detectionWebPages, err
	}

	matchImages := detection.GetPagesWithMatchingImages()
	for _, matchImage := range matchImages {
		detectionWebPages = append(detectionWebPages, &detectionWebPage{matchImage.Url, matchImage.PageTitle})
	}

	return detectionWebPages, nil
}
