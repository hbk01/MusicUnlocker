## 音乐解锁 - 电报机器人

### 使用的开源项目：

+ [连接Telegram](https://github.com/go-telegram-bot-api/telegram-bot-api)
+ [解密音乐文件](https://github.com/unlock-music/cli)

### 使用方法

1. 到 [解密音乐文件](https://github.com/unlock-music/cli) 中的 release 页面
   下载你操作系统的可执行文件（若本项目 release 中有 `um.exe` 的话，也可以下载它）
2. 编译本项目（或者下载本项目的 release）
3. **重要** 两个可执行文件放在同目录
4. 执行本程序
    1. **必须** 将你的 API token 使用 -token 选项传递给程序
    2. 要使用 socks5 代理，使用 -proxy 选项
