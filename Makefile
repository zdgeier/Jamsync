# Local Dev ===========================
client:
	JAM_ENV=local go run cmd/jamcli/main.go

web:
	cd cmd/jamhubweb/; JAM_ENV=local go run main.go

server:
	JAM_ENV=local go run cmd/jamhubgrpc/main.go

# Build ================================

clean:
	rm -rf jamhub-build && rm -rf .jam && rm -rf jamhub-build.zip && rm -rf jamhubdata/

zipself:
	git archive --format=zip --output jamhub-source.zip HEAD && mkdir -p ./jamhub-build/ && mv jamhub-source.zip ./jamhub-build/

protos:
	mkdir -p gen/go && protoc --proto_path=proto --go_out=gen/pb --go_opt=paths=source_relative --go-grpc_out=gen/pb --go-grpc_opt=paths=source_relative proto/*.proto

buildeditor:
	cd cmd/jamhubweb/editor && ./node_modules/.bin/rollup -c rollup.config.mjs && mv *.bundle.js ../public/

movewebassets:
	cp -R cmd/jamhubweb/public jamhub-build/; cp -R cmd/jamhubweb/template jamhub-build/;

buildui:
	./scripts/buildui.sh

# Needed to be done locally since Mac requires signing binaries. Make sure you have signing env variables setup to do this.
buildclients:
	./scripts/buildclients.sh

# Run on server since ARM has some weirdness with cgo
buildservers:
	go build -ldflags "-X main.built=`date -u +%Y-%m-%d+%H:%M:%S` -X main.version=v0.0.1" -o jamhubgrpc cmd/jamhubgrpc/main.go && go build -ldflags "-X main.built=`date -u +%Y-%m-%d+%H:%M:%S` -X main.version=v0.0.1"  -o jamhubweb cmd/jamhubweb/main.go

zipbuild:
	zip -r jamhub-build.zip jamhub-build/

# Make sure to setup hosts file to resolve ssh.prod.jamhub.dev to proper backend server.
uploadbuild:
	scp -i ~/jamsync-prod-us-west-1.pem ./jamhub-build.zip ec2-user@ssh.prod.jamsync.dev:~/jamhub-build.zip

# Needed since make doesn't build same target twice and I didn't bother to find a better way
cleanbuild:
	rm -rf jamhub-build && rm -rf jamhub-build.zip

# Deploy ===============================

# Make sure to setup hosts file to resolve ssh.prod.jamhub.dev to proper backend server.
deploy:
	./scripts/deploy.sh

build: clean zipself protos buildeditor movewebassets buildclients buildui zipbuild uploadbuild deploy cleanbuild installclientremote

# Misc ================================

install:
	go mod tidy && cd cmd/web/editor/ && npm install

installclient:
	go build -ldflags "-X main.built=`date -u  +%Y-%m-%d+%H:%M:%S` -X main.version=v0.0.1" -o jam cmd/jamcli/main.go && mv jam ~/bin/jam

installclientremote:
	rm -rf jam_darwin_arm64.zip && wget https://jamhub.dev/public/jam_darwin_arm64.zip && unzip jam_darwin_arm64.zip && mv jam ~/bin/jam && rm -rf jam_darwin_arm64.zip

grpcui:
	grpcui -insecure 0.0.0.0:14357

ssh:
	ssh -i ~/jamsync-prod-us-west-1.pem ec2-user@ssh.prod.jamsync.dev
