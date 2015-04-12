package main

import (
        "fmt"
        "net"
        "bufio"
        "strings"
        "sync"
        "os"
)

var room_list map[string] *chatroom
var room_mutex sync.Mutex
var server_src user
var nickserv chan nick_req

type nick_req struct{
	nick string
	usr *user
	retchan chan bool
	uchan chan *user
	search bool
}

type user struct{
	nickname string
	id int
	rooms []*chatroom 
	channel chan message
}

type message struct {
	dest chatroom
	source *user
	message []byte	
}

type chatroom struct {
	name string
	topic string
	users []*user
	usercnt int
}

func nick_service(){
	var nick_reg map[string] *user
	nickserv = make (chan nick_req, 0)
	nick_reg = make (map[string] *user)
	for {
		req, ok := <- nickserv
		if !ok {
			fmt.Println("Nick service error!")
			continue
		}
		if nick_reg[req.nick] == nil {
			delete(nick_reg, req.usr.nickname)
			if !req.search{
				nick_reg[req.nick]=req.usr
				req.usr.nickname=req.nick
			}
			select { //non blocking send success/fail back
				case req.retchan <- true:
				default:
			}
			if req.search && req.uchan != nil{
				select { //non blocking send success/fail back
					case req.uchan <- nick_reg[req.nick]:
					default:
				}	
			}
		} else { //non blocking send success/fail back
			select {
				case req.retchan <- false:
				default:
			}
		}
	}
}

func join_room(name string, u *user) *chatroom{
	room_mutex.Lock();
	//if the room exists, join it otherwise create it
	if room_list[name] == nil {
		cr := chatroom{name: name}
		cr.users = make([]*user,0)
		room_list[name]=&cr //save roomlist to map
		cr.join_room(u)
	} else {
		room_list[name].join_room(u)
	}
	var room *chatroom
	room = room_list[name]
	room_mutex.Unlock();
	return room
}

func part_room(name string, u *user){
	if room_list[name] != nil{
		room_list[name].part_room(u)
	}
}

func (c *chatroom) join_room(u *user){
	c.users = append(c.users, u)
	u.rooms = append(u.rooms, c)
	fmt.Printf("User #%d joins %s\n", u.id, c.name)
	c.usercnt++
	fmt.Println("We now have", c.usercnt, "users in", c.name ,"with an slice of size", len(c.users))
}

func (c *chatroom) part_room(u *user){
	//may need a mutex
	var i, j int
	room_mutex.Lock();
	for i=0; i<=len(c.users); i++ {
		if c.users[i].id == u.id { break }
	}
	for j=0; j<=len(u.rooms); i++ {
		if u.rooms[j] == c { break }
	}
	if i==len(c.users) || j==len(u.rooms){
		//This user is not in this room
		return
	}
	fmt.Printf("User #%d departs %s\n", u.id, c.name)
	
	c.users = append(c.users[:i], c.users[i+1:]...)
	u.rooms = append(u.rooms[:j], u.rooms[j+1:]...)
	c.usercnt--;
	if c.usercnt == 0 {
		delete(room_list,c.name)
	}
	room_mutex.Unlock();
	//send_room(p1, usr, fmt.Sprintf(":%s PART %s\r\n", usr.nickname, p1))
	msg := get_msg()
	msg.dest=*c
	msg.source = u
	msg.message = []byte(fmt.Sprintf(":%s PART %s\r\n", u.nickname, c.name))
	c.send_room(&msg)
}

func (c *chatroom) send_room(msg *message){
		fmt.Printf("%s(%d): %s\n", c.name, msg.source.id,string(msg.message))
		for _, usr := range c.users {
			if usr.id != msg.source.id {
				select {
					case usr.channel <- *msg:
					default:
				}
			}
		}
}
func get_msg() message{
	var msg message;
	return msg;
}
func send_room(room string, usr *user, message string) {
	room_mutex.Lock()
	rl := room_list //make a local copy to be threadsafe
	room_mutex.Unlock()
	if rl[room] == nil {
		fmt.Printf("Bad news, You asked me to post to '%s' but I have no idea what it is!\n",room)
		return 
	}
	msg := get_msg()
	msg.source=usr
	msg.dest=*rl[room]
	msg.message = []byte(message)
	rl[room].send_room(&msg)
}

