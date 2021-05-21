package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/proxy"
)

var (
	client = &http.Client{}
	socks  = flag.String("proxy", "socks5://localhost:8888", "set proxy")
	token  = flag.String("token", "", "your telegram bot token")
	debug  = flag.Bool("debug", false, "enable debug output")
	dbname = flag.String("dbname", "root", "mysql database username")
	dbpswd = flag.String("dbpswd", "123456", "mysql database password")
	dbhost = flag.String("dbhost", "localhost:3306", "mysql database address")
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
	log.Printf("Login as: %s\n", bot.Self.UserName)

	dataSourceName := *dbname + ":" + *dbpswd + "@tcp(" + *dbhost + ")/music_unlock?parseTime=true"
	log.Println("Database source: " + dataSourceName)
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		log.Println("连接数据库失败：" + err.Error())
	}
	defer db.Close()

	// 登录完成 ============================================================

	bot.Debug = *debug
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// 接收消息 ===================================================
	updates, _ := bot.GetUpdatesChan(u)
	for update := range updates {
		chatId := update.Message.Chat.ID
		// ignore any non-Message Updates
		if update.Message == nil {
			continue
		}

		// if the msg is a file.
		if update.Message.Document != nil {
			doc := update.Message.Document
			log.Printf("Recv document: %s\n", doc.FileName)
			sendErrorMsg(bot, chatId, "收到文件", errors.New(doc.FileName))

			// TODO check the file name(only the encypted music file can to next step.)

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

			// TODO save information into databases
			err = db.Ping()
			if err == nil {
				username := update.Message.Chat.UserName
				displayName := update.Message.Chat.FirstName + update.Message.Chat.LastName
				unlockFileName := doc.FileName
				unlockFileSize := doc.FileSize
				time := update.Message.Time().Format("2006-01-02 15:04:05")

				stmt, err := db.Prepare("insert into logs(username, display_name, time, unlock_file_name, unlock_file_size) values (?, ?, ?, ?, ?);")
				if err != nil {
					log.Println(err.Error())
				}
				if stmt == nil {
					panic("stmt is nil")
				}
				_, err = stmt.Exec(username, displayName, time, unlockFileName, unlockFileSize)
				if err != nil {
					log.Println(err.Error())
				}
				_ = stmt.Close()
			} else {
				log.Println(err.Error())
			}

		}

		// if the msg is a command.
		if update.Message.IsCommand() {
			log.Printf("Recv command: %s\n", update.Message.Command())
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
				msg := "本机器人项目开源在 [Github](https://github.com/hbk01/MusicUnlocker)"
				sendMsg(bot, chatId, msg)
			default:
				sendMsg(bot, chatId, "这个命令我不认识，你是不是在玩我？")
			}
			continue
		}

		// if the msg is a text.
		if update.Message.Text != "" {
			log.Printf("Recv msg: %s\n", update.Message.Text)
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
	log.Printf("Send msg to %d: %s\n", msg.ChatID, msg.Text)
	if err != nil {
		log.Printf("Send message failed. %s", err.Error())
	}
}

func sendErrorMsg(api *tgbotapi.BotAPI, chatID int64, text string, err error) {
	msg := tgbotapi.NewMessage(chatID, text+" "+err.Error())
	msg.DisableWebPagePreview = true
	_, err = api.Send(msg)
	log.Printf("Send error msg to %d: %s\n", msg.ChatID, msg.Text)
	if err != nil {
		log.Printf("Send error message failed. %s", err.Error())
	}
}

func sendFile(api *tgbotapi.BotAPI, chatID int64, file string) {
	msg := tgbotapi.NewAudioUpload(chatID, file)
	_, err := api.Send(msg)
	log.Printf("Send file to %d: %s\n", msg.ChatID, file)
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
	unlockedFileName := temp[len(temp)-2]
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
	}

	filename = "locked/" + filename

	log.Printf("Download file: %s\n", filename)
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
	size, _ := io.Copy(file, resp.Body)
	defer file.Close()
	log.Printf("Downloaded: %s, Size: %s\n", filename, FormatSize(size))
	return file, nil
}

func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB",
		float64(size)/float64(div), "KMGTPE"[exp])
}
