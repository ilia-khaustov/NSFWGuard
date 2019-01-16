package tgbot

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/parnurzeal/gorequest"
	"gopkg.in/telegram-bot-api.v4"
)

const helpText string = `*TIRED OF NFSW PICS IN YOUR CHAT ? CONGRATS COMRADE ! YOU FOUND THE RIGHT BOT*

I am a sleepless guard powered by [open-source image classification API](https://github.com/rahiel/open_nsfw--).

Main algorithm is simple:
1. Check every pic users send to any chat I've beed added to;
2. If I'm not sure for 95% that pic is NSFW I just let it go;
3. Else, I delete _the pic_ and send an _URL to the pic_ instead.

Also, I can rate pics for you. Reply to a pic with /rate command and see what I think about it.

Configure per-chat rating threshold that _triggers_ me with a /threshold command supplied by argument, i.e. ` + "`/threshold 80`" + `
`

type checkTask struct {
	chatID   int64
	msgID    int
	author   *tgbotapi.User
	fileID   string
	fileURL  string
	nsfwRate float64
}

type rateTask struct {
	msg      *tgbotapi.Message
	user     *tgbotapi.User
	fileID   string
	nsfwRate float64
}

// BotCtrl provides API for the NSFWGuard Telegram bot
type BotCtrl struct {
	bot         *tgbotapi.BotAPI
	nsfwAPIAddr string
	nsfwAPIPrec float64
	stop        chan interface{}
	check       chan *checkTask
	rate        chan *rateTask
	rates       map[string]float64
	urls        map[string]string
	thresholds  map[int64]float64
}

// NewBotCtrl creates a new BotCtrl
func NewBotCtrl() (*BotCtrl, error) {
	token, isGiven := os.LookupEnv("TLGRM_TOKEN")
	if !isGiven {
		return nil, fmt.Errorf("TLGRM_TOKEN not set in environment")
	}

	nsfwAPIAddr, isGiven := os.LookupEnv("NSFW_API_ADDR")
	if !isGiven {
		return nil, fmt.Errorf("NSFW_API_ADDR not set in environment")
	}

	nsfwAPIPrecStr, isGiven := os.LookupEnv("NSFW_API_PREC")
	if !isGiven {
		return nil, fmt.Errorf("NSFW_API_PREC not set in environment")
	}

	nsfwAPIPrec, err := strconv.ParseFloat(nsfwAPIPrecStr, 64)
	if err != nil {
		return nil, fmt.Errorf("NSFW_API_PREC is not a valid float")
	}

	debugFlag := flag.Bool("bot-debug", false, "Debug mode (bot)")
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = *debugFlag
	ctrl := &BotCtrl{
		bot:         bot,
		nsfwAPIAddr: nsfwAPIAddr,
		nsfwAPIPrec: nsfwAPIPrec,
		rates:       make(map[string]float64),
		urls:        make(map[string]string),
		thresholds:  make(map[int64]float64),
		check:       make(chan *checkTask, 10),
		rate:        make(chan *rateTask, 10),
		stop:        make(chan interface{}),
	}

	go ctrl.readUpdates(tgbotapi.NewUpdate(0))

	return ctrl, nil
}

func (ctrl *BotCtrl) readUpdates(conf tgbotapi.UpdateConfig) {
	_, err := ctrl.bot.RemoveWebhook()
	if err != nil {
		log.Fatal(err)
	}

	upds, err := ctrl.bot.GetUpdatesChan(conf)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case upd := <-upds:
			switch {
			case upd.Message != nil && upd.Message.IsCommand():
				switch upd.Message.Command() {
				case "help":
					go ctrl.doReplyText(upd.Message, helpText)
				case "threshold":
					if upd.Message.CommandArguments() == "" {
						go ctrl.doReplyText(upd.Message, "No argument given for threshold; need /help?")
					} else {
						threshold, err := strconv.ParseFloat(upd.Message.CommandArguments(), 64)
						if err != nil || threshold > 100 {
							go ctrl.doReplyText(upd.Message, "Given argument is invalid, use numbers below 100")
						} else {
							ctrl.thresholds[upd.Message.Chat.ID] = threshold / 100.
							go ctrl.doReplyText(upd.Message, fmt.Sprintf("Set threshold to *%.0f%%*", threshold))
						}
					}
				case "rate":
					if upd.Message.ReplyToMessage == nil {
						go ctrl.doReplyText(upd.Message, "No message found in a reply; need /help?")
					} else if upd.Message.ReplyToMessage.Photo == nil {
						go ctrl.doReplyText(upd.Message, "I can't see any photos in a reply.")
					} else {
						biggest := getBiggestPhotoSize(upd.Message.ReplyToMessage)
						if biggest == nil {
							continue
						}
						rate, hasRate := ctrl.rates[biggest.FileID]
						if !hasRate {
							log.Printf("Got unrated from %s", upd.Message.From.String())
							go ctrl.doRate(upd.Message, biggest.FileID)
						} else {
							ctrl.rate <- &rateTask{
								msg:      upd.Message,
								fileID:   biggest.FileID,
								nsfwRate: rate,
							}
						}
					}
				}
			case upd.Message != nil && upd.Message.Photo != nil:
				biggest := getBiggestPhotoSize(upd.Message)
				if biggest == nil {
					continue
				}
				rate, hasRate := ctrl.rates[biggest.FileID]
				url, hasURL := ctrl.urls[biggest.FileID]
				thrs, hasThrs := ctrl.thresholds[upd.Message.Chat.ID]
				if !hasThrs {
					thrs = ctrl.nsfwAPIPrec
				}
				if !hasRate || !hasURL {
					log.Printf("Got unchecked from %s", upd.Message.From.String())
					go ctrl.doCheck(upd.Message, biggest.FileID)
				} else if rate >= thrs {
					log.Printf("Got NSFW from %s", upd.Message.From.String())
					ctrl.check <- &checkTask{
						chatID:   upd.Message.Chat.ID,
						msgID:    upd.Message.MessageID,
						author:   upd.Message.From,
						fileID:   biggest.FileID,
						fileURL:  url,
						nsfwRate: rate,
					}
				}
			}
		case task := <-ctrl.check:
			log.Printf("Checked %+v", task)
			ctrl.rates[task.fileID] = task.nsfwRate
			thrs, hasThrs := ctrl.thresholds[task.chatID]
			if !hasThrs {
				thrs = ctrl.nsfwAPIPrec
			}
			if task.nsfwRate < thrs {
				continue
			}
			ctrl.urls[task.fileID] = task.fileURL
			go ctrl.doDeleteMsg(task.chatID, task.msgID)
			go ctrl.doSendURL(task.chatID, task.author, task.fileURL)
		case task := <-ctrl.rate:
			log.Printf("Rated %+v", task)
			ctrl.rates[task.fileID] = task.nsfwRate
			go ctrl.doReplyText(task.msg, fmt.Sprintf("My NSFW rating is *%.2f%%*", task.nsfwRate*100.))
		case <-ctrl.stop:
			return
		}
	}
}

