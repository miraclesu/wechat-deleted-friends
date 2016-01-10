package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
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

	wx, err := NewWebwx()
	if err != nil {
		log.Printf("程序出错了: %s\n", err.Error())
		return
	}

	if err = wx.WaitForLogin(); err != nil {
		log.Println(err.Error())
		return
	}

	log.Println("登录中...")
	if err = wx.Login(); err != nil {
		log.Printf("登录失败: %s\n", err.Error())
		return
	}
	log.Println("登录成功")

	Myself = wx.Myself
	if err = wx.GetContact(); err != nil {
		log.Printf("获取联系人失败: %s\n", err.Error())
		return
	}
	log.Printf("总共获取到[%d]联系人，其中普通好友[%d]人，开始查找\"好友\"\n", wx.Total, len(wx.MemberList))

	if err = wx.Search(); err != nil {
		log.Printf("查找\"好友\"失败: %s\n", err.Error())
		return
	}

	show()
	// TODO 删除创建的群
	// TODO 关闭打开的二维码
	log.Println("感谢你使用本程序！ 按 Ctrl+C 退出程序")
	WaitForExit()
}

func WaitForExit() os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM)
	return <-c
}
