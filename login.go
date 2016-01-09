package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

func getUUID() (uuid string, err error) {
	jsloginUrl := "https://login.weixin.qq.com/jslogin"
	params := url.Values{}
	params.Set("appid", "wx782c26e4c19acffb")
	params.Set("fun", "new")
	params.Set("lang", "zh_CN")
	params.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	resp, err := Client.PostForm(jsloginUrl, params)
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

func showQRImage(uuid string) (cmd *exec.Cmd, err error) {
	qrUrl := `https://login.weixin.qq.com/qrcode/` + uuid
	params := url.Values{}
	params.Set("t", "webwx")
	params.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	resp, err := Client.PostForm(qrUrl, params)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if err = createFile(QRImagePath, data); err != nil {
		return
	}

	command := "open"
	switch runtime.GOOS {
	case "linux":
		command = "xdg-open"
	case "windows", "darwin":
	default:
		err = fmt.Errorf("暂不支持此操作系统[%s]", runtime.GOOS)
		return
	}

	cmd = exec.Command(command, QRImagePath)
	err = cmd.Start()
	return
}

func waitForLogin(uuid string, tip int) (redirectUri, code string, rt int, err error) {
	loginUrl, rt := fmt.Sprintf("https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?tip=%d&uuid=%s&_=%s", tip, uuid, time.Now().Unix()), tip
	resp, err := Client.Get(loginUrl)
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

	switch code {
	case "201":
		log.Println("成功扫描,请在手机上点击确认以登录")
		rt = 0
	case "200":
		redirectUri, err = findData(ds, `window.redirect_uri="`, `";`)
		if err != nil {
			return
		}
		redirectUri += "&fun=new"
	case "408":
		err = fmt.Errorf("超时")
	default:
		err = fmt.Errorf("未知错误，请重试")
	}

	return
}

func login(redirectUri string) (bReq *BaseRequest, err error) {
	resp, err := Client.Get(redirectUri)
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

	bReq = new(BaseRequest)
	if err = xml.NewDecoder(reader).Decode(bReq); err != nil {
		return
	}

	if bReq.Ret != Success {
		err = fmt.Errorf("message:[%s]", bReq.Message)
		return
	}

	bReq.DeviceID = *DeviceId
	return
}

func webwxInit(baseUri string, bReq *BaseRequest) (err error) {
	br := Request{
		BaseRequest: bReq,
	}
	data, err := json.Marshal(br)
	if err != nil {
		return
	}

	reader, err := callApi(baseUri, "webwxinit", bReq, bytes.NewReader(data))
	if err != nil {
		return
	}

	r := new(InitResp)
	if err = json.NewDecoder(reader).Decode(r); err != nil {
		return
	}

	if !r.IsSuccess() {
		err = fmt.Errorf("message:[%s]", r.BaseResponse.ErrMsg)
		return
	}

	Myself = r.User.UserName
	return
}