func getBiggestPhotoSize(msg *tgbotapi.Message) *tgbotapi.PhotoSize {
	photos := *msg.Photo
	maxSize := 0
	var biggest *tgbotapi.PhotoSize
	for _, photo := range photos {
		if photo.Height*photo.Width >= maxSize {
			maxSize = photo.Height * photo.Width
			biggest = &photo
		}
	}
	return biggest
}

func (ctrl *BotCtrl) doCheck(msg *tgbotapi.Message, fileID string) {
	fileURL, err := ctrl.bot.GetFileDirectURL(fileID)
	if err != nil {
		log.Println(fmt.Errorf("could not get file direct URL: %v", err))
		return
	}
	_, body, errs := gorequest.New().Post(ctrl.nsfwAPIAddr).Send(fmt.Sprintf("url=%s", fileURL)).End()
	if errs != nil {
		log.Println(fmt.Errorf("request to NSFW API failed: %v", errs))
		return
	}
	rate, err := strconv.ParseFloat(body, 64)
	if err != nil {
		log.Println(fmt.Errorf("response from NSFW API is not a valid float: %v when parsing `%s`", err, body))
		return
	}
	ctrl.check <- &checkTask{
		chatID:   msg.Chat.ID,
		msgID:    msg.MessageID,
		author:   msg.From,
		fileID:   fileID,
		fileURL:  fileURL,
		nsfwRate: rate,
	}
}

func (ctrl *BotCtrl) doRate(msg *tgbotapi.Message, fileID string) {
	fileURL, err := ctrl.bot.GetFileDirectURL(fileID)
	if err != nil {
		log.Println(fmt.Errorf("could not get file direct URL: %v", err))
		return
	}
	_, body, errs := gorequest.New().Post(ctrl.nsfwAPIAddr).Send(fmt.Sprintf("url=%s", fileURL)).End()
	if errs != nil {
		log.Println(fmt.Errorf("request to NSFW API failed: %v", errs))
		return
	}
	rate, err := strconv.ParseFloat(body, 64)
	if err != nil {
		log.Println(fmt.Errorf("response from NSFW API is not a valid float: %v when parsing `%s`", err, body))
		return
	}
	ctrl.rate <- &rateTask{
		msg:      msg,
		fileID:   fileID,
		nsfwRate: rate,
	}
}

func (ctrl *BotCtrl) doDeleteMsg(chatID int64, msgID int) {
	_, err := ctrl.bot.DeleteMessage(tgbotapi.DeleteMessageConfig{ChatID: chatID, MessageID: msgID})
	if err != nil {
		log.Println(fmt.Errorf("could not delete message: %v", err))
	}
}

func (ctrl *BotCtrl) doSendURL(chatID int64, author *tgbotapi.User, url string) {
	_, err := ctrl.bot.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: chatID,
		},
		DisableWebPagePreview: true,
		ParseMode:             tgbotapi.ModeHTML,
		Text:                  fmt.Sprintf("Naughty <b>%s</b> sent us a <a href='%s'>NSFW pic</a>, oh boi", author.String(), url),
	})
	if err != nil {
		log.Println(fmt.Errorf("could not send message: %v", err))
	}
}

func (ctrl *BotCtrl) doReplyText(msg *tgbotapi.Message, text string) {
	_, err := ctrl.bot.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:           msg.Chat.ID,
			ReplyToMessageID: msg.MessageID,
		},
		DisableWebPagePreview: true,
		ParseMode:             tgbotapi.ModeMarkdown,
		Text:                  text,
	})

	if err != nil {
		log.Println(fmt.Errorf("could not send message: %v", err))
	}
}

// Stop stops bot
func (ctrl *BotCtrl) Stop() {
	close(ctrl.stop)
}
