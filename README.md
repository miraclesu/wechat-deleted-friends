# wechat-deleted-friends

查看被删的微信好友

感谢 [0x5e](https://github.com/0x5e) 提供的 [Python版本](https://github.com/0x5e/wechat-deleted-friends)

## 使用

在 [Release](https://github.com/miraclesu/wechat-deleted-friends/releases) 里下载对应平台的最新版本，在电脑上运行后按指示做即可

1. 程序运行的时候最好不要动微信，有朋友反馈说可能会造成程序运行失败
2. 打开的二维码目前需要自己关闭
2. 最终会遗留下一个只有自己的群组,需要手工删一下

如果本地有 Go 环境：

1. `go get github.com/skratchdot/open-golang/open`
2. clone 代码下来后在目录里执行 `go build -o wdf *.go`
3. 运行 `./wdf` 按照提示走即可
4. 参数设置请查看帮助 `./wdf -h`
