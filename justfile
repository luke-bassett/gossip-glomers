ch1: (echo "1" "100" "5")
ch2: (generate "1" "100" "5")
ch3a: (broadcast "1" "100" "5")
ch3b: (broadcast "5" "100" "5")

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

broadcast node-count="5" rate="10" time-limit="20": build-broadcast
    ./maelstrom/maelstrom test -w broadcast --bin ./maelstrom-broadcast/bin/maelstrom-broadcast --node-count {{node-count}} --time-limit {{time-limit}} --rate {{rate}} 

serve:
    ./maelstrom/maelstrom serve
