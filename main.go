package main

import (
	"fmt"
	"github.com/cxindex/xmpp"
	"io/ioutil"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const room = "ttyh@conference.jabber.ru"
const name = "Жобe"
const me = "hypnotoad@xmpp.ru"

// const room = "kkkkkkk2@conference.jabber.ru"
var (
	ping  time.Time
	admin []string
	cs    = make(chan xmpp.Stanza)
	next  xmpp.Stanza
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
start:
	Conn, err := xmpp.Dial("xmpp.ru:5222", "hypnotoad", "xmpp.ru", "pass", "AllHailHypnotoad", nil)
	if err != nil {
		log.Println("Conn", err)
		time.Sleep(5 * time.Second)
		goto start
	}
	if err := Conn.SignalPresence("dnd", "is there some food in this world?", 12); err != nil {
		log.Println("Signal", err)
		time.Sleep(5 * time.Second)
		goto start
	}
	if err := Conn.SendPresence(room+"/"+name, ""); err != nil {
		log.Println("Presence", err)
		time.Sleep(5 * time.Second)
		goto start
	}
	
	go func(Conn *xmpp.Conn) {
		for {
			select {
			case <-time.After(60 * time.Second):
				//just in case
				Conn.SendIQ("jabber.ru", "set", "<keepalive xmlns='urn:xmpp:keepalive:0'> <interval>60</interval> </keepalive>")
				if _, _, err = Conn.SendIQ("jabber.ru", "get", "<ping xmlns='urn:xmpp:ping'/>"); err != nil {
					log.Println("KeepAlive err:", err)
					return
				}
				ping = time.Now()
				log.Println("KeepAlive is sent")
			}
		}
	}(Conn)

	go func(Conn *xmpp.Conn) {
		for {
			next, err := Conn.Next()
			if err != nil {
				log.Println("Next err:", err)
				return
			}
			cs <- next
		}
	}(Conn)

	for {
		select {
		case next = <-cs:
			log.Println(next.Value)
		case <-time.After(65 * time.Second):
			log.Println(Conn.Close(), "\n\t", "closed after 65 seconds of inactivity")
			goto start
		}
		switch t := next.Value.(type) {
		case *xmpp.ClientPresence:
			PresenceHandler(Conn, t)
		case *xmpp.ClientIQ:
			if t.Type == "result" {
				since := time.Since(ping)
				log.Println(since, t.From)
			}
		case *xmpp.ClientMessage:
			if len(t.Delay.Stamp) == 0 && len(t.Subject) == 0 && GetNick(t.From) != name {
				if t.Type == "groupchat" {
					go MessageHandler(Conn, t)
				} else if xmpp.RemoveResourceFromJid(strings.ToLower(t.From)) == me {
					go SelfHandler(Conn, t)
				}
			}
		default:
			log.Println("da fuq?")
		}
	}
	log.Println(Conn.Close(), "\n\t", "wtf am I doing here?")
	time.Sleep(5 * time.Second)
	goto start
}

func SelfHandler(Conn *xmpp.Conn, Msg *xmpp.ClientMessage) {
	Msg.Body = strings.TrimSpace(Msg.Body)
	Conn.Send(room, "groupchat", Msg.Body)
	Strip(&Msg.Body, &Msg.From)
	if err := exec.Command("bash", "-c", GetCommand("!"+Msg.Body, Msg.From, "./func/")).Run(); err != nil {
		log.Println(err)
		return
	}
}

func MessageHandler(Conn *xmpp.Conn, Msg *xmpp.ClientMessage) {
	Msg.Body = strings.TrimSpace(Msg.Body)
	f := func(s string, s2 *string) bool {
		ok, _ := regexp.MatchString(s, *s2)
		return ok
	}
	switch {
	case f("^\\!", &Msg.Body): //any external command
		Strip(&Msg.Body, &Msg.From)
		cmd := exec.Command("bash", "-c", GetCommand(Msg.Body, Msg.From, "./plugins/"))
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			log.Println(err)
			return
		}
		out, _ := ioutil.ReadAll(stdout)
		outerr, _ := ioutil.ReadAll(stderr)
		if err := cmd.Wait(); err != nil {
			if err.Error() == "exit status 127" {
				Conn.Send(room, "groupchat", fmt.Sprintf("%s: WAT", GetNick(Msg.From)))
				return
			}
		}
		if len(outerr) != 0 && in(admin, Msg.From) {
			Conn.Send(Msg.From, "chat", string(outerr))
		}
		Conn.Send(room, "groupchat", strings.TrimRight(string(out), " \n"))
	case f("^"+name, &Msg.Body): //chat
		Strip(&Msg.Body, &Msg.From)
		r, _ := regexp.Compile("^\\./chat/" + name + "[:,]")
		command := r.ReplaceAllString(GetCommand("!"+Msg.Body, Msg.From, "./chat/"), "./chat/answer")
		out, err := exec.Command("bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Println(err)
			return
		}
		Conn.Send(room, "groupchat", strings.TrimRight(string(out), " \n"))
	}
}

func PresenceHandler(Conn *xmpp.Conn, Prs *xmpp.ClientPresence) {
	switch Prs.Item.Affiliation {
	case "owner":
		fallthrough
	case "admin":
		if Prs.Item.Role != "none" {
			if !in(admin, Prs.From) {
				admin = append(admin, Prs.From)
			}
		} else {
			admin = del(admin, Prs.From)
		}
	}
}

func GetCommand(body, from, dir string) string {
	split := strings.SplitAfterN(body, " ", 2)
	r, _ := regexp.Compile("^\\!")
	command := r.ReplaceAllString(split[0], dir) + " '" + GetNick(from) + "' '" + strconv.FormatBool(in(admin, from)) + "'"
	if len(split) == 2 {
		command += " '" + split[1] + "'"
	}
	return command
}

func Strip(s, s2 *string) {
	r, _ := regexp.Compile("(`|\\$|\"|'|\\.\\.)") //strip
	*s = r.ReplaceAllString(*s, "")
	*s2 = r.ReplaceAllString(*s2, "")
}

func GetNick(s string) string {
	slash := strings.Index(s, "/")
	if slash != -1 {
		return s[slash+1:]
	}
	return s
}

func in(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func pos(slice []string, value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}
	return -1
}

func del(slice []string, value string) []string {
	if i := pos(slice, value); i >= 0 {
		return append(slice[:i], slice[i+1:]...)
	}
	return slice
}
