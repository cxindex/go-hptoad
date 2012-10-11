package main

import (
	"github.com/cxindex/xmpp"
	"fmt"
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
const name = "Жобе"
const me = "hypnotoad@xmpp.ru"

var (
	ping  time.Time
	admin []string
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	Conn, err := xmpp.Dial("xmpp.ru:5222", "hypnotoad", "xmpp.ru", "password", "AllHailHypnotoad", nil)
	if err != nil {
		log.Fatalln("Conn", err)
	}
	if err := Conn.SignalPresence("dnd", "is there some food in this world?", 11); err != nil {
		log.Fatalln("Signal", err)
	}
	if err := Conn.SendPresence(room+"/"+name, ""); err != nil {
		log.Fatalln("Presence", err)
	}

	//just in case
    go func() {
        for {
            select {
            case <-time.After(60 * time.Second):
                if _, _, err = Conn.SendIQ("xmpp.ru", "set", "<keepalive xmlns='urn:xmpp:keepalive:0'> <interval>60</interval> </keepalive>"); err != nil {
					log.Fatalln("KeepAlive", err)
				}
                log.Println("SENT 60")
            }
        }
    }()

	for {
		next, err := Conn.Next()
		if err != nil {
			log.Fatalln("Next", err)
		}
		switch t := next.Value.(type) {
		case *xmpp.ClientPresence:
			fmt.Println(t)
			PresenceHandler(Conn, t)
		case *xmpp.ClientIQ:
			fmt.Println(t)
			if t.Type == "result" {
				since := time.Since(ping)
				Conn.Send(room, "groupchat", fmt.Sprintf("%v %v", since, t.From))
			}
		case *xmpp.ClientMessage:
			fmt.Println(t)
			if len(t.Delay.Stamp) == 0 && len(t.Subject) == 0 && GetNick(t.From) != name {
				if t.Type == "groupchat" {
					go MessageHandler(Conn, t)
				} else if xmpp.RemoveResourceFromJid(strings.ToLower(t.From)) == me {
					go SelfHandler(Conn, t)
				}
			}
		default:
			fmt.Println(t)
		}
	}
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
	case f("^\\!ping($| )", &Msg.Body): //built-in
		to := strings.Split(Msg.Body, " ")
		if len(to) > 1 {
			for k, v := range to {
				if k == 0 {
					continue
				}
				ping = time.Now()
				Conn.SendIQ(room+"/"+v, "get", "<ping xmlns='urn:xmpp:ping'/>")
			}
			return
		}
		ping = time.Now()
		Conn.SendIQ(Msg.From, "get", "<ping xmlns='urn:xmpp:ping'/>")
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
		r, _ := regexp.Compile("^\\./chat/" + name + ":")
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
	r, _ := regexp.Compile("(`|\\$|\"|')") //strip
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
