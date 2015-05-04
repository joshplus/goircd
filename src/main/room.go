package main

import (
        "fmt"
		)

//chatroom - stores the information needed for a chatroom in IRC - a name, topic and list of users
type Chatroom struct {
	name string
	topic string
	users []*user
	usercnt int //this is just used for error checking in the code. Could be removed
}


//add a user to a chatroom. name is the name of the chatroom provided by the user, u
func join_room(name string, u *user) *Chatroom{
	room_mutex.Lock(); //must lock during this function because maps are not threadsafe
	//if the room exists, join it otherwise create it
	if room_list[name] == nil {
		cr := Chatroom{name: name}
		cr.users = make([]*user,0)
		room_list[name]=&cr //save roomlist to map
		cr.join_room(u) //call the join_room method in the chatroom's struct
	} else {
		room_list[name].join_room(u) //call the join_room method in the chatroom's struct
	}
	var room *Chatroom  //save the chatroom to return it
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
func (c *Chatroom) join_room(u *user){
	c.users = append(c.users, u)
	u.rooms = append(u.rooms, c)
	fmt.Printf("User #%d joins %s\n", u.id, c.name)
	c.usercnt++
	fmt.Println("We now have", c.usercnt, "users in", c.name ,"with an slice of size", len(c.users))
}

//attach a part room function to the chatroom struct
func (c *Chatroom) part_room(u *user){
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
func (c *Chatroom) send_room(msg *message){
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
