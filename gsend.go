package main

import (
	"github.com/cxindex/xmpp"
	"io/ioutil"
	"os"
)

func main() {
	Conn, err := xmpp.Dial("xmpp.ru:5222", "hypnotoad", "xmpp.ru", "password", "ic", nil)
	if err != nil {
		println(err)
		return
	}

	if s, err := ioutil.ReadAll(os.Stdin); err == nil {
		Conn.Send("hypnotoad@xmpp.ru", "chat", string(s[:len(s)-1]))
		return
	}
}
