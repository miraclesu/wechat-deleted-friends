package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func webwxGetContact(baseUri string, bReq *BaseRequest) (list []*Member, count int, err error) {
	name := "webwxgetcontact"
	apiUri := fmt.Sprintf("%s/%s?pass_ticket=%s&skey=%s&r=%s", baseUri, name, bReq.PassTicket, bReq.Skey, time.Now().Unix())
	reader, err := send(apiUri, name, nil)
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
