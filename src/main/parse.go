package main

import (
        "fmt"
        "strings"
        "os"
)

//Send a message to usr in the format used from server-to-client communications
func server_msg(usr *user, code string, message string){
	mb:=fmt.Sprintf(":server %s %s %s", code, usr.nickname, message)
	fmt.Printf(">user %d: %s\n",usr.id, mb)
	msg := get_msg()
	msg.message=[]byte(mb)
	usr.channel <- msg
}

//Parse input takes a string which represnet input from the client
//it parses it and decides what to do with it. Much of the RFC-specific behavior
//is in this behemoth of a function!
func parse_input(line string, usr *user){
	fmt.Printf("<USER %d: %s\n",usr.id, line) //Debug output of what the user saw
	
	//Split line into parts. The first token should be the command (verb)
	//additional tokens are given separated by spaces
	//the final token starts with a : which denotes a token that may have spaces (used for messages)
	//I do not handle all possible command parameters. This should be redone as a regex.
	comstr := strings.SplitAfterN(line, " ", 2)
	c := strings.ToLower(comstr[0]) //c == command
	p1:=""; mb:=""; var p1t, mbt []string //deal with different length arguments
	if len(comstr) > 1 { p1t = strings.SplitAfterN(comstr[1], " ", 2)} //param 1
	if len(p1t) > 0 {p1=strings.Trim(strings.Title(p1t[0])," \r\n")} //set parameter 1 to title case
	if len(comstr)  > 1 { mbt = strings.SplitAfterN(comstr[1], ":", 2) } //locate the message body
	if len(mbt) > 1 {mb=mbt[1]}
	
	//In this switch statement, I look at the verb with the space at the end not stripped out to improve efficnency
	//Commands that are just the verb (list) won't necessary have the space
	switch c {
		case "nick ": //set the user's nickname
			if (p1 == "") {break} //no nickname was provided
			oldnick := usr.nickname //store old nick because you need to provide an accurate "from" message
			if (change_nick(usr, p1)){ //if we can change nicks, make appropriate updates
				m := message {source: &server_src, message: []byte(fmt.Sprintf(":%s NICK %s\r\n", oldnick, p1))}
				if true || usr.nickname!="" { usr.channel <- m }
				for _, x := range usr.rooms {
					x.send_room(&m)
				}
			} else {
				server_msg(usr, "433", ":Nickname already in use");
			}
		case "privmsg ": //this is how you send messages between users and users or other rooms
			if p1[0] == '#' || p1[0] == '&' {
				send_room(p1, usr, fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", usr.nickname, p1, mb))
			} else {
				priv_msg_usr(usr, p1, mb)
			}
		case "topic ": //change the room topic
			room_mutex.RLock()
			room := room_list[p1] //this guy isn't threadsafe!
			room_mutex.RUnlock()
			if room != nil {
				room.topic=mb
				//send channel topic update to all users in the room
				m := message {source: usr, message: []byte(fmt.Sprintf(":%s TOPIC %s :%s\r\n", usr.nickname, p1, mb))}
				if usr.nickname!="" { usr.channel <- m }
				for _, x := range usr.rooms {
					x.send_room(&m)
				}
			}
		case "part ": //leave a chatroom
			if len(p1) == 0 { return }
			if p1[0] != '#' && p1[0] != '&' {p1 = "#" + p1} //force the room to fit the channel format (start with # or &)
			part_room(p1, usr)
			usr.channel <- message {source: &server_src, message: []byte(fmt.Sprintf(":%s PART %s\r\n", usr.nickname, p1))}
		case "join ": //join a chataroom
			if p1[0] != '#' && p1[0] != '&' {p1 = "#" + p1}
			for _,r := range usr.rooms { //don't join a room twice
				if r.name == p1 {return}
			}
			room := join_room(p1, usr) //join the room and update everyone that you're there
			send_room(p1, usr, fmt.Sprintf(":%s JOIN %s\r\n", usr.nickname, p1))
			usr.channel <- message {source: &server_src, message: []byte(fmt.Sprintf(":%s JOIN %s\r\n", usr.nickname, p1))}
			server_msg(usr, "332", p1 + " :" + room.topic)
			fallthrough //Give the list of users when someone joins a room. Not in the RFC but it is expected behavior
		case "names ":
			room_mutex.RLock()
			room := room_list[p1] //this guy isn't threadsafe
			room_mutex.RUnlock()
			if (room != nil){ //if the room exists, send the neames
					nstr := " = " + room.name + " :"
					for _, u := range room.users {
						if (len(nstr) + len(u.nickname) + 2) < 450 { 
							nstr = nstr + u.nickname + " "
						} else {
							server_msg(usr, "353",nstr); //names list ID
							nstr = " = " + usr.nickname + " :"
						}
					}
					if (len(nstr) > 0){
						server_msg(usr, "353",nstr); //continue names if there are any left in the buffer
					}
					nstr = usr.nickname + " " + p1 + " :End of /names list"
					server_msg(usr, "366",nstr); //366 = end names list
			} 
		case "who ": //give details about users in a room - this is like names but with more details
			room_mutex.RLock()
			room := room_list[p1] //this guy is still not threadsafe - that's why we keep locking him up!
			room_mutex.RUnlock()
			if (room != nil){
				for _, u := range room.users {
					//output one name per line with details. Most of the additional details are simulated such as Hx or the server name
					server_msg(usr, "352",room.name + " " + u.nickname + " example.net server " + u.nickname +  " Hx :0 " + u.nickname);
				}
				server_msg(usr, "315", room.name + " :End of /WHO list");
			}
		case "list ", "list": //list channels that exist. List may be given with no other parameters
			room_mutex.RLock()
			rl:=room_list //I only copy this because server_msg could possibly block if there is a bug or heavy load which would break join/part
			room_mutex.RUnlock()
			server_msg(usr, "321", "Channel :Users  Name");
			for _, room := range rl{
				nstr := fmt.Sprintf("%s %d :%s", room.name, len(room.users), room.topic) 
				server_msg(usr, "322", nstr);
			}
			server_msg(usr, "323", " :End of /LIST");
		case "ping ": //RFC requires a PING message to respond with PONG providing the same parameter
			usr.channel <- message {message: []byte(fmt.Sprintf("PONG %s\r\n", p1))}
			fmt.Printf(">user %d: %s\n",usr.id, fmt.Sprintf("PONG %s\r\n", p1))
		case "user ": //when a user logs in, send all of the "welcome" headers. Many clients halt without this.
			resetnick := false
			if usr.nickname == "" { usr.nickname="*"; resetnick=true }
			server_msg(usr, "001","Thanks for connecting!");
			server_msg(usr, "002","Your host is server.localhost");
			server_msg(usr, "003","This server was created Fri Apr 11, 2015 at 00:33:00 UTC");
			server_msg(usr, "004","server.localhost ircgo-blah-blah");
			server_msg(usr, "005","server.localhost ircgo-blah-blah");
			server_msg(usr, "254",fmt.Sprintf("%d: channels found", len(room_list)));
			server_msg(usr, "375","server.localhost Message of the Day");
			server_msg(usr, "372","My MOTD");
			server_msg(usr, "376","End of /MOTD");
			if resetnick { usr.nickname="" }
		case "pass ": //clients will send pass, they just need a repsonse to continue
			server_msg(usr, "462",":Unauthorized command (already registered)")
		case "notice ": //a notice is similar to a PRIVMSG. Does not currently support users
			send_room(p1, usr, fmt.Sprintf(":%s NOTICE %s :%s\r\n", usr.nickname, p1, mb))
		case "explode ": //my custom command to cause the server to shutdown
			if p1 == "Please" { os.Exit(0) }
		case "quit ","quit": //quit is the command for a user to gracefully disconnect
			close(usr.channel)
		default: 
			fmt.Printf("Don't know what to do with %s from '%s'\n", c, line)
	}
}