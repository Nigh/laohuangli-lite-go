package main

import (
	"fmt"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/sync/syncmap"
	tele "gopkg.in/telebot.v3"
)

const (
	IDLE int = iota
	NOMINATE
)

type privateChat struct {
	State   int
	Timeout int
}

var chats = syncmap.Map{}

func chatLoad(id int64) privateChat {
	var chat privateChat
	chatx, _ := chats.Load(id)
	chat = chatx.(privateChat)
	return chat
}

func init() {
	chats = syncmap.Map{}
	go updateChats()
}
func msgDelete() func(c tele.Context) error {
	return func(c tele.Context) error {
		n := getNomination(c.Message().ID)
		if n != nil {
			c.Send("提名词条 \"`"+n.Content+"`\" 已被删除，投票立即失效", tele.ModeMarkdownV2)
			deleteNomination(c.Message().ID)
		}
		defer func() {
			c.Delete()
		}()
		return nil
	}
}
func cmdOnChat(c tele.Context) error {
	if _, ok := chats.Load(c.Chat().ID); !ok {
		chats.Store(c.Chat().ID, privateChat{
			State: IDLE,
		})
	}
	chat := chatLoad(c.Chat().ID)
	defer func() {
		chat.Timeout = 9
		chats.Store(c.Chat().ID, chat)
	}()
	switch c.Text() {
	case "/start":
		fallthrough
	case "/help":
		chat.State = IDLE
		return c.Send("提名新词条请发送 /nominate\n列举提名词条请发送 /list")
	case "/list":
		chat.State = IDLE
		existNomination := 0
		var msg string
		for _, v := range nominations {
			if v.NominatorID == c.Sender().ID {
				existNomination++
				msg += fmt.Sprintf("提名词条 \"`%s`\" 赞成 `%d` 票，反对 `%d` 票\n", v.Content, len(v.ApprovedUsers), len(v.RefusedUsers))
			}
		}
		if existNomination == 0 {
			return c.Send("你还没有提名任何词条，请发送 /nominate 提名新词条")
		} else {
			return c.Send(msg, tele.ModeMarkdownV2)
		}
	case "/nominate":
		existNomination := 0
		for _, v := range nominations {
			if v.NominatorID == c.Sender().ID {
				existNomination++
			}
		}
		if existNomination >= 5 {
			chat.State = IDLE
			return c.Send("你已经提名过太多词条了，请等待提名投票结束再提交新词条吧！")
		}
		chat.State = NOMINATE
		return c.Send("请输入你要提名的词条内容：")
	}
	return nil
}
func msgOnChat(c tele.Context) error {
	senderName := fullName(c.Sender())
	if _, ok := chats.Load(c.Chat().ID); !ok {
		chats.Store(c.Chat().ID, privateChat{
			State: IDLE,
		})
	}
	chat := chatLoad(c.Chat().ID)
	defer func() {
		chat.Timeout = 9
		chats.Store(c.Chat().ID, chat)
	}()

	switch chat.State {
	case NOMINATE:
		nominate := strings.TrimSpace(c.Text())
		if len(nominate) < 1 {
			return c.Send("提名内容太短，请重新提名。")
		}
		if nominate[0] == '/' {
			return c.Send("格式错误，请重新提名。")
		}
		success, reason := dupNominationCheck(nominate, senderName)
		for _, v := range reason {
			c.Send(v)
		}
		chat.State = IDLE
		if success == -1 {
			chat.State = NOMINATE
		}
		if success == 0 {
			mk := &tele.ReplyMarkup{ResizeKeyboard: true}
			publishBtn := mk.Query("发布", nominate)
			deleteBtn := mk.Data("删除", "deleteBtn")

			mk.Inline(mk.Row(publishBtn, deleteBtn))
			newNomination := nomination{
				Content:       nominate,
				NominatorName: senderName,
				NominatorID:   c.Sender().ID,
				Time:          time.Now().Unix(),
				ApprovedUsers: make([]int64, 0),
				RefusedUsers:  make([]int64, 0),
			}
			msg, err := b.Send(c.Chat(), buildVoteText(newNomination), mk, tele.ModeMarkdownV2)
			if err != nil {
				fmt.Println(err)
			} else {
				newNomination.UUID = uuid.NewV4().String()
				newNomination.ID = msg.ID
				addNomination(newNomination)
				b.Handle(&deleteBtn, msgDelete())
			}
			return err
		}
	}
	return nil
}

func updateChats() {
	second := time.NewTicker(10 * time.Second)
	for range second.C {
		chats.Range(func(i, v any) bool {
			chat := v.(privateChat)
			if chat.Timeout <= 0 {
				chats.Delete(i)
			} else {
				chat.Timeout--
				chats.Store(i, chat)
			}
			return true
		})
	}
}