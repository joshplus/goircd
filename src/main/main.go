package main

import (
        "fmt"
        "net"
)

/* The main function! 
 * kicks off the program by doing some initializations and starting the network listener
 */
func main() {
		//initialize the roomlist and start the nickname service
		Initialize_irc() //block until everything is initialized
		go Nick_service()
		
		//start listening over the network
		service := "0.0.0.0:6667"
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
                  go Handle_conn(conn, id);
                } else {
                  fmt.Println("Listener error. Sad day :-(\n");
                  conn.Close()
                  break
                }
        }
}

//Error checking function used by main
func checkError(err error) {
        if err != nil {
                fmt.Println("Fatal error ", err.Error())
        }
}
