ch1: (echo "1" "100" "5")
ch2: (generate "1" "100" "5")
ch3a: (broadcast "--node-count 1 --rate 20 --time-limit 10")
ch3b: (broadcast "--node-count 5 --rate 20 --time-limit 10")
ch3c: (broadcast "--node-count 5 --rate 20 --time-limit 10 --nemesis partition")
ch3d: (broadcast "--node-count 25 --rate 100 --time-limit 20 --latency 100")
ch3e: ch3d
ch4: (g-counter "--node-count 3 --rate 100 --time-limit 20 --nemesis partition")
ch4easy: (g-counter "--node-count 1 --rate 10 --time-limit 3")
ch5a: (kafka "--node-count 1 --concurrency 2n --time-limit 20 --rate 1000")
ch5b: (kafka "--node-count 2 --concurrency 2n --time-limit 20 --rate 1000")
ch5c: ch5b
ch6a: (txn-rw-register "--node-count 1 --time-limit 20 --rate 1000 --concurrency 2n --consistency-models read-uncommitted --availability total")
ch6beasy: (txn-rw-register "--node-count 2 --concurrency 2n --time-limit 20 --rate 1000 --consistency-models read-uncommitted")
ch6b: (txn-rw-register "--node-count 2 --concurrency 2n --time-limit 20 --rate 1000 --consistency-models read-uncommitted --availability total --nemesis partition")
ch6c: (txn-rw-register "--node-count 2 --concurrency 2n --time-limit 20 --rate 1000 --consistency-models read-committed --availability total --nemesis partition")

build-echo:
    cd maelstrom-echo && go build -o bin/maelstrom-echo

echo node-count="1" rate="100" time-limit="5": build-echo
    ./maelstrom/maelstrom test -w echo --bin ./maelstrom-echo/bin/maelstrom-echo --node-count {{node-count}} --time-limit {{time-limit}} --rate {{rate}}

build-generate:
    cd maelstrom-generate && go build -o bin/maelstrom-generate
    
generate node-count="1" rate="100" time-limit="5": build-generate
    ./maelstrom/maelstrom test -w unique-ids --bin ./maelstrom-generate/bin/maelstrom-generate --node-count {{node-count}} --time-limit {{time-limit}} --rate {{rate}}

build-broadcast:
    cd maelstrom-broadcast && go build -o bin/maelstrom-broadcast

broadcast args: build-broadcast
    ./maelstrom/maelstrom test -w broadcast --bin ./maelstrom-broadcast/bin/maelstrom-broadcast {{args}}

serve:
    ./maelstrom/maelstrom serve

latest-results:
    cat ./store/latest/results.edn

build-g-counter:
    cd maelstrom-g-counter && go build -o bin/maelstrom-g-counter

g-counter args: build-g-counter
    ./maelstrom/maelstrom test -w g-counter --bin ./maelstrom-g-counter/bin/maelstrom-g-counter {{args}}

build-kafka:
    cd maelstrom-kafka && go build -o bin/maelstrom-kafka

kafka args: build-kafka
    ./maelstrom/maelstrom test -w kafka --bin ./maelstrom-kafka/bin/maelstrom-kafka {{args}}

build-txn-rw-register:
    cd maelstrom-txn-rw-register && go build -o bin/maelstrom-txn-rw-register

txn-rw-register args: build-txn-rw-register
    ./maelstrom/maelstrom test -w txn-rw-register --bin ./maelstrom-txn-rw-register/bin/maelstrom-txn-rw-register {{args}}
