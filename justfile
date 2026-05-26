ch1: (echo "1" "100" "5")
ch2: (generate "1" "100" "5")
ch3a: (broadcast "--node-count 1 --rate 20 --time-limit 10")
ch3b: (broadcast "--node-count 5 --rate 20 --time-limit 10")
ch3c: (broadcast "--node-count 5 --rate 20 --time-limit 10 --nemesis partition")
ch3d: (broadcast "--node-count 25 --rate 100 --time-limit 20 --latency 100")

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
