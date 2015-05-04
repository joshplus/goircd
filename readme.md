# Go IRCd

## Project Goals

My plan with this project was to create an IRC server in Go with a focus on concurrency. The impitus for this design was my study of concurrent programming practices. As you'll see in the source, I focus a lot on concurrency and used a few different methods to ensure that concurrent updates happens safely. The main things that I use are a readers-writers semaphore (from the Sync package) and a goroutine that handles concurrent access to a shared resource (nick-to-user mapping).

## What this does and does not do

Since my focus was on concurrency, I did not focus on implementing all features in RFC 1459 (or later RFCs). The short version of what it does/does not do is this: It lets users chat but there is no one "in charge."

The following features are supported:

* Nicknames
* Join/Part in channels
* Chatting in a channel
* List of users in a channel
* Setting a channel's status
* User-to-user messages
* Response to user PING messages
* List of channels*
* Basic login commands*
* My every own unofficial terminate server command
* Who for channels

Things This doesn't do:
 
* Filtering on channel lists
* Basic login commands only do enough to make IRC clients happy
* User or Channel MODE commands
* Things that require modes like moderation, kicking, kline, gline, etc
* Who for users

## Usage

Compile and run the execuatble. It will be listening on port 6667 on your machine. If you want to change the port number, you have to modify main.go to use a different port. 

## Notes on design, problems, etc

Of course, this doesn't do everything an IRC server needs to so there should be additional features.

The parser is wrtting in one big, ugly function. It could really use some additional work and should be split into different functions.

The code is all sitting in the "Main" package since this is wrtten as an execuatble. It'd be great to break the parts of this out into a package so that it could be imported. The parser could even be written to be reusable between user and server if one wanted to. 

## License & The Author

This code was written by Josh Abraham. You are free to use it on your own system and contribute. If you wish to use it commercially, please contact me. Once I'm over the shock, I'll be glad to talk to you!

I'll add a license to this at some point in the near future.