func main() {
		room_list = make(map[string] *chatroom)
		go nick_service()
		server_src = user{nickname: "server.localhost", id: -1, channel: make (chan message)}
		service := "0.0.0.0:6000"
        tcpAddr, err := net.ResolveTCPAddr("tcp", service)
        checkError(err)
        listener, err := net.ListenTCP("tcp", tcpAddr)
        checkError(err)
        fmt.Println("Server listerning")
        for  id:=0; true; id++ {
                conn, err := listener.Accept()
                if err == nil {
                  fmt.Println("Connection OK!");
                  go handle_conn(conn, id);
                } else {
                  fmt.Println("Listener error. Sad day :-(\n");
                  conn.Close()
                  break
                }
        }
}

func handle_conn(conn net.Conn, id int){
       defer conn.Close()
       defer fmt.Println("Disconnected:", id);
       var usr user
       usr.id=id
       usr.nickname=""
       usr.channel = make(chan message, 25)
       defer part_all(&usr)
       fmt.Println("Got connection #", id);
       reader := bufio.NewReader(conn)
	   go handle_in_conn(&usr, reader)
	   conn.Write([]byte("NOTICE AUTH :*** Welcome to the server!\r\n"))
       for  {
      	  	imsg, ok := <- usr.channel
      	  	if !ok { break }
      	  	_, err := conn.Write(imsg.message)
      	  	_, err2 := conn.Write([]byte("\r\n"))
      	  	if err != nil || err2 != nil {
      	  		break
      	  	}
       }		
}

func handle_in_conn(usr *user, reader *bufio.Reader) {
	for {
		n:=false
		buf, n, err := reader.ReadLine()
      	if err != nil { break }
      	if (!n) {
    		parse_input(string(buf), usr)
      	} else {
      		fmt.Println("Message larger than buffer :-/ ")
	    	parse_input(string(buf), usr)
	    }
     }
	 //We are no longer reading due to a read error
	 //send a message to the conn_hanler to force an error on
	 //write and a quick disconnection
	 var msg message
     msg.message = []byte("Bye! I'm not listening any more!")
     defer func() {recover()}() //If someone typed /quit,the channel will be closed so recover to keep from blowing up
     usr.channel <- msg
}

