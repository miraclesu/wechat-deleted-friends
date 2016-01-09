package main

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	Success = 0
)

var (
	Client      = newClient()
	QRImagePath string
	CurrentDir  string
	Myself      string

	SpecialUsers = []string{
		"newsapp", "fmessage", "filehelper", "weibo", "qqmail",
		"tmessage", "qmessage", "qqsync", "floatbottle", "lbsapp",
		"shakeapp", "medianote", "qqfriend", "readerapp", "blogapp",
		"facebookapp", "masssendapp", "meishiapp", "feedsapp", "voip",
		"blogappweixin", "weixin", "brandsessionholder", "weixinreminder", "wxid_novlwrv3lqwv11",
		"gh_22b87fa7cb3c", "officialaccounts", "notification_messages", "wxitil", "userexperience_alarm",
	}
)

func init() {
	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	var err error
	if CurrentDir, err = os.Getwd(); err != nil {
		log.Panicln(err.Error())
	}
	QRImagePath = filepath.Join(CurrentDir, "qrcode.jpg")
}

type Request struct {
	BaseRequest *BaseRequest
}

type BaseRequest struct {
	XMLName xml.Name `xml:"error",json:"-"`

	Ret        int    `xml:"ret",json:"-"`
	Message    string `xml:"message",json:"-"`
	Skey       string `xml:"skey"`
	Wxsid      string `xml:"wxsid",json:"Sid"`
	Wxuin      int    `xml:"wxuin",json:"Uin"`
	PassTicket string `xml:"pass_ticket",json:"-"`

	DeviceID string `xml:"-"`
}

type Response struct {
	BaseResponse *BaseResponse
}

func (this *Response) IsSuccess() bool {
	return this.BaseResponse.Ret == Success
}

type BaseResponse struct {
	Ret    int
	ErrMsg string
}

type InitResp struct {
	Response
	User User
}

type User struct {
	UserName string
}

type ContactResp struct {
	Response
	MemberCount int
	MemberList  []*Member
}

type Member struct {
	UserName   string
	NickName   string
	RemarkName string
	VerifyFlag int
}

func (this *Member) IsNormal() bool {
	return this.VerifyFlag&8 == 0 && // 公众号/服务号
		!strings.HasPrefix(this.UserName, "@@") && // 群聊
		this.UserName != Myself && // 自己
		!this.IsSpecail() // 特殊账号
}

func (this *Member) IsSpecail() bool {
	for i, count := 0, len(SpecialUsers); i < count; i++ {
		if this.UserName == SpecialUsers[i] {
			return true
		}
	}
	return false
}

func newClient() (client *http.Client) {
	transport := *(http.DefaultTransport.(*http.Transport))
	transport.ResponseHeaderTimeout = 1 * time.Minute
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Panicln(err.Error())
	}

	client = &http.Client{
		Transport: &transport,
		Jar:       jar,
		Timeout:   1 * time.Minute,
	}
	return
}

func createFile(name string, data []byte) (err error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Write(data)
	return
}

func findData(data, prefix, suffix string) (result string, err error) {
	index := strings.Index(data, prefix)
	if index == -1 {
		err = fmt.Errorf("本程序已无法处理接口返回的新格式的数据:[%s]", data)
		return
	}
	index += len(prefix)

	end := strings.Index(data[index:], suffix)
	if end == -1 {
		err = fmt.Errorf("本程序已无法处理接口返回的新格式的数据:[%s]", data)
		return
	}

	result = data[index : index+end]
	return
}

func callApi(baseUri, name string, bReq *BaseRequest, body io.Reader) (reader io.Reader, err error) {
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&skey=%s&r=%s", baseUri, name, bReq.PassTicket, bReq.Skey, time.Now().Unix())

	method := "GET"
	if body != nil {
		method = "POST"
	}
	req, err := http.NewRequest(method, apiUri, body)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp, err := Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	reader = resp.Body.(io.Reader)
	if *Debug {
		var data []byte
		data, err = ioutil.ReadAll(reader)
		if err != nil {
			return
		}

		if err = createFile(filepath.Join(CurrentDir, name+".json"), data); err != nil {
			return
		}
		reader = bytes.NewReader(data)
	}

	return
}
