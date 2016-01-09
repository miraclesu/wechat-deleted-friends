package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func createChatRoom(baseUri string, bReq *BaseRequest, users []User) (chatRoomName string, num int, err error) {
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
			num++
		}
	}

	return
}
