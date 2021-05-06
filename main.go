package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/proxy"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

var (
	client = &http.Client{}
	socks  = flag.String("proxy", "", "socks5 proxy.")
	token  = flag.String("token", "", "your telegram bot token")
)

func main() {
	// 设置代理并登录 =========================================================
	flag.Parse()
	tgProxyURL, err := url.Parse(*socks)
	if err != nil {
		log.Printf("Failed to parse proxy URL:%s\n", err)
	}
	tgDialer, err := proxy.FromURL(tgProxyURL, proxy.Direct)
	if err != nil {
		log.Printf("Failed to obtain proxy dialer: %s\n", err)
	}
	tgTransport := &http.Transport{
		Dial: tgDialer.Dial,
	}
	client.Transport = tgTransport

	bot, err := tgbotapi.NewBotAPIWithClient(*token, client)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// 登录完成 ============================================================

	bot.Debug = true
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// 接收消息 ===================================================
	updates, err := bot.GetUpdatesChan(u)
	for update := range updates {
		chatId := update.Message.Chat.ID
		// ignore any non-Message Updates
		if update.Message == nil {
			continue
		}

		// if the msg is a file.
		if update.Message.Document != nil {
			doc := update.Message.Document
			sendErrorMsg(bot, chatId, "收到文件", errors.New(doc.FileName))

			if doc.FileSize >= 20*1024*1024 {
				sendMsg(bot, chatId, "文件大于20MB，电报暂不允许机器人接收超过20MB的文件")
				continue
			}

			directURL, err := bot.GetFileDirectURL(doc.FileID)
			if err != nil {
				sendErrorMsg(bot, chatId, "获取下载链接失败", err)
				continue
			}

			file, err := downloadTgFile(directURL, doc.FileName)
			if err != nil {
				sendErrorMsg(bot, chatId, "下载文件失败", err)
				continue
			}
			sendMsg(bot, chatId, "开始解密文件")
			unlockedFile := unlock(*file)
			sendFile(bot, chatId, unlockedFile)
		}

		if update.Message.IsCommand() {
			//log.Printf("[%s] say [%s]\n", update.Message.From.UserName, update.Message.Text)
			switch update.Message.Command() {
			case "start":
				sendMsg(bot, chatId, "发送被加密的文件，稍后会把解密后的文件发回。")
			case "license":
				msg := "本机器人使用了以下开源项目：\n" +
					"*MIT License* \\- [连接Telegram](https://github.com/go-telegram-bot-api/telegram-bot-api)\n" +
					"*MIT License* \\- [解密音乐文件](https://github.com/unlock-music/cli)\n" +
					"\n" +
					"非常感谢以上的开源项目！"
				sendMsg(bot, chatId, msg)
			case "about":
				msg := "本机器人项目开源在 [github](https://github.com/hbk01/MusicUnlocker)"
				sendMsg(bot, chatId, msg)
			default:
				sendMsg(bot, chatId, "这个命令我不认识，你是不是在玩我？")
			}
			continue
		}

		if update.Message.Text != "" {
			sendMsg(bot, chatId, "请不要调戏本机器人，服务器很贵的好不啦！")
			continue
		}
	}
}

func sendMsg(api *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true
	_, err := api.Send(msg)
	if err != nil {
		log.Printf("Send message failed. %s", err.Error())
	}
}

func sendErrorMsg(api *tgbotapi.BotAPI, chatID int64, text string, err error) {
	msg := tgbotapi.NewMessage(chatID, text+" "+err.Error())
	msg.DisableWebPagePreview = true
	_, err = api.Send(msg)
	if err != nil {
		log.Printf("Send error message failed. %s", err.Error())
	}
}

func sendFile(api *tgbotapi.BotAPI, chatID int64, file string) {
	msg := tgbotapi.NewAudioUpload(chatID, file)
	_, err := api.Send(msg)
	if err != nil {
		log.Printf("Send file failed. %s", err.Error())
	}
}

/*
unlock the file.

i is input filename, return unlocked filename.
*/
func unlock(i os.File) string {
	cmd := exec.Command("./um.exe", "-i", i.Name(), "-o", "unlocked")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("%s\n", err.Error())
	}
	output := fmt.Sprintf("%s", out)
	temp := strings.Split(output, "\"")
	//
	//for a := range temp {
	//	fmt.Println(a, " -- ", temp[a])
	//}
	unlockedFileName := temp[len(temp)-2]
	//open, err := os.Open(unlockedFileName)
	//if err != nil {
	//	log.Printf("Open unlocked file %s failed. %s\n", unlockedFileName, err.Error())
	//}
	return unlockedFileName
}

/*
Download Telegram file, save in 'locked' folder.
*/
func downloadTgFile(url, filename string) (*os.File, error) {
	// 预先创建 locked 目录
	_, err := os.Stat("locked/")
	if err != nil {
		err = os.Mkdir("locked/", 0666)
		if err != nil {
			log.Printf("Create 'locked' folder failed: %s", err.Error())
		}
	} else {
		filename = "locked/" + filename
	}

	log.Printf("Download %s from %s\n", filename, url)
	// Create blank file
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	// Put content on file
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Connect failed: %s", err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	size, err := io.Copy(file, resp.Body)
	defer file.Close()
	fmt.Printf("Downloaded a file %s with size %d\n", filename, size)
	return file, nil
}
