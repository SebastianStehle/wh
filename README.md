# Simple Webhook Tunnel

This is a simple http tunnel for webhooks. The server accepts http requests and forwards them via GRPC to the command line interface, which executes the requests and sends the response to the server, which forwards the response back to the origin.

The main purpose of the repository is to act as a learning project for Golang.

## Tech Stack

* GO
* Grpc for the tunnel
* Echo for the http server
* Templ for a super simple view
* Cobra for the CLI commands.

## Todos and Contributing

The project is very early in development and there are a lot of todos. In general the goal is to keep it simple:

* Automatic build and releases in Github.
* Unit tests where applicable.
* Docker image for the server.
* Binaries for the CLI and the server for all major operation systems.
* Testing, testing, testing....
* Streaming or large requests / response via multiple grpc messages.
* Tunnels for TCP requests.
* Removing all the quirks that I created as a new Golang developer.

If you are an experienced Go developer, every feedback is really appreciated. Contributions are welcome.

## How to run it

### The server

For the server you also need NodeJS to build tailwind for the user interface.

```
cd server
go get
npm i
npm run dev
```

The npm command will run 3 parallel processes: 

1. Tailwind compiler
2. Templ compiler on watch mode.
3. Air for live reloading of the webapp (https://github.com/air-verse/air).

The UI also has a simple live-reload feature via Javascript. It creates an event source connection to the webserver and whenever it reconnects to the server it assumes that the server was restarted and also reloads the webpage in the browser.

The server will run on port **5000** and listen for changes.

> For authentication a simple API Key based system is implemented. The default key is just **key**.

### The client (CLI)

For the client only Go is needed. Therefore just go into the client folder and run the CLI: 

```
cd client
go run main.go
```

The CLI stores the configuration in the user folder. First you need to connect to a server

```
go run main.go config add http://localhost:5000 key
```
then you can connect to an an endpoint:

```
go run main.go tunnel google https://google.com
```

Now every request ot http://localhost:5000/endpoints/google will be forwareded to the CLI, then google and back.