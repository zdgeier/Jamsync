
# Jamsync 

Jamsync is an open-source version control system for game development. You can try out a hosted version at [jamsync.dev](https://jamsync.dev). We'll be hosting the code on Github until we can bootstrap on Jamsync. 

Join the [Discord](https://discord.gg/6bK3GPKhpa) for questions or email me at [zach@jamsync.dev](zach@jamsync.dev).

### Algorithm

The idea behind Jamsync based off of the [rsync algorithm](https://www.andrew.cmu.edu/course/15-749/READINGS/required/cas/tridgell96.pdf) and [Content Defined Chunking (CDC)](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia). If you haven't read these, I would highly recommend them!

### How Jamsync uses Rsync and CDC

The main idea behind Jamsync is that we can store the operations sent by the sender in an rsync-like stream to track changes to a file. This means we treat rsync operations like a delta chain that we can use later to regenerate the file. The storage of deltas and their usage to regenerate a file is similar to the Mercurial concept of a [Revlog](https://www.mercurial-scm.org/wiki/Revlog). However, the advantage of using rsync blocks is that we can efficiently store changes to, and regenerate, arbitrarily large files since these blocks can be streamed and regenerated independently.

### Data pointers

In each block, we can store the location of the last data block to regenerate the file efficiently. By using blocks instead of an xdelta approach, we can store pointers in each block find the last actual data block to use in the file, rather than regenerating the file through a delta chain which Mercurial does. Mercurial [essentially caches the entire file](https://wiki.mercurial-scm.org/RevlogNG#Deficiencies_in_original_revlog_format) at certain points and uses this later to have a smaller regeneration length.

### Branches

A chain of changes, formed by the process above, can be used to regenerate every file in a project. Branches can be automatically rebased on top of the mainline. This means that every branch will always be up-to-date. If conflicts occur during the rebase, a branch will need manual merging.

### Limitations

The goal is to be able to handle over 100M files and over 1TB-sized files in a single repository. We're not there yet in the current implementation (~1M files with 16GB-sized files) but should be there in the next couple months.

### Implementation

Jamsync is being written from scratch in [Golang](https://go.dev/) and uses [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) to store projects and change information. [gRPC](https://grpc.io/) and [Protocol buffers](https://developers.google.com/protocol-buffers) are used for service definitions and data serialization.

## Developing

### Setup

Note that this is for setting up development or compiling Jamsync yourself. If
you want binaries and installation instructions go to
[jamsync.dev/login](https://jamsync.dev). Documentation for self-hosting is going to be pretty sparse right now, but reach out on [Discord](https://discord.gg/6bK3GPKhpa) if you need any help. The general steps are below and you should be able to solve most issues by resolving any errors/dependencies that occur.

1. Install Go, Protoc, Make
2. Setup env with Auth0 variables
3. Run desired `make` target

### Architecture

As a general overview there are three services that currently compose Jamsync -- web, server, client.

1. Web - runs the REST API and website
2. Server - runs the backend server for storing and retrieving changes
3. Client - client-side CLI tool to connect to the server

The client and web services connect through gRPC to the backend server and we
interact online through the web REST API. More documentation will be added in
the future to detail how changes are stored but changes and project files are currently stored in the `jb/` directory where the server is started.

## Contributing

Although I welcome contributions, I ask that you DM me first before making changes so you don't waste time. Jamsync is early in development so things could change drastically architecture or implementation.

## Contact

Email Zach Geier at [zach@jamsync.dev](mailto:zach@jamsync.dev) or join the [Discord](https://discord.gg/6bK3GPKhpa) 
