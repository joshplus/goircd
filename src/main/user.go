package main

import (
        "fmt"
)

//Represents a connected user
type user struct{
	nickname string //nickname of user (displayed to other users and in chat rooms)
	id int //sequential connection ID - used in a few places to compare users. Could probably be replaced
	rooms []*Chatroom 
	channel chan message
}

//message - this is what a user sends to a chatroom and what the chatroom sends to the user
//The byte slice is the message body sent to the user
type message struct {
	dest Chatroom
	source *user
	message []byte	//Byte slices because the RFC says that a message is a sequence of up to 512 bytes terminated by \r\n
}

/* This function shouldn't be necessary 
 * I ran in to some cases where I couldn't create an msg struct but I couldn't figure out why
 * this creates msg structs for me to get around that problem. 
 * You can see an example in send_room()
 */
func get_msg() message{
	var msg message;
	return msg;
}


//exit all chatrooms. This is done on QUIT and on closed socket
func part_all(usr *user){
	for _, r := range usr.rooms {
		fmt.Println(usr.nickname + " leaving " + r.name);
		r.part_room(usr)
	}
}

//send a user-to-user message. usr is the source user. nick si the destination nickname
func priv_msg_usr(usr *user, nick string, message string){
	var r nick_req //create the nickname service request object
	r.nick=nick
	r.usr = usr
	r.search = true
	r.retchan = make (chan bool, 1) //buffer to ensure we get a response (see the nickservices non-blocking code)
	r.uchan = make (chan *user, 1)
	nickserv <- r
	if <- r.retchan { //if the nick was found, create a message and send to the user
		u := <- r.uchan
		fmt.Println("User " + nick + " found for u2u private message -> "+u.nickname);
		msg := get_msg()
		msg.message = []byte(fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", usr.nickname, nick, message))
		msg.source = usr
		u.channel <- msg 
	} else {
		fmt.Println("User " + nick + " not found for u2u private message :-(");
	}
}