func parse_input(line string, usr *user){
	fmt.Printf("<USER %d: %s\n",usr.id, line)
	comstr := strings.SplitAfterN(line, " ", 2)
	c := strings.ToLower(comstr[0]) //c == command
	p1:=""; mb:=""; var p1t, mbt []string //deal with different length arguments
	if len(comstr) > 1 { p1t = strings.SplitAfterN(comstr[1], " ", 2)} //param 1
	if len(p1t) > 0 {p1=strings.Trim(strings.Title(p1t[0])," \r\n")}
	if len(comstr)  > 1 { mbt = strings.SplitAfterN(comstr[1], ":", 2) } //message body
	if len(mbt) > 1 {mb=mbt[1]}
	switch c {
		case "nick ":
			if (p1 == "") {break}
			oldnick := usr.nickname
			if (change_nick(usr, p1)){
				m := message {source: &server_src, message: []byte(fmt.Sprintf(":%s NICK %s\r\n", oldnick, p1))}
				if true || usr.nickname!="" { usr.channel <- m }
				for _, x := range usr.rooms {
					x.send_room(&m)
				}
			} else {
				server_msg(usr, "433", ":Nickname already in use");
			}
		case "privmsg ":
			if p1[0] == '#' || p1[0] == '&' {
				send_room(p1, usr, fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", usr.nickname, p1, mb))
			} else {
				priv_msg_usr(usr , p1, mb)
			}
		case "topic ":
			room := room_list[p1]
			if room != nil {
				room.topic=mb
				//TOPIC #gobomo :irc client research (foamer2)
				m := message {source: usr, message: []byte(fmt.Sprintf(":%s TOPIC %s :%s\r\n", usr.nickname, p1, mb))}
				if usr.nickname!="" { usr.channel <- m }
				for _, x := range usr.rooms {
					x.send_room(&m)
				}
			}
		case "part ":
			if len(p1) == 0 { return }
			if p1[0] != '#' && p1[0] != '&' {p1 = "#" + p1}
			part_room(p1, usr)
			usr.channel <- message {source: &server_src, message: []byte(fmt.Sprintf(":%s PART %s\r\n", usr.nickname, p1))}
		case "join ":
			if p1[0] != '#' && p1[0] != '&' {p1 = "#" + p1}
			for _,r := range usr.rooms { //don't join a room twice
				if r.name == p1 {return}
			}
			room := join_room(p1, usr)
			send_room(p1, usr, fmt.Sprintf(":%s JOIN %s\r\n", usr.nickname, p1))
			usr.channel <- message {source: &server_src, message: []byte(fmt.Sprintf(":%s JOIN %s\r\n", usr.nickname, p1))}
			server_msg(usr, "332", p1 + " :" + room.topic)
			fallthrough
		case "names ":
			room := room_list[p1]
			if (room != nil){
					nstr := " = " + room.name + " :"
					for _, u := range room.users {
						if (len(nstr) + len(u.nickname) + 2) < 450 { 
							nstr = nstr + u.nickname + " "
						} else {
							server_msg(usr, "353",nstr);
							nstr = " = " + usr.nickname + " :"
						}
					}
					if (len(nstr) > 0){
						server_msg(usr, "353",nstr);
					}
					nstr = usr.nickname + " " + p1 + " :End of /names list"
					server_msg(usr, "366",nstr);
			} else {
				/*nstr := " = " + room.name + " :"
				server_msg(usr, "353",nstr);
				nstr = usr.nickname + " " + p1 + " :End of /names list"
				server_msg(usr, "366",nstr);*/
			} 
		case "who ":
			room := room_list[p1]
			if (room != nil){
				for _, u := range room.users {
					server_msg(usr, "352",room.name + " " + u.nickname + " example.net server " + u.nickname +  " Hx :0 " + u.nickname);
				}
				server_msg(usr, "315", room.name + " :End of /WHO list");
			}
		case "list ", "list":
			rl:=room_list
			server_msg(usr, "321", "Channel :Users  Name");
			for _, room := range rl{
				nstr := fmt.Sprintf("%s %d :%s", room.name, len(room.users), room.topic) 
				server_msg(usr, "322", nstr);
			}
			server_msg(usr, "323", " :End of /LIST");
			
		case "ping ":
			usr.channel <- message {message: []byte(fmt.Sprintf("PONG %s\r\n", p1))}
			fmt.Printf(">user %d: %s\n",usr.id, fmt.Sprintf("PONG %s\r\n", p1))
		case "user ":
			if usr.nickname == "" { return }
			server_msg(usr, "001","Thanks for connecting!");
			server_msg(usr, "002","Your host is server.localhost");
			server_msg(usr, "003","This server was created Fri Apr 11, 2015 at 00:33:00 UTC");
			server_msg(usr, "004","server.localhost ircgo-blah-blah");
			server_msg(usr, "005","server.localhost ircgo-blah-blah");
			server_msg(usr, "254",fmt.Sprintf("%d: channels found", len(room_list)));
			server_msg(usr, "375","server.localhost Message of the Day");
			server_msg(usr, "372","My MOTD");
			server_msg(usr, "376","End of /MOTD");
		case "pass ":
			server_msg(usr, "462",":Unauthorized command (already registered)")
		case "notice ":
			send_room(p1, usr, fmt.Sprintf(":%s NOTICE %s :%s\r\n", usr.nickname, p1, mb))
		case "explode ": 
			if p1 == "Please" { os.Exit(0) }
		case "quit ","quit":
			close(usr.channel)
		default: 
			fmt.Printf("Don't know what to do with %s from '%s'\n", c, line)
	}
}

func priv_msg_usr(usr *user, nick string, message string){
	var r nick_req
	r.nick=nick
	r.usr = usr
	r.search = true
	r.retchan = make (chan bool)
	r.uchan = make (chan *user)
	nickserv <- r
	if <- r.retchan {
		u := <- r.uchan
		msg := get_msg()
		msg.message = []byte(fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", usr.nickname, nick, message))
		msg.source = usr
		u.channel <- msg 
	}
}

func change_nick(usr *user, nick string) bool{
	var r nick_req
	r.nick=nick
	r.usr = usr
	r.search = false
	r.retchan = make (chan bool)
	nickserv <- r
	return <- r.retchan;
}

func server_msg(usr *user, code string, message string){
	mb:=fmt.Sprintf(":server %s %s %s", code, usr.nickname, message)
	fmt.Printf(">user %d: %s\n",usr.id, mb)
	msg := get_msg()
	msg.message=[]byte(mb)
	usr.channel <- msg
}

func part_all(usr *user){
	for _, r := range usr.rooms {
		fmt.Println(usr.nickname + " leaving " + r.name);
		r.part_room(usr)
	}
}

func checkError(err error) {
        if err != nil {
                fmt.Println("Fatal error ", err.Error())
        }
}
