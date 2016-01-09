package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

func webwxGetContact(baseUri string, bReq *BaseRequest) (list []*Member, count int, err error) {
	name, resp := "webwxgetcontact", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&skey=%s&r=%s", baseUri, name, bReq.PassTicket, bReq.Skey, time.Now().Unix())
	if err = send(apiUri, name, nil, resp); err != nil {
		return
	}

	list, count = make([]*Member, 0, resp.MemberCount/5*2), resp.MemberCount
	for i := 0; i < count; i++ {
		if resp.MemberList[i].IsNormal() {
			list = append(list, resp.MemberList[i])
		}
	}

	return
}

func createChatRoom(baseUri string, bReq *BaseRequest, users []User) (chatRoomName string, count int, err error) {
	br := Request{
		BaseRequest: bReq,
		MemberCount: len(users),
		MemberList:  users,
		Topic:       "",
	}
	data, err := json.Marshal(br)
	if err != nil {
		return
	}

	name, resp := "webwxcreatechatroom", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&r=%s", baseUri, name, bReq.PassTicket, time.Now().Unix())
	if err = send(apiUri, name, bytes.NewReader(data), resp); err != nil {
		return
	}

	chatRoomName = resp.ChatRoomName
	for _, member := range resp.MemberList {
		if member.IsOnceFriend() {
			OnceFriends = append(OnceFriends, fmt.Sprintf("昵称:[%s], 备注:[%s]", member.NickName, member.RemarkName))
			count++
		}
	}

	return
}

func deleteMember(baseUri string, bReq *BaseRequest, chatRoomName string, users []string) (err error) {
	br := Request{
		BaseRequest:   bReq,
		ChatRoomName:  chatRoomName,
		DelMemberList: strings.Join(users, ","),
	}
	data, err := json.Marshal(br)
	if err != nil {
		return
	}

	name, fun, resp := "webwxupdatechatroom", "delmember", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?fun=%s&pass_ticket=%s", baseUri, name, fun, bReq.PassTicket)
	err = send(apiUri, fun, bytes.NewReader(data), resp)
	return
}

func addMember(baseUri string, bReq *BaseRequest, chatRoomName string, users []string) (count int, err error) {
	br := Request{
		BaseRequest:   bReq,
		ChatRoomName:  chatRoomName,
		AddMemberList: strings.Join(users, ","),
	}
	data, err := json.Marshal(br)
	if err != nil {
		return
	}

	name, fun, resp := "webwxupdatechatroom", "addmember", new(MemberResp)
	apiUri := fmt.Sprintf("%s/%s?fun=%s&pass_ticket=%s", baseUri, name, fun, bReq.PassTicket)
	if err = send(apiUri, fun, bytes.NewReader(data), resp); err != nil {
		return
	}

	for _, member := range resp.MemberList {
		if member.IsOnceFriend() {
			OnceFriends = append(OnceFriends, fmt.Sprintf("昵称:[%s], 备注:[%s]", member.NickName, member.RemarkName))
			count++
		}
	}
	return
}

func search(baseUri string, bReq *BaseRequest, list []*Member) (err error) {
	if len(list) == 0 {
		return
	}

	chatRoomName, count := "", 0
	names, users := make([]string, 0, *GroupNum), make([]User, 0, *GroupNum)
	for _, member := range list {
		if len(chatRoomName) == 0 {
			users = append(users, User{
				UserName: member.UserName,
			})
		}
		names = append(names, member.UserName)

		if len(names) < *GroupNum {
			continue
		}

		if len(chatRoomName) > 0 {
			err = try("增加群成员", func() error {
				count, err = addMember(baseUri, bReq, chatRoomName, names)
				return err
			})
		} else {
			err = try("创建群", func() error {
				chatRoomName, count, err = createChatRoom(baseUri, bReq, users)
				return err
			})
		}

		if err != nil {
			return
		}

		if err = try("删除群成员", func() error {
			return deleteMember(baseUri, bReq, chatRoomName, names)
		}); err != nil {
			return
		}
	}

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
