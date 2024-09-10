# CLI

A command line interface to manage cccc.

## Build

Binaries for Linux, MacOS and Windows are generated in the build pipeline and can bei downloaded as artifacts of the `build_cli` step.

For development purposes, the cli can be locally build with

``` bash
go build -o cli main.go
```

## Usage

The cli is supposed to be easy to use for multiple instances of cccc. It has a simple manager built in to save API keys for multiple instances.

### Preparation

First, you need to generate a cccc API Key. In the web frontend go to `Administration` (click on your username in the bottom left corner).
Under `Users` click your user name to show details and edit it. If the field `API Key` is empty, generate a new one with the button next to the field and do not forget to click `save`.

Then add your key to the cli configuration. For example for local development use:

```bash
./cli config add https://localhost:3000 <Your API Key> -name local
```

### Manage the configuration of the CLI program

The command `config` offers functionality to manage the API keys.

* `config view` to show all servers known to the cli
* `config add` to add another server to be managed
* `config rm` to remove a server
* `config use` to change which is the active server

To show all available commands and how to use them, call

```bash
./cli config
```

### Manage the configuration of cccc

The command `sync` offers functionality to import and export configurations to and from cccc instances.

* `sync in` to import existing configuration (currently only the deployments) into the active server
* `sync out` to export the configuration (currently only the deployments) of the active server to a file

To show all available commands and how to use them, call

```bash
./cli sync
```
