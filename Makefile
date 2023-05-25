# Local Dev ===========================
client:
	JAM_ENV=local go run cmd/client/main.go

web:
	cd cmd/web/; JAM_ENV=local go run main.go

server:
	JAM_ENV=local go run cmd/server/main.go

# Build ================================

clean:
	rm -rf jamsync-build && rm -rf .jamsync && rm -rf jamsync-build.zip && rm -rf internal/server/client/jb && rm -rf jb/

zipself:
	git archive --format=zip --output jamsync-source.zip HEAD && mkdir -p ./jamsync-build/ && mv jamsync-source.zip ./jamsync-build/

protos:
	mkdir -p gen/go && protoc --proto_path=proto --go_out=gen/pb --go_opt=paths=source_relative --go-grpc_out=gen/pb --go-grpc_opt=paths=source_relative proto/*.proto

buildeditor:
	cd cmd/web/editor && ./node_modules/.bin/rollup -c rollup.config.mjs && mv *.bundle.js ../public/

movewebassets:
	cp -R cmd/web/public jamsync-build/; cp -R cmd/web/template jamsync-build/;

# Needed to be done locally since Mac requires signing binaries. Make sure you have signing env variables setup to do this.
buildclients:
	./scripts/buildclients.sh

# Run on server since ARM has some weirdness with cgo
buildservers:
	go build -ldflags "-X main.built=`date -u +%Y-%m-%d+%H:%M:%S` -X main.version=v0.0.4" -o jamserver cmd/server/main.go && go build -ldflags "-X main.built=`date -u +%Y-%m-%d+%H:%M:%S` -X main.version=v0.0.4"  -o jamweb cmd/web/main.go

zipbuild:
	zip -r jamsync-build.zip jamsync-build/

# Make sure to setup hosts file to resolve ssh.prod.jamsync.dev to proper backend server.
uploadbuild:
	scp -i ~/jamsync-prod-us-west-1.pem ./jamsync-build.zip ec2-user@ssh.prod.jamsync.dev:~/jamsync-build.zip

# Needed since make doesn't build same target twice and I didn't bother to find a better way
cleanbuild:
	rm -rf jamsync-build && rm -rf jamsync-build.zip

# Deploy ===============================

# Make sure to setup hosts file to resolve ssh.prod.jamsync.dev to proper backend server.
deploy:
	./scripts/deploy.sh

build: clean zipself protos buildeditor movewebassets buildclients zipbuild uploadbuild deploy cleanbuild installclientremote

# Misc ================================

install:
	go mod tidy && cd cmd/web/editor/ && npm install

installclient:
	go build -ldflags "-X main.built=`date -u  +%Y-%m-%d+%H:%M:%S` -X main.version=v0.0.4" -o jam cmd/client/main.go && mv jam ~/bin/jam

installclientremote:
	rm -rf jam_darwin_arm64.zip && wget https://jamsync.dev/public/jam_darwin_arm64.zip && unzip jam_darwin_arm64.zip && mv jam ~/bin/jam && rm -rf jam_darwin_arm64.zip

grpcui:
	grpcui -insecure 0.0.0.0:14357

ssh:
	ssh -i ~/jamsync-prod-us-west-1.pem ec2-user@ssh.prod.jamsync.dev
