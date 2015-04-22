package main

import (
        "fmt"
        "net"
        "bufio"
        "strings"
        "sync"
        "os"
	"math/rand"
)

//chatroom list and its associated mutex
var room_list map[string] *chatroom
var room_mutex sync.RWMutex


//source that is used in message structs whenever the server sends a message 
//itself such as when sending an error message
var server_src user

//nickname service global channels
var nickserv chan nick_req

//nickname service request 
type nick_req struct{
	nick string
	usr *user
	retchan chan bool
	uchan chan *user
	search bool //true=look for this nickname; false=set user's nickname to this
}

//Represents a connected user
type user struct{
	nickname string //nickname of user (displayed to other users and in chat rooms)
	id int //sequential connection ID - used in a few places to compare users. Could probably be replaced
	rooms []*chatroom 
	channel chan message
}

//message - this is what a user sends to a chatroom and what the chatroom sends to the user
//The byte slice is the message body sent to the user
type message struct {
	dest chatroom
	source *user
	message []byte	//Byte slices because the RFC says that a message is a sequence of up to 512 bytes terminated by \r\n
}

//chatroom - stores the information needed for a chatroom in IRC - a name, topic and list of users
type chatroom struct {
	name string
	topic string
	users []*user
	usercnt int //this is just used for error checking in the code. Could be removed
}

//Nickname service - this is used as a go routine
func nick_service(){
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

//add a user to a chatroom. name is the name of the chatroom provided by the user, u
func join_room(name string, u *user) *chatroom{
	room_mutex.Lock(); //must lock during this function because maps are not threadsafe
	//if the room exists, join it otherwise create it
	if room_list[name] == nil {
		cr := chatroom{name: name}
		cr.users = make([]*user,0)
		room_list[name]=&cr //save roomlist to map
		cr.join_room(u) //call the join_room method in the chatroom's struct
	} else {
		room_list[name].join_room(u) //call the join_room method in the chatroom's struct
	}
	var room *chatroom  //save the chatroom to return it
	room = room_list[name]
	room_mutex.Unlock();
	return room
}

//Have user, u, leave a chatroom called name
func part_room(name string, u *user){
	if room_list[name] != nil{
		room_list[name].part_room(u) //the mutex operation is called from within the chatroom part_room function
	}
}

//attach a join room function to the chatroom struct
//just some appends and some debugging output
func (c *chatroom) join_room(u *user){
	c.users = append(c.users, u)
	u.rooms = append(u.rooms, c)
	fmt.Printf("User #%d joins %s\n", u.id, c.name)
	c.usercnt++
	fmt.Println("We now have", c.usercnt, "users in", c.name ,"with an slice of size", len(c.users))
}

//attach a part room function to the chatroom struct
func (c *chatroom) part_room(u *user){
	//i = the user's position in the chatroom; j = the chatroom's position in the user's list
	var i, j int
	room_mutex.Lock();
	//locate the user and chatroom
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
	//re-create the user and room slices without the room, user respectively
	c.users = append(c.users[:i], c.users[i+1:]...)
	u.rooms = append(u.rooms[:j], u.rooms[j+1:]...)
	c.usercnt--;
	//clean up stale rooms
	if c.usercnt == 0 {
		delete(room_list,c.name)
	}
	room_mutex.Unlock();
	//notify users in the chatroom that the user has parted
	msg := get_msg()
	msg.dest=*c
	msg.source = u
	msg.message = []byte(fmt.Sprintf(":%s PART %s\r\n", u.nickname, c.name))
	c.send_room(&msg)
}

//send a message to each user in a room
func (c *chatroom) send_room(msg *message){
		fmt.Printf("%s(%d): %s\n", c.name, msg.source.id,string(msg.message))
		//this is not threadsafe however, it is not really a problem to deliver a message to a user
		//who has concurrently left a room or to not send a message to a user who has concurrently
		//joined a room. 
		for _, usr := range c.users {
			if usr.id != msg.source.id {
				select { //this is non-blocking to deal with users that disconnect from the server concurrently
					case usr.channel <- *msg:
					default:
				}
			}
		}
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

/* 
 * Send a message to every user in a given room
 * Because this requires reading from a mutex-locked 
 */
func send_room(room string, usr *user, message string) {
	room_mutex.RLock()
	rl := room_list[room] 
	room_mutex.RUnlock();
	if rl == nil {
		fmt.Printf("Bad news, You asked me to post to '%s' but I have no idea what it is!\n",room)
		return 
	}
	//Make a message including casting the string to a byte array and send it to each user
	msg := get_msg()
	msg.source=usr
	msg.dest=*rl
	msg.message = []byte(message)
	rl.send_room(&msg)
}

/* The main function! 
 * kicks off the program by doing some initializations and starting the network listener
 */
func main() {
		//initialize the roomlist and start the nickname service
		room_list = make(map[string] *chatroom) 
		go nick_service()
		
		//start listening over the network
		server_src = user{nickname: "server.localhost", id: -1, channel: make (chan message)}
		service := "0.0.0.0:6000"
        tcpAddr, err := net.ResolveTCPAddr("tcp", service)
        checkError(err)
        listener, err := net.ListenTCP("tcp", tcpAddr)
        checkError(err)
        fmt.Println("Server listerning")
        //For each new connection, create a handler and set a new, higher ID
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
       for !change_nick(&usr,randNick()) {}
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
				priv_msg_usr(usr, p1, mb)
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
	r.retchan = make (chan bool, 1)
	r.uchan = make (chan *user, 1)
	nickserv <- r
	if <- r.retchan {
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

func change_nick(usr *user, nick string) bool{
	var r nick_req
	r.nick=nick
	r.usr = usr
	r.search = false
	r.retchan = make (chan bool, 1)
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

func randNick() string{
	var chars = []rune("abcdefghijklmnopqrstuvwxyz1234567890-");
	nick:="user-"
	b := make([]rune, 6)
	for i:= range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return nick+string(b)
}

func checkError(err error) {
        if err != nil {
                fmt.Println("Fatal error ", err.Error())
        }
}
