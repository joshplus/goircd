package main

import (
        "fmt"
        "net"
        "bufio"
        "sync"
	)

//chatroom list and its associated mutex
var room_list map[string] *Chatroom
var room_mutex sync.RWMutex


//source that is used in message structs whenever the server sends a message 
//itself such as when sending an error message
var server_src user

//nickname service global channels
var nickserv chan nick_req

func Initialize_irc(){
	room_list = make(map[string] *Chatroom)
	server_src = user{nickname: "server.localhost", id: -1, channel: make (chan message)}
}

/*
 * handle_conn takes a network connection and a user ID. This ID is a serial number for users
 * who connect to the server. It is put in the user object which is created and used for comparisons.  
 */
func Handle_conn(conn net.Conn, id int){
       //Make sure that the connection is closed when (and however) the function ends
       defer conn.Close()
       defer fmt.Println("Disconnected:", id);
       
       //Create a user struct for this user
       var usr user
       usr.id=id
       usr.nickname=""
       usr.channel = make(chan message, 25)
       
       //ensure that this user leaves all chat rooms when disconnected
       defer part_all(&usr)
       
       //Pick a random nickname for the initial connection
       //this is important for clients understanding welcome messages
       //if there is a nickname collision
       for !change_nick(&usr,randNick()) {}
       fmt.Println("Got connection #", id);
       
       //spawn another goroutine to handle inbound messages
       reader := bufio.NewReader(conn)
	   go handle_in_conn(&usr, reader)
	   
	   //Send the initial connection message
	   conn.Write([]byte("NOTICE AUTH :*** Welcome to the server!\r\n"))
	   
	   //This is the outbound message event loop. Look for messages to send out and send them
       for  {
      	  	imsg, ok := <- usr.channel
      	  	if !ok { break }
      	  	_, err := conn.Write(imsg.message)
      	  	_, err2 := conn.Write([]byte("\r\n"))//messages are handled internally without the newline
      	  	//exit on error - this may be caused by a closed socket. It is intentionally invoked by the 
      	  	//inbound conneciton handler 
      	  	if err != nil || err2 != nil {
      	  		break
      	  	}
       }		
}

//handlie_in_conn
//handle inbound connections from the user.  
func handle_in_conn(usr *user, reader *bufio.Reader) {
	for { //When a message comes in, parse it 
		n:=false
		buf, n, err := reader.ReadLine()
      	if err != nil { break } //on error break and force  the outbound to fail
      	if (!n) {
    		parse_input(string(buf), usr)
      	} else {
      		fmt.Println("Message larger than buffer :-/ ")
	    	parse_input(string(buf), usr)
	    }
     }
	 //We are no longer reading due to a read error
	 //send a message to the conn_hanler to force an error on
	 //write which ensures a quick disconnection
	 var msg message
     msg.message = []byte("Bye! I'm not listening any more!")
     defer func() {recover()}() //If someone typed /quit,the channel will be closed so recover to keep from blowing up
     usr.channel <- msg
}