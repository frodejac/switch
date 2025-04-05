rm -rf data
go build -o switchd .
./switchd --node-id=node1 --http-addr=0.0.0.0:9990 --raft-addr=0.0.0.0:9991 --raft-advertise-addr=127.0.0.1:9991 --raft-dir=data/node1/raft --feature-flags-dir=data/node1/feature_flags --membership-dir=data/node1/membership --bootstrap --pre-warm
