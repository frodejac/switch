rm -rf data
go build -o switch .
./switch --node-id=node1 --http-addr=0.0.0.0:9990 --raft-addr=0.0.0.0:9991 --raft-advertise-addr=127.0.0.1:9991 --raft-dir=data/node1 --bootstrap --pre-warm
