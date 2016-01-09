package main

import (
	"flag"
	"log"
	"os"
	"strings"
)

var (
	Debug    = flag.Bool("debug", false, "是否为 debug 模式")
	GroupNum = flag.Int("num", 34, "群最大人数，不要顺便设置此参数，除非群机制变了")
	Duration = flag.Int("d", 16, `接口调用时间间隔，单位/s, 值设为 13 时亲测出现"操作太频繁"`)
	Progress = flag.Int("p", 50, "进度条长度")
	Retry    = flag.Int("r", 3, "出错重试次数")
	DeviceId = flag.String("did", "e000000000000000", "device id")

	OnceFriends []string
)

func main() {
	flag.Parse()
	log.Println("本程序的查询结果可能会引起一些心理上的不适，请做好心理准备...")

	uuid, err := getUUID()
	if err != nil {
		log.Printf("获取 uuid 失败: %s\n", err.Error())
		return
	}

	cmd, err := showQRImage(uuid)
	if err != nil {
		log.Printf("创建二维码失败: %s\n", err.Error())
		return
	}
	log.Println("请使用微信扫描二维码以登录")
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		os.Remove(QRImagePath)
	}()

	redirectUri, code, tip := "", "", 1
	for code != "200" {
		redirectUri, code, tip, err = waitForLogin(uuid, tip)
		if err != nil {
			log.Printf("描述二维码登录失败: %s\n", err.Error())
			return
		}
	}

	bReq, err := login(redirectUri)
	if err != nil {
		log.Printf("登录失败: %s\n", err.Error())
		return
	}
	log.Println("登录成功")

	index := strings.LastIndex(redirectUri, "/")
	if index == -1 {
		index = len(redirectUri)
	}
	baseUri := redirectUri[:index]

	if err = webwxInit(baseUri, bReq); err != nil {
		log.Printf("初始化失败: %s\n", err.Error())
		return
	}
	log.Println("初始化成功")

	memberList, count, err := webwxGetContact(baseUri, bReq)
	if err != nil {
		log.Printf("获取联系人失败: %s\n", err.Error())
		return
	}
	log.Printf("总共获取到[%d]联系人，其中普通好友[%d]人，开始查找\"好友\"\n", count, len(memberList))

	if err = search(baseUri, bReq, memberList); err != nil {
		log.Printf("查找\"好友\"失败: %s\n", err.Error())
		return
	}

	show()
	// TODO 删除创建的群
	// TODO 关闭打开的二维码
	log.Println("感谢你使用本程序！")
}
