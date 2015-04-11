package main

import (
        "fmt"
        "net"
        "bufio"
        "strings"
        "sync"
)

var room_list map[string] *chatroom
var room_mutex sync.Mutex
var server_src user

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
	topic []byte
	users []user
	usercnt int
}

func join_room(name string, u *user) *chatroom{
	room_mutex.Lock();
	//if the room exists, join it otherwise create it
	if room_list[name] == nil {
		cr := chatroom{name: name}
		cr.users = make([]user,0)
		room_list[name]=&cr //save roomlist to map
		cr.join_room(u)
	} else {
		room_list[name].join_room(u)
	}
	room_mutex.Unlock();
	return room_list[name]
}

func part_room(name string, u *user){
	if room_list[name] != nil{
		room_list[name].part_room(u)
	}
}

func (c *chatroom) join_room(u *user){
	c.users = append(c.users, *u)
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
      	  	imsg:= <- usr.channel
      	  	_, err := conn.Write(imsg.message)
      	  	_, err2 := conn.Write([]byte("\r\n"))
      	  	if err != nil || err2 != nil{
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
     msg.message = []byte("I'm not listening any more!")
     usr.channel <- msg
}

func parse_input(line string, usr *user){
	fmt.Printf("USER %d: %s\n",usr.id, line)
	comstr := strings.SplitAfterN(line, " ", 2)
	if len(comstr) < 2 {
		usr.channel <- message{message: []byte("Message too short")}
		return
	}
	c := strings.ToLower(comstr[0]) //c == command
	p1t := strings.SplitAfterN(comstr[1], " ", 2) //param 1
	p1:=""; mb:=""
	if len(p1t) > 0 {p1=strings.Trim(strings.Title(p1t[0])," \r\n")}
	mbt := strings.SplitAfterN(comstr[1], ":", 2) //message body
	if len(mbt) > 1 {mb=mbt[1]}
	switch c {
		case "nick ":
			m := message {message: []byte(fmt.Sprintf(":%s NICK %s\r\n", usr.nickname, p1))}
			usr.channel <- m
			for _, x := range usr.rooms {
				x.send_room(&m)
			}
			usr.nickname=comstr[1]
		case "join ":
			join_room(p1, usr)
			send_room(p1, usr, fmt.Sprintf(":%s JOIN %s\r\n", usr.nickname, p1))
			usr.channel <- message {message: []byte(fmt.Sprintf(":%s JOIN %s\r\n", usr.nickname, p1))}
		case "privmsg ":
			send_room(p1, usr, fmt.Sprintf(":%s PRIVMSG %s :%s\r\n", usr.nickname, p1, mb))
		case "topic ":
		case "part ":
			part_room(p1, usr)
			send_room(p1, usr, fmt.Sprintf(":%s PART %s\r\n", usr.nickname, p1))
			usr.channel <- message {message: []byte(fmt.Sprintf(":%s PART %s\r\n", usr.nickname, p1))}
		case "names ":
		case "who ":
		
		case "list ":
		case "ping ":
			usr.channel <- message {message: []byte(fmt.Sprintf("PONG %s\r\n", p1))}
		case "user ":
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
		default: 
			fmt.Printf("Don't know what to do with %s from '%s'\n", c, line)
	}
}

func server_msg(usr *user, code string, message string){
	mb:=fmt.Sprintf(":server %s %s %s", code, usr.nickname, message)
	msg := get_msg()
	msg.message=[]byte(mb)
	usr.channel <- msg
}

func part_all(usr *user){
	fmt.Printf("User #%d (in %d rooms) has disconnected!\n",usr.id, len(usr.rooms));
	for _, r := range usr.rooms {
		r.part_room(usr)
	}
}

func checkError(err error) {
        if err != nil {
                fmt.Println("Fatal error ", err.Error())
        }
}
