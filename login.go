package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/skratchdot/open-golang/open"
)

func (this *Webwx) getUUID() (uuid string, err error) {
	jsloginUrl := "https://login.weixin.qq.com/jslogin"
	params := url.Values{}
	params.Set("appid", "wx782c26e4c19acffb")
	params.Set("fun", "new")
	params.Set("lang", "zh_CN")
	params.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	resp, err := this.Client.PostForm(jsloginUrl, params)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	ds := string(data)

	if *Debug {
		log.Printf("[debug] uuid:[%s]\n", ds)
	}

	code, err := findData(ds, "window.QRLogin.code = ", ";")
	if err != nil {
		return
	}
	if code != "200" {
		err = fmt.Errorf("错误的状态码:[%s], data:[%s]", code, ds)
		return
	}

	uuid, err = findData(ds, `window.QRLogin.uuid = "`, `";`)
	if err != nil {
		return
	}

	return
}

func (this *Webwx) showQRImage(uuid string) (err error) {
	qrUrl := `https://login.weixin.qq.com/qrcode/` + uuid
	params := url.Values{}
	params.Set("t", "webwx")
	params.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequest("POST", qrUrl, strings.NewReader(params.Encode()))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := this.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if err = createFile(this.QRImagePath, data, false); err != nil {
		return
	}

	return open.Start(this.QRImagePath)
}

func (this *Webwx) waitForLogin(uuid string, tip int) (redirectUri, code string, rt int, err error) {
	loginUrl, rt := fmt.Sprintf("https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?tip=%d&uuid=%s&_=%s", tip, uuid, time.Now().Unix()), tip
	resp, err := this.Client.Get(loginUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	ds := string(data)

	if *Debug {
		log.Printf("[debug] wait for login:[%s]\n", ds)
	}

	code, err = findData(ds, "window.code=", ";")
	if err != nil {
		return
	}

	rt = 0
	switch code {
	case "201":
		log.Println("成功扫描,请在手机上点击确认以登录")
	case "200":
		redirectUri, err = findData(ds, `window.redirect_uri="`, `";`)
		if err != nil {
			return
		}
		redirectUri += "&fun=new"
	case "408":
	case "0":
		err = fmt.Errorf("超时，请重新运行程序")
	default:
		err = fmt.Errorf("未知错误，请重试")
	}

	return
}

func (this *Webwx) WaitForLogin() (err error) {
	uuid, err := this.getUUID()
	if err != nil {
		err = fmt.Errorf("获取 uuid 失败: %s", err.Error())
		return
	}

	if err = this.showQRImage(uuid); err != nil {
		err = fmt.Errorf("创建二维码失败: %s", err.Error())
		return
	}
	defer os.Remove(this.QRImagePath)
	log.Println("请使用微信扫描二维码以登录")

	code, tip := "", 1
	for code != "200" {
		this.RedirectUri, code, tip, err = this.waitForLogin(uuid, tip)
		if err != nil {
			err = fmt.Errorf("描述二维码登录失败: %s", err.Error())
			return
		}
	}

	return
}

func (this *Webwx) login() (err error) {
	resp, err := this.Client.Get(this.RedirectUri)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	reader := resp.Body.(io.Reader)
	if *Debug {
		var data []byte
		data, err = ioutil.ReadAll(reader)
		if err != nil {
			return
		}
		log.Printf("[debug] login:[%s]\n", string(data))
		reader = bytes.NewReader(data)
	}

	if err = xml.NewDecoder(reader).Decode(this.Request); err != nil {
		return
	}

	if this.Request.Ret != Success {
		err = fmt.Errorf("message:[%s]", this.Request.Message)
		return
	}

	this.Request.DeviceID = *DeviceId
	return
}

func (this *Webwx) initBaseUri() {
	index := strings.LastIndex(this.RedirectUri, "/")
	if index == -1 {
		index = len(this.RedirectUri)
	}

	this.BaseUri = this.RedirectUri[:index]
}

func (this *Webwx) webwxInit() (err error) {
	data, err := json.Marshal(Request{
		BaseRequest: this.Request,
	})
	if err != nil {
		return
	}

	name, resp := "webwxinit", new(InitResp)
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&skey=%s&r=%s", this.BaseUri, name, this.Request.PassTicket, this.Request.Skey, time.Now().Unix())
	if err = send(apiUri, name, bytes.NewReader(data), resp); err != nil {
		return
	}

	this.Myself = resp.User.UserName
	return
}

func (this *Webwx) Login() (err error) {
	if err = this.login(); err != nil {
		return
	}

	if err = this.webwxInit(); err != nil {
		err = fmt.Errorf("初始化失败: %s", err.Error())
		return
	}
	log.Println("初始化成功")

	return
}
