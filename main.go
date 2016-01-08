package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	Success = 0
)

var (
	Debug    = flag.Bool("debug", false, "是否为 debug 模式")
	GroupNum = flag.Int("num", 35, "每组人数")
	Duration = flag.Int("d", 16, `接口调用时间间隔, 值设为 13 时亲测出现"操作太频繁"`)
	Progress = flag.Int("p", 50, "进度条")
	DeviceId = flag.String("did", "e000000000000000", "device id")

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

func main() {
	flag.Parse()

	log.Println("本程序的查询结果可能会引起一些心理上的不适，三五好友足已，请做好心理准备...")
	log.Println("开始")

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

	index := strings.LastIndex(redirectUri, "/")
	if index == -1 {
		index = len(redirectUri)
	}
	baseUri := redirectUri[:index]

	if err = webwxInit(baseUri, bReq); err != nil {
		log.Printf("初始化失败: %s\n", err.Error())
		return
	}

	if err = webwxGetContact(baseUri, bReq); err != nil {
		log.Printf("获取联系人失败: %s\n", err.Error())
		return
	}

	log.Println("结束")
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

func createFile(name string, data []byte) (err error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Write(data)
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

func webwxGetContact(baseUri string, bReq *BaseRequest) (list []*Member, count int, err error) {
	reader, err := callApi(baseUri, "webwxgetcontact", bReq, nil)
	if err != nil {
		return
	}

	r := new(ContactResp)
	if err = json.NewDecoder(reader).Decode(r); err != nil {
		return
	}

	if !r.IsSuccess() {
		err = fmt.Errorf("message:[%s]", r.BaseResponse.ErrMsg)
		return
	}

	list, count = make([]*Member, 0, r.MemberCount/5*2), r.MemberCount
	for i := 0; i < count; i++ {
		if r.MemberList[i].IsNormal() {
			list = append(list, r.MemberList[i])
		}
	}

	return
}
