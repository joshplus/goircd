package main

import (
        "fmt"
		"math/rand"
		)

//nickname service request 
type nick_req struct{
	nick string
	usr *user
	retchan chan bool
	uchan chan *user
	search bool //true=look for this nickname; false=set user's nickname to this
}

//Change the nickname. Create nick service object and send it
func change_nick(usr *user, nick string) bool{
	var r nick_req
	r.nick=nick
	r.usr = usr
	r.search = false
	r.retchan = make (chan bool, 1) //buffer to ensure respone (see non-blocking code)
	nickserv <- r
	return <- r.retchan;
}

//generate a random nickname. This was based on some code I found online but I've lost the reference to it
func randNick() string{
	var chars = []rune("abcdefghijklmnopqrstuvwxyz1234567890-");
	nick:="user-"
	b := make([]rune, 6)
	for i:= range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return nick+string(b)
}

//Nickname service - this is used as a go routine
func Nick_service(){
	var nick_reg map[string] *user //map to store nickname information
	nickserv = make (chan nick_req, 0) //global  
	nick_reg = make (map[string] *user)
	for {
		//read from nickserv channel. If there is an error, continue and keep running the nick service. 
		req, ok := <- nickserv
		if !ok {
			fmt.Println("Nick service error!")
			continue
		}
		//name change mode
		if !req.search {
			if nick_reg[req.nick] == nil { //only new name changes
				delete(nick_reg, req.usr.nickname) //remove old name if it exists
				nick_reg[req.nick]=req.usr
				req.usr.nickname=req.nick
				select { //non blocking send success/fail back
					case req.retchan <- true:
					default:
				}
			} else { //non blocking send success/fail back
				select {
					case req.retchan <- false:
					default:
				}
			}
		}
		//search mode 
		if req.search {
			if nick_reg[req.nick] == nil { //nickname not found
				select {
					case req.retchan <- false:
					default:
				}
			} else { //nickname found, return the user struct
				select {
					case req.retchan <- true:
					default:
				}
				select {
					case req.uchan <- nick_reg[req.nick]:
					default:
				}

			}
		}
	}
}
