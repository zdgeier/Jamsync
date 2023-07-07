![grapes on a vine](https://jamhub.dev/favicon.svg)

# JamHub

[JamHub](https://jamhub.dev) is an open-source version control system for game
development. We're building an open-source alternative to Perforce using
[Go](https://go.dev) and
[Content Defined Chunking (CDC)](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia)
techniques. Join the [Discord](https://discord.gg/6bK3GPKhpa) for questions or
email me at [zach@jamhub.dev](zach@jamhub.dev).

JamHub aims to solve the problems game developers have with currently available
version control systems. Perforce, which is widely used in the gamedev industry,
is closed-source, expensive and complicated. Open-source solutions like
Git/GitLFS and SVN do not scale for large projects with large files. They are
also missing features, like file locking, which are necessary for working with
binary files that cannot be merged. By using modern Content-Defined-Chunking and
hashing approaches we can also get performance improvements over current
systems.

## Algorithm

JamHub is based off the
[rsync algorithm](https://www.andrew.cmu.edu/course/15-749/READINGS/required/cas/tridgell96.pdf)
and
[FastCDC](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia)
papers. If you haven't read these, I would highly recommend them! They're pretty
approachable for anyone with a computer science background.

### Content Defined Chunking (CDC)

Content Defined Chunking is a method to split a file into predictable chunks
while being able to recognize those chunks when shifted or modified. Google's
[cdc-file-transfer](https://github.com/google/cdc-file-transfer) repository has
some helpful information about the details of the algorithm for syncing two
similar directories and a comparison to rsync. The main advantage of a CDC
approach over rsync's fixed chunk size is the removal of the rolling hash lookup
on every byte in the "no match" case. CDC bases the block boundaries on the
content of the file, so insertions and deletions do not affect the boundaries of
other blocks.

### How JamHub uses CDC

The main idea behind JamHub is that blocks sent during the CDC syncing process
can be used to track changes to a file. When changes are made locally, the
blocks that are changed or reused can be logged by a server. Essentially, this
creates a delta each time a push is made and we can use the delta to regenerate
the file.

### Optimizations

The storage of deltas and their usage to regenerate a file is similar to the
Mercurial concept of a
[Revlog](https://book.mercurial-scm.org/read/concepts.html#fast-retrieval).
However, there are several advantages of using chunks over a delta approach.

- Chunks can be regenerated and streamed independently
- Data pointers can directly reuse chunks from previous versions of a file

These two advantages are particularly useful when storing large files. Unlike
Git's approach of storing the entire history of a file locally, which can get
large over time, JamHub stores only the blocks that have changed over time on
the server so local storage never increases in size. Also, we do not have to
cache the entire file to make the regeneration length small, since pointers can
jump directly to the data. This is also an improvement over GitLFS which stores
an entirely new snapshot of the file on every change.

### Terminology

This section is still in progress. Expect terminology to change during
development.

- Mainline - The production history of the project. Made up of a series of
  "commits" that represent good versions of the project.
- Workspace - A workspace for developers to make changes in. Developers will make
  "changes" in their workspace and merge into the "mainline" when approved/ready.
  "Changes" will be tracked while in the workspace, but will be squashed into a
  single "commit" when merged into the mainline. Eventually, changes will be
  able to be synced live between local developer machine and their workspace.
- Change - A modification of a workspace while developers are working on their
  project, made by doing a `jam push`.
- Commit - A modification of the production version of the project, made by
  merging in a "workspace".
- Merge - Occurs when a workspace is squashed and committed to the "mainline".

### Limitations

The goal is to be able to handle over 100M files and over 1TB-sized files in a
single repository. We're not there yet in the current implementation (~1M files
with 16GB-sized files) but should be there in the next couple months.

### Implementation

JamHub is being written from scratch in [Golang](https://go.dev/) and uses
[mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) to store projects and
change information. [gRPC](https://grpc.io/) and
[Protocol buffers](https://developers.google.com/protocol-buffers) are used for
service definitions and data serialization.

## Current Status

JamHub is currently in development. You can currently push, pull, workon 
workspaces, and merge workspaces. Over the next few months we'll be adding features
to make this usable for regular development. The following features are planned:

- Multi-person project collaboration
- File locking
- File ownership and permissions
- Live file and change syncing
- Open API access
- NFS protocol implementation

If there are any additional features you would like to see, please make a
discussion or email me at [zach@jamhub.dev](mailto:zach@jamhub.dev).

## Developing

### Setup

Note that this is for setting up development or compiling JamHub yourself. If
you want binaries and installation instructions go to
[jamhub.dev/login](https://jamhub.dev). Documentation for self-hosting is
going to be pretty sparse right now, but reach out on
[Discord](https://discord.gg/6bK3GPKhpa) if you need any help. The general steps
are below and you should be able to solve most issues by resolving any
errors/dependencies that occur.

1. Install Go, Protoc, Make
2. Setup env with Auth0 variables
3. Run desired `make` target

### Architecture

As a general overview there are three services that currently compose JamHub --
web, server, client.

1. Web - runs the REST API and website
2. Server - runs the backend server for storing and retrieving changes
3. Client - client-side CLI tool to connect to the server

The client and web services connect through gRPC to the backend server and we
interact online through the web REST API. More documentation will be added in
the future to detail how changes are stored but changes and project files are
currently stored in the `jb/` directory where the server is started.

## Contributing

Although I welcome contributions, I ask that you DM me first before making
changes so you don't waste time unless it's a small bug. JamHub is early in
development so things could change drastically.

## Contact

Email Zach Geier at [zach@jamhub.dev](mailto:zach@jamhub.dev) or join the
[Discord](https://discord.gg/6bK3GPKhpa)
