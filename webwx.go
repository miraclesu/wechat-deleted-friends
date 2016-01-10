package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
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
	var err error
	if CurrentDir, err = os.Getwd(); err != nil {
		log.Panicln(err.Error())
	}
	QRImagePath = filepath.Join(CurrentDir, "qrcode.jpg")
}

type Request struct {
	BaseRequest *BaseRequest

	MemberCount int    `json:",omitempty"`
	MemberList  []User `json:",omitempty"`
	Topic       string `json:",omitempty"`

	ChatRoomName  string `json:",omitempty"`
	DelMemberList string `json:",omitempty"`
	AddMemberList string `json:",omitempty"`
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

type Caller interface {
	IsSuccess() bool
	Error() error
}

type Response struct {
	BaseResponse *BaseResponse
}

func (this *Response) IsSuccess() bool {
	return this.BaseResponse.Ret == Success
}

func (this *Response) Error() error {
	return fmt.Errorf("message:[%s]", this.BaseResponse.ErrMsg)
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

type MemberResp struct {
	Response
	MemberCount  int
	ChatRoomName string
	MemberList   []*Member
}

type Member struct {
	UserName     string
	NickName     string
	RemarkName   string
	VerifyFlag   int
	MemberStatus int
}

func (this *Member) IsOnceFriend() bool {
	return this.MemberStatus == 4
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

type Webwx struct {
	Client  *http.Client
	Request *BaseRequest

	CurrentDir  string
	QRImagePath string

	RedirectUri  string
	BaseUri      string
	ChatRoomName string // 用于查找好友的群号
	Myself       string // 自己

	Total      int       // 好友总数
	MemberList []*Member // 普通好友

	OnceFriends []string
}

func NewWebwx() (wx *Webwx, err error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return
	}

	transport := *(http.DefaultTransport.(*http.Transport))
	transport.ResponseHeaderTimeout = 1 * time.Minute
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	wx = &Webwx{
		Client: &http.Client{
			Transport: &transport,
			Jar:       jar,
			Timeout:   1 * time.Minute,
		},
		Request: new(BaseRequest),

		CurrentDir:  currentDir,
		QRImagePath: filepath.Join(currentDir, "qrcode.jpg"),
	}
	return
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

func createFile(name string, data []byte, isAppend bool) (err error) {
	oflag := os.O_CREATE | os.O_WRONLY
	if isAppend {
		oflag |= os.O_APPEND
	} else {
		oflag |= os.O_TRUNC
	}

	file, err := os.OpenFile(name, oflag, 0600)
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

func send(apiUri, name string, body io.Reader, call Caller) (err error) {
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

	reader := resp.Body.(io.Reader)
	if *Debug {
		var data []byte
		data, err = ioutil.ReadAll(reader)
		if err != nil {
			return
		}

		if err = createFile(filepath.Join(CurrentDir, name+".json"), data, strings.HasSuffix(name, "member")); err != nil {
			return
		}
		reader = bytes.NewReader(data)
	}

	if err = json.NewDecoder(reader).Decode(call); err != nil {
		return
	}

	if !call.IsSuccess() {
		return call.Error()
	}
	return
}
