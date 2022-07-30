package main

import (
	vision "cloud.google.com/go/vision/apiv1"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"io/ioutil"
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
	lineMessageAPIChannelSecretKey = "LINE_BOT_CHANNEL_SECRET"
	lineMessageAPIChannelTokenKey  = "LINE_BOT_CHANNEL_TOKEN"
	OpenVisionAPICredential        = "GOOGLE_APPLICATION_CREDENTIALS"
	lineMessageAPICallBackEndpoint = "/callback"
)

func main() {
	log.Println("application entry")
	log.Println("gen bis cli")
	jwtConfig, err := google.JWTConfigFromJSON([]byte(os.Getenv(OpenVisionAPICredential)), vision.DefaultAuthScopes()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	visClient, err := vision.NewImageAnnotatorClient(ctx, option.WithTokenSource(jwtConfig.TokenSource(ctx)))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("gen line cli")
	lineClient, err := linebot.New(os.Getenv(lineMessageAPIChannelSecretKey), os.Getenv(lineMessageAPIChannelTokenKey))
	if err != nil {
		log.Fatal(err)
	}

	engine := gin.Default()
	engine.GET("/ping", func(c *gin.Context) {
		log.Println("pong")
	})
	engine.POST(lineMessageAPICallBackEndpoint, func(c *gin.Context) {
		err = exec(lineClient, visClient, ctx, c)
		if err != nil {
			log.Println(err)
		}
	})
	engine.Run(":" + os.Getenv("PORT"))
}

func exec(lineClient *linebot.Client, visClient *vision.ImageAnnotatorClient, ctx context.Context, c *gin.Context) error {
	log.Println("start exec")
	events, err := lineClient.ParseRequest(c.Request)
	if err != nil {
		return err
	}
	log.Println("start extractImageFromLINEMessage")
	replyToken, detectionWebPages, err := extractImageFromLINEMessage(lineClient, events, visClient, ctx)
	if err != nil {
		return err
	}
	log.Println("end checkReprint")
	log.Println(detectionWebPages)
	log.Println("start sendLINEMessageWithMatchWebPages")
	if err = sendLINEMessageWithMatchWebPages(lineClient, replyToken, detectionWebPages); err != nil {
		return err
	}
	return nil
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

func extractImageFromLINEMessage(lineClient *linebot.Client, events []*linebot.Event, visClient *vision.ImageAnnotatorClient, ctx context.Context) (string, []*detectionWebPage, error) {
	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.ImageMessage:
				file, err := os.Create("sample.png")
				if err != nil {
					return "", nil, err
				}
				defer file.Close()

				content, err := lineClient.GetMessageContent(message.ID).Do()
				if err != nil {
					return "", nil, err
				}
				defer content.Content.Close()
				byte, err := ioutil.ReadAll(content.Content)
				_, err = file.Write(byte)
				//if err != nil {
				//	return err
				//}
				//_, err = io.Copy(file, byte)
				//if err != nil {
				//	return "", nil, err
				//}
				log.Println(file.Name())
				log.Println(file.Stat())
				log.Println("start NewImageFromReader")
				image, err := vision.NewImageFromReader(file)
				log.Println(err)
				if err != nil {
					log.Println(err)
					return "", nil, err
				}
				log.Println(image)

				detectionWebPages := make([]*detectionWebPage, 0)

				log.Println("start DetectWeb")
				detection, err := visClient.DetectWeb(ctx, file, nil)
				if err != nil {
					log.Println(err)
					return "", nil, err
				}

				defer file.Close()
				log.Println("start GetPagesWithMatchingImages")
				matchImages := detection.GetPagesWithMatchingImages()
				log.Println(matchImages)
				for _, matchImage := range matchImages {
					detectionWebPages = append(detectionWebPages, &detectionWebPage{matchImage.Url, matchImage.PageTitle})
				}
				return event.ReplyToken, detectionWebPages, nil
			default:
				if _, err := lineClient.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("画像を送信してください")).Do(); err != nil {
					return event.ReplyToken, nil, err
				}
			}
		}
	}
	return "", nil, errors.New("failed extract image from line message")
}

//func checkReprint(file *os.File, visClient *vision.ImageAnnotatorClient, ctx context.Context) ([]*detectionWebPage, error) {
//	detectionWebPages := make([]*detectionWebPage, 0)
//
//
//	log.Println("start DetectWeb")
//	detection, err := visClient.DetectWeb(ctx, image, nil)
//	if err != nil {
//		log.Println(err)
//		return detectionWebPages, err
//	}
//
//	log.Println("start GetPagesWithMatchingImages")
//	matchImages := detection.GetPagesWithMatchingImages()
//	log.Println(matchImages)
//	for _, matchImage := range matchImages {
//		detectionWebPages = append(detectionWebPages, &detectionWebPage{matchImage.Url, matchImage.PageTitle})
//	}
//
//	return detectionWebPages, nil
//}
