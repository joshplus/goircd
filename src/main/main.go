package main

import (
        "fmt"
        "net"
        "bufio"
        "os"
        "strings"
)


type user struct{
	nickname string
	id int
	rooms []*chatroom
	channel chan message
}

type message struct {
	dest chatroom
	source user
	message []byte	
}

type chatroom struct {
	name string
	topic []byte
	users []user
	usercnt int
}

func (c *chatroom) join_room(u *user){
	//may need a mutex
	c.users = append(c.users, *u)
	u.rooms = append(u.rooms, c)
	fmt.Printf("User #%d joins %s\n", u.id, c.name)
	c.usercnt++
	fmt.Println("We now have", c.usercnt, "users in", c.name ,"with an slice of size", len(c.users))
}

func (c *chatroom) part_room(u *user){
	//may need a mutex
	var i, j int
	for i=0; i<len(c.users); i++ {
		if c.users[i].id == u.id { break }
	}
	for j=0; j<len(u.rooms); i++ {
		if u.rooms[j] == c { break }
	}
	fmt.Printf("User #%d parts %s\n", u.id, c.name)
	
	c.users = append(c.users[:i], c.users[i+1:]...)
	u.rooms = append(u.rooms[:j], u.rooms[j+1:]...)
	c.usercnt--;
}

func (c *chatroom) send_room(msg message){
		fmt.Printf("%s(%d): %s\n", c.name, len(c.users),string(msg.message))
		for _, usr := range c.users {
			if usr.id != msg.source.id {
				select {
					case usr.channel <- msg:
					default:
				}
			}
		}
}

func main() {
        //chan_handler := make (chan chanmsg, 10)
        var cr chatroom
        cr.name="Test"
        cr.topic=[]byte("Test Topical")
        //cr.users = make([]chan []byte, 0)
        
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
                  go handle_conn(conn, &cr, id);
                } else {
                  fmt.Println("Listener error. Sad day :-(\n");
                  conn.Close()
                  break
                }
        }
}

func handle_conn(conn net.Conn, cr *chatroom, id int){
       defer conn.Close()
       defer fmt.Println("Disconnected:", id);
       var usr user
       usr.id=id
       usr.nickname=""
       usr.channel = make(chan message, 25)
       defer part_all(&usr)
       //defer part_room: (cr.part)
       f := bufio.NewWriter(os.Stdout)
       fmt.Println("Got connection #", id);
       cr.join_room(&usr)
       reader := bufio.NewReader(conn)
	   go handle_in_conn(&usr, cr, reader)
       for  {
      	  	imsg:= <- usr.channel
      	  	_, err := conn.Write(imsg.message)
      	  	_, err2 := conn.Write([]byte("\r\n"))
      	  	if err != nil || err2 != nil{
      	  		break
      	  	}
          	f.Flush()
       }		
}

func handle_in_conn(usr *user, cr *chatroom, reader *bufio.Reader) {
	for {
		n:=false
		buf, n, err := reader.ReadLine()
      	if err != nil { break }
      	var msg message;
      	msg.dest=*cr;
      	msg.source=usr;
      	msg.message=[]byte(buf)
      	if (!n) {
    		cr.send_room(msg)
      	} else {
      		fmt.Println("Message larger than buffer :-/ ")
	    	cr.send_room(msg)
	    }
     }
}

func parse_input(line string, usr *user){
	comstr := strings.SplitAfterN(line, " ", 1)
	c := strings.ToLower(comstr[0]) //c == command
	switch c {
		case "nick":
			usr.nickname=comstr[1]
		case "join":
			usr.nickname=comstr[1]
	}
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
