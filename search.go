package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

func (this *Webwx) GetContact() (err error) {
	name, resp := "webwxgetcontact", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&skey=%s&r=%s", this.BaseUri, name, this.Request.PassTicket, this.Request.Skey, time.Now().Unix())
	if err = this.send(apiUri, name, nil, resp); err != nil {
		return
	}

	this.MemberList, this.Total = make([]*Member, 0, resp.MemberCount/5*2), resp.MemberCount
	for i := 0; i < this.Total; i++ {
		if resp.MemberList[i].IsNormal() {
			this.MemberList = append(this.MemberList, resp.MemberList[i])
		}
	}

	return
}

func (this *Webwx) createChatRoom(users []User, namesMap map[string]*Member) (err error) {
	data, err := json.Marshal(Request{
		BaseRequest: this.Request,
		MemberCount: len(users),
		MemberList:  users,
		Topic:       "",
	})
	if err != nil {
		return
	}

	name, resp := "webwxcreatechatroom", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&r=%s", this.BaseUri, name, this.Request.PassTicket, time.Now().Unix())
	if err = this.send(apiUri, name, bytes.NewReader(data), resp); err != nil {
		return
	}

	this.ChatRoomName = resp.ChatRoomName
	this.search(resp.MemberList, namesMap)
	return
}

func (this *Webwx) deleteMember(users []string) (err error) {
	data, err := json.Marshal(Request{
		BaseRequest:   this.Request,
		ChatRoomName:  this.ChatRoomName,
		DelMemberList: strings.Join(users, ","),
	})
	if err != nil {
		return
	}

	name, fun, resp := "webwxupdatechatroom", "delmember", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?fun=%s&pass_ticket=%s", this.BaseUri, name, fun, this.Request.PassTicket)
	return this.send(apiUri, fun, bytes.NewReader(data), resp)
}

func (this *Webwx) addMember(users []string, namesMap map[string]*Member) (err error) {
	data, err := json.Marshal(Request{
		BaseRequest:   this.Request,
		ChatRoomName:  this.ChatRoomName,
		AddMemberList: strings.Join(users, ","),
	})
	if err != nil {
		return
	}

	name, fun, resp := "webwxupdatechatroom", "addmember", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?fun=%s&pass_ticket=%s", this.BaseUri, name, fun, this.Request.PassTicket)
	if err = this.send(apiUri, fun, bytes.NewReader(data), resp); err != nil {
		return
	}

	this.search(resp.MemberList, namesMap)
	return
}

func (this *Webwx) Search() (err error) {
	total := len(this.MemberList)
	if total == 0 {
		return
	}

	names, users, namesMap := make([]string, 0, *GroupNum), make([]User, 0, *GroupNum), make(map[string]*Member, *GroupNum)
	for i, member := range this.MemberList {
		if len(this.ChatRoomName) == 0 {
			users = append(users, User{
				UserName: member.UserName,
			})
		}
		names, namesMap[member.UserName] = append(names, member.UserName), member

		if len(names) < *GroupNum {
			continue
		}

		if i / *GroupNum > 0 {
			log.Printf("程序等待 %ds 后将继续查找，请耐心等待...\n", *Duration)
			time.Sleep(time.Duration(*Duration) * time.Second)
		}

		if len(this.ChatRoomName) > 0 {
			err = try("增加群成员", func() error {
				return this.addMember(names, namesMap)
			})
		} else {
			err = try("创建群", func() error {
				return this.createChatRoom(users, namesMap)
			})
		}

		if err != nil {
			return
		}

		if err = try("删除群成员", func() error {
			return this.deleteMember(names)
		}); err != nil {
			return
		}

		names, namesMap = names[:0], make(map[string]*Member, *GroupNum)
		this.progress(i+1, total)
	}

	this.progress(total, total)
	return
}

func try(name string, f func() error) (err error) {
	duration, retry := *Duration, 0
	for retry <= *Retry {
		if retry > 0 {
			log.Printf("程序将等待 %ds 后进行重试[%s]...\n", duration, name)
			time.Sleep(time.Duration(duration) * time.Second)
			if retry < 3 {
				duration *= 2
			}
		}

		if err = f(); err == nil {
			return
		}

		retry++
		log.Printf("[%s]失败:[%s]\n", name, err.Error())
	}

	return fmt.Errorf("程序重试[%s] %d 次后出错: %s, 过段时间再尝试吧\n", name, retry-1, err.Error())
}
