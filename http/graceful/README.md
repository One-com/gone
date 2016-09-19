# gone/http/graceful

A golang library wrapping around the stdlib http.Server to provide graceful shutdown.

## Example

```go
	s := &Server{
		Server: &http.Server{},
	}
	
	l, err := net.Listen("tcp", "")
	if err != nil {
		log.Fatal(err)
	}
	
	go func() {
		time.Sleep(time.Second)
		s.Shutdown() // Signal the server for shutdown.
		s.Wait() // Wait for the last connection to be closed.
	}() 

	err = s.Serve(l) // Block until Shutdown() is called
	if err != nil {
		log.Fatal(err)
	}
```

