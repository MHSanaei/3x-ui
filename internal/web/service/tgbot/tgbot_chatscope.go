package tgbot

import "github.com/mymmrac/telego"

// Authorization keys on the sender but conversation state keys on the chat, and those
// are the same identity only in a private chat — so group traffic is dropped at the door.
func isPrivateChat(chat telego.Chat) bool {
	return chat.Type == telego.ChatTypePrivate
}
