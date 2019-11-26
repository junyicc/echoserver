# Echo server
A simple multi-client echo server in Go: every message
sent by a client should be echoed to all connected clients.

## Server Characteristics

1. The server must manage and interact with its clients concurrently using goroutines and channels. 
   
2. Multiple clients should be able to connect/disconnect to the server simultaneously.

3. The server should assume that all messages are line-oriented, separated by newline (\n) characters.
   
4. The server must be responsive to slow-reading clients. 

   To handle these cases, your server should keep a queue of at most 100 outgoing messages to be written to the client at a later time. Messages sent to a slow-reading client whose outgoing message buffer has reached the maximum capacity of 100 should simply be dropped. If the slow-reading client starts reading again later on, the server should make sure to write any buffered messages in its queue back to the client.
